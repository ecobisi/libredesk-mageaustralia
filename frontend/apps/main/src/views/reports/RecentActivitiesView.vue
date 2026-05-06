<template>
  <div class="overflow-y-auto">
    <div
      class="p-6 w-[calc(100%-3rem)]"
      :class="{ 'opacity-50 transition-opacity duration-300': isLoading && !activities.length }"
    >
      <div class="space-y-4">
        <!-- Activity list -->
        <div v-if="activities.length" class="space-y-0">
          <div
            v-for="activity in activities"
            :key="activity.id"
            class="flex items-start gap-3 py-3 border-b last:border-b-0"
          >
            <Avatar class="h-8 w-8 shrink-0">
              <AvatarImage :src="activity.actor_avatar_url || ''" />
              <AvatarFallback class="text-xs font-medium">
                {{ initials(activity.actor_first_name, activity.actor_last_name) }}
              </AvatarFallback>
            </Avatar>

            <div class="min-w-0 flex-1">
              <p class="text-sm">
                <span v-if="activity.type === 'activity'" v-html="activity.content" />
                <span v-else>
                  <span class="font-medium">
                    {{ activity.actor_first_name }} {{ activity.actor_last_name }}
                  </span>
                  {{ t('report.recentActivities.sentResponse') }}
                </span>
                <router-link
                  :to="conversationLink(activity)"
                  class="text-primary hover:underline ml-1"
                >
                  #{{ activity.reference_number }}
                </router-link>
                <span v-if="activity.subject" class="text-muted-foreground"> — {{ activity.subject }}</span>
              </p>
              <p
                class="text-xs text-muted-foreground mt-0.5"
                :title="formatFullTimestamp(new Date(activity.created_at))"
              >
                {{ getRelativeTime(new Date(activity.created_at)) }}
              </p>
            </div>
          </div>
        </div>

        <!-- Empty state -->
        <div
          v-if="!isLoading && !activities.length"
          class="text-center text-muted-foreground py-12"
        >
          {{ t('report.recentActivities.empty') }}
        </div>

        <!-- Loading -->
        <div v-if="isLoading" class="flex justify-center py-8">
          <Spinner />
        </div>

        <!-- Load more -->
        <div v-if="hasMore && !isLoading" class="flex justify-center pt-2 pb-4">
          <Button variant="outline" size="sm" @click="loadMore">
            {{ t('globals.terms.loadMore') }}
          </Button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { Avatar, AvatarFallback, AvatarImage } from '@shared-ui/components/ui/avatar'
import { Button } from '@shared-ui/components/ui/button'
import { Spinner } from '@shared-ui/components/ui/spinner'
import { getRelativeTime, formatFullTimestamp } from '@shared-ui/utils/datetime'
import { handleHTTPError } from '@shared-ui/utils/http.js'
import { useEmitter } from '@/composables/useEmitter'
import { EMITTER_EVENTS } from '@/constants/emitterEvents'
import api from '@/api'

const { t } = useI18n()
const emitter = useEmitter()

const activities = ref([])
const isLoading = ref(false)
const page = ref(1)
const pageSize = 20
const totalItems = ref(0)

const hasMore = computed(() => activities.value.length < totalItems.value)

const showToast = (description, variant) =>
  emitter.emit(EMITTER_EVENTS.SHOW_TOAST, variant ? { variant, description } : { description })

const initials = (first, last) =>
  `${first?.[0] || ''}${last?.[0] || ''}`.toUpperCase() || '?'

const conversationLink = (activity) =>
  `/inboxes/all/conversation/${activity.conversation_uuid}`

const fetchActivities = async (append = false) => {
  isLoading.value = true
  try {
    const res = await api.getRecentActivities({ page: page.value, page_size: pageSize })
    const data = res.data?.data
    if (data) {
      const results = data.results || []
      activities.value = append ? [...activities.value, ...results] : results
      totalItems.value = data.total || 0
    }
  } catch (err) {
    showToast(handleHTTPError(err).message, 'destructive')
  } finally {
    isLoading.value = false
  }
}

const loadMore = () => {
  page.value++
  fetchActivities(true)
}

onMounted(() => fetchActivities())
</script>
