<template>
  <div class="flex flex-col h-screen">
    <SearchHeader ref="searchHeader" v-model="searchQuery" @search="handleSearch" />
    <div class="flex-1 overflow-y-auto">
      <!-- Spinner only on initial fetch — subsequent paginated loads render
           the existing list and an inline spinner on the load-more button. -->
      <div v-if="loading && page === 1" class="flex justify-center items-center h-64">
        <Spinner />
      </div>
      <div v-else-if="error" class="mt-8 text-center space-y-4">
        <p class="text-lg text-destructive">{{ error }}</p>
        <Button @click="handleSearch"> {{ $t('globals.terms.tryAgain') }} </Button>
      </div>

      <div v-else>
        <p
          v-if="searchPerformed && results.length === 0"
          class="mt-8 text-center text-muted-foreground"
        >
          {{
            $t('search.noResultsForQuery', {
              query: searchQuery
            })
          }}
        </p>
        <template v-else-if="searchPerformed">
          <SearchResults :results="results" :total="total" class="h-full" />
          <div v-if="hasMore" class="flex justify-center py-6">
            <Button variant="outline" :disabled="loading" @click="loadMore">
              <Spinner v-if="loading" class="mr-2 h-4 w-4" />
              {{ $t('search.loadMore', { shown: results.length, total }) }}
            </Button>
          </div>
        </template>

        <p
          v-else-if="searchQuery.length > 0 && searchQuery.length < MIN_SEARCH_LENGTH"
          class="mt-8 text-center text-muted-foreground"
        >
          {{
            $t('search.minQueryLength', {
              length: MIN_SEARCH_LENGTH
            })
          }}
        </p>
        <div v-else class="mt-16 text-center">
          <h2 class="text-2xl font-semibold text-primary mb-4">
            {{
              $t('conversation.search')
            }}
          </h2>
          <p class="text-lg text-muted-foreground">
            {{ $t('search.searchBy') }}
          </p>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onBeforeUnmount } from 'vue'
import { useRoute } from 'vue-router'
import { handleHTTPError } from '@shared-ui/utils/http.js'
import { Button } from '@shared-ui/components/ui/button'
import SearchHeader from '@/features/search/SearchHeader.vue'
import SearchResults from '@/features/search/SearchResults.vue'
import Spinner from '@shared-ui/components/ui/spinner/Spinner.vue'
import api from '../../api'

const MIN_SEARCH_LENGTH = 3
const DEBOUNCE_DELAY = 300
const PAGE_SIZE = 30

// FS8: Persist the last search across navigation so clicking into a result
// and pressing Back lands you in your previous result list. sessionStorage
// scope is fine — we only want this within the current tab.
//
// Defensive parse: an older fork format stored results as an object
// (`{messages, tickets}`); coerce anything that isn't an array back to []
// so the iterator and load-more logic don't silently break for users
// upgrading mid-tab.
const restoreResults = () => {
  try {
    const parsed = JSON.parse(sessionStorage.getItem('searchResults') || '[]')
    return Array.isArray(parsed) ? parsed : []
  } catch (e) {
    return []
  }
}
const searchQuery = ref(sessionStorage.getItem('searchQuery') || '')
const results = ref(restoreResults())
const total = ref(parseInt(sessionStorage.getItem('searchTotal') || '0', 10))
const page = ref(1)
const loading = ref(false)
const error = ref(null)
const searchPerformed = ref(
  searchQuery.value.length >= MIN_SEARCH_LENGTH && results.value.length > 0
)
let debounceTimer = null

const hasMore = computed(() => results.value.length < total.value)

const fetchResults = async (pageNum) => {
  loading.value = true
  error.value = null

  try {
    const resp = await api.searchUnified({
      query: searchQuery.value,
      page: pageNum,
      page_size: PAGE_SIZE
    })
    const data = resp.data.data || {}
    if (pageNum === 1) {
      results.value = data.results || []
    } else {
      results.value = [...results.value, ...(data.results || [])]
    }
    total.value = data.total || 0
    page.value = pageNum
    sessionStorage.setItem('searchQuery', searchQuery.value)
    sessionStorage.setItem('searchResults', JSON.stringify(results.value))
    sessionStorage.setItem('searchTotal', String(total.value))
  } catch (err) {
    error.value = handleHTTPError(err).message
  } finally {
    loading.value = false
  }
}

const handleSearch = async () => {
  if (searchQuery.value.length < MIN_SEARCH_LENGTH) {
    results.value = []
    total.value = 0
    searchPerformed.value = false
    return
  }
  searchPerformed.value = true
  page.value = 1
  await fetchResults(1)
}

const loadMore = () => {
  page.value = page.value + 1
  fetchResults(page.value)
}

const debouncedSearch = () => {
  clearTimeout(debounceTimer)
  debounceTimer = setTimeout(handleSearch, DEBOUNCE_DELAY)
}

watch(searchQuery, (newValue) => {
  if (newValue.length >= MIN_SEARCH_LENGTH) {
    debouncedSearch()
  } else {
    clearTimeout(debounceTimer)
    results.value = []
    total.value = 0
    searchPerformed.value = false
  }
})

// FS14: When the sidebar's search icon is clicked while we're already on the
// search route, it clears sessionStorage and pushes a timestamped query.
// Watch the route so the in-memory state (which was hydrated from
// sessionStorage at mount) gets cleared too, otherwise the user sees their
// previous results until they start typing. Also refocus the input, since
// the component isn't remounted on a same-route nav so SearchHeader's
// onMounted focus doesn't fire.
const route = useRoute()
const searchHeader = ref(null)
watch(
  () => route.fullPath,
  () => {
    if (route.name === 'search' && !sessionStorage.getItem('searchQuery')) {
      clearTimeout(debounceTimer)
      searchQuery.value = ''
      results.value = []
      total.value = 0
      page.value = 1
      searchPerformed.value = false
      error.value = null
      searchHeader.value?.focus?.()
    }
  }
)

onBeforeUnmount(() => {
  clearTimeout(debounceTimer)
})
</script>
