<template>
  <div class="flex flex-col h-screen">
    <SearchHeader v-model="searchQuery" @search="handleSearch" />
    <div class="flex-1 overflow-y-auto">
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
            <Button variant="outline" @click="loadMore" :disabled="loading">
              <Spinner v-if="loading" class="mr-2 h-4 w-4" />
              Load more ({{ results.length }} of {{ total }})
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
              $t('globals.messages.search', {
                name: $t('globals.terms.conversation', 2).toLowerCase()
              })
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
import { handleHTTPError } from '@/utils/http'
import { Button } from '@/components/ui/button'
import SearchHeader from '@/features/search/SearchHeader.vue'
import SearchResults from '@/features/search/SearchResults.vue'
import Spinner from '@/components/ui/spinner/Spinner.vue'
import api from '@/api'

const MIN_SEARCH_LENGTH = 3
const DEBOUNCE_DELAY = 300
const PAGE_SIZE = 30

const searchQuery = ref(sessionStorage.getItem('searchQuery') || '')
const results = ref(JSON.parse(sessionStorage.getItem('searchResults') || '[]'))
const total = ref(parseInt(sessionStorage.getItem('searchTotal') || '0'))
const page = ref(1)
const loading = ref(false)
const error = ref(null)
const searchPerformed = ref(searchQuery.value.length >= MIN_SEARCH_LENGTH && results.value.length > 0)
let debounceTimer = null

// When navigating to search fresh (sessionStorage cleared by sidebar/shortcut),
// reset local state if it's stale

import { useRoute } from 'vue-router'
const _route = useRoute()
watch(() => _route.fullPath, () => {
  if (_route.name === 'search' && !sessionStorage.getItem('searchQuery')) {
    searchQuery.value = ''
    results.value = []
    total.value = 0
    searchPerformed.value = false
    page.value = 1
  }
})

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
    const data = resp.data.data
    if (pageNum === 1) {
      results.value = data.results || []
    } else {
      results.value = [...results.value, ...(data.results || [])]
    }
    total.value = data.total || 0
    page.value = pageNum
    // Clear old results before storing new ones to avoid exceeding sessionStorage quota.
    sessionStorage.removeItem('searchResults')
    sessionStorage.setItem('searchQuery', searchQuery.value)
    sessionStorage.setItem('searchTotal', String(total.value))
    try {
      sessionStorage.setItem('searchResults', JSON.stringify(results.value))
    } catch {
      // Results too large for sessionStorage — skip caching.
    }
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

onBeforeUnmount(() => {
  clearTimeout(debounceTimer)
})
</script>
