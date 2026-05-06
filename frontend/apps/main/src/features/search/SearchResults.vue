<template>
  <div class="max-w-5xl mx-auto p-6 min-h-screen">
    <p class="text-sm text-muted-foreground mb-4">
      {{ $t('search.results', { total }) }}
    </p>
    <div class="bg-background rounded border overflow-hidden">
      <div class="divide-y divide-border">
        <div
          v-for="item in results"
          :key="item.uuid"
          class="p-6 hover:bg-accent/50 transition duration-200 ease-in-out group"
        >
          <router-link
            :to="{
              name: 'inbox-conversation',
              params: { uuid: item.uuid, type: 'assigned' }
            }"
            class="block"
          >
            <div class="flex justify-between items-start">
              <div class="flex-grow min-w-0">
                <!-- Reference number + status -->
                <div
                  class="text-sm font-semibold mb-1 text-muted-foreground group-hover:text-primary transition duration-200 flex items-center gap-2"
                >
                  <span>#{{ item.reference_number }}</span>
                  <Badge v-if="item.status" variant="outline" class="text-xs font-medium">
                    {{ item.status }}
                  </Badge>
                </div>
                <!-- Subject -->
                <div
                  class="text-foreground font-medium mb-2 text-lg group-hover:text-primary transition duration-200"
                >
                  {{ item.subject || $t('conversation.list.noSubject') }}
                </div>
                <!-- Snippet (truncated message body) -->
                <div
                  v-if="item.snippet"
                  class="text-sm text-muted-foreground mb-2 line-clamp-2"
                >
                  {{ truncateText(item.snippet, 200) }}
                </div>
                <!-- Timestamp -->
                <div class="text-sm text-muted-foreground flex items-center">
                  <ClockIcon class="h-4 w-4 mr-1" />
                  {{ formatDate(item.created_at) }}
                </div>
              </div>
              <div
                class="bg-secondary rounded-full p-2 group-hover:bg-primary transition duration-200 shrink-0 ml-3"
              >
                <ChevronRightIcon
                  class="h-5 w-5 text-secondary-foreground group-hover:text-primary-foreground"
                  aria-hidden="true"
                />
              </div>
            </div>
          </router-link>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ChevronRightIcon, ClockIcon } from 'lucide-vue-next'
import { format, parseISO } from 'date-fns'
import { Badge } from '@shared-ui/components/ui/badge'

defineProps({
  results: {
    type: Array,
    required: true
  },
  total: {
    type: Number,
    default: 0
  }
})

const formatDate = (dateString) => {
  const date = parseISO(dateString)
  return format(date, 'MMM d, yyyy HH:mm')
}

const truncateText = (text, length) => {
  if (!text) return ''
  if (text.length <= length) return text
  return text.slice(0, length) + '...'
}
</script>
