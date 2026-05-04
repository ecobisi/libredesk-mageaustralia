<template>
  <div>
    <div v-if="conversations.length" class="text-xs text-muted-foreground mb-3">
      {{ conversations.length }} {{ conversations.length === 1 ? 'conversation' : 'conversations' }}
    </div>
    <div v-if="loading" class="text-center text-sm text-muted-foreground py-4">
      <Spinner />
    </div>
    <div v-else-if="conversations.length === 0" class="text-center text-sm text-muted-foreground py-4">
      {{ $t('conversation.sidebar.noPreviousConvo') }}
    </div>
    <div v-else class="space-y-1">
      <router-link
        v-for="conversation in conversations"
        :key="conversation.uuid"
        :to="{
          name: 'inbox-conversation',
          params: {
            uuid: conversation.uuid,
            type: 'assigned'
          }
        }"
        class="block p-2 rounded hover:bg-muted border"
      >
        <div class="flex flex-wrap items-start justify-between gap-1">
          <div class="flex flex-col flex-1 min-w-[120px]">
            <Tooltip>
              <TooltipTrigger asChild>
                <span class="font-medium text-sm truncate max-w-[400px]">
                  {{ conversation.subject || '(no subject)' }}
                </span>
              </TooltipTrigger>
              <TooltipContent>
                {{ conversation.subject || '(no subject)' }}
              </TooltipContent>
            </Tooltip>
            <span class="text-xs text-muted-foreground truncate max-w-[500px]">
              {{ conversation.last_message }}
            </span>
          </div>
          <Tooltip>
            <TooltipTrigger asChild>
              <div class="flex gap-1 items-center text-xs text-muted-foreground flex-shrink-0">
                <span v-if="conversation.created_at">
                  {{ getRelativeTime(new Date(conversation.created_at)) }}
                </span>
                <span v-if="conversation.last_message_at">•</span>
                <span v-if="conversation.last_message_at">
                  {{ getRelativeTime(new Date(conversation.last_message_at)) }}
                </span>
              </div>
            </TooltipTrigger>
            <TooltipContent>
              <div class="space-y-1 text-xs">
                <p>
                  {{ $t('globals.terms.createdAt') }}:
                  {{ formatFullTimestamp(new Date(conversation.created_at)) }}
                </p>
                <p v-if="conversation.last_message_at">
                  {{ $t('globals.terms.lastMessageAt') }}:
                  {{ formatFullTimestamp(new Date(conversation.last_message_at)) }}
                </p>
              </div>
            </TooltipContent>
          </Tooltip>
        </div>
      </router-link>
    </div>
  </div>
</template>

<script setup>
import { ref, watch } from 'vue'
import api from '@/api'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { Spinner } from '@/components/ui/spinner'
import { formatFullTimestamp, getRelativeTime } from '@/utils/datetime'

const props = defineProps({
  contactId: { type: [Number, String], required: true }
})

const conversations = ref([])
const loading = ref(false)

const load = async (id) => {
  if (!id) return
  loading.value = true
  try {
    const resp = await api.getContactConversations(id)
    conversations.value = resp.data?.data || []
  } catch (e) {
    conversations.value = []
  } finally {
    loading.value = false
  }
}

watch(() => props.contactId, (id) => load(id), { immediate: true })
</script>
