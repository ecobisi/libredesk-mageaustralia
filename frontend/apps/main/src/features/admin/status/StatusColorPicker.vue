<template>
  <Popover>
    <PopoverTrigger asChild>
      <button
        type="button"
        class="text-[10px] font-medium px-2 py-0.5 rounded-full whitespace-nowrap cursor-pointer hover:opacity-80 transition-opacity inline-flex items-center gap-1"
        :style="statusColorStyle(currentColor)"
        :title="$t('globals.messages.edit')"
      >
        <span>{{ getStatusColor(currentColor).label }}</span>
        <ChevronDown class="w-2.5 h-2.5 opacity-60" />
      </button>
    </PopoverTrigger>
    <PopoverContent class="w-[180px] p-1" align="start">
      <button
        v-for="opt in STATUS_COLOR_OPTIONS"
        :key="opt.value"
        type="button"
        class="flex items-center gap-2 w-full px-2 py-1.5 text-xs rounded hover:bg-muted cursor-pointer"
        :class="{ 'bg-muted': currentColor === opt.value }"
        @click="select(opt.value)"
      >
        <span
          class="w-4 h-4 rounded border border-black/10"
          :style="{ backgroundColor: opt.bg }"
        ></span>
        <span :style="{ color: opt.text, fontWeight: currentColor === opt.value ? 600 : 400 }">
          {{ opt.label }}
        </span>
      </button>
    </PopoverContent>
  </Popover>
</template>

<script setup>
// Inline colour picker for the admin status list. Calls the color-only
// endpoint (PUT /statuses/{id}/color) so it works on default statuses
// (Open / Snoozed / Resolved / Closed) which the regular update path
// blocks via the cannotUpdateDefault guard.
import { ref, watch } from 'vue'
import { ChevronDown } from 'lucide-vue-next'
import {
  Popover,
  PopoverContent,
  PopoverTrigger
} from '@shared-ui/components/ui/popover'
import { STATUS_COLOR_OPTIONS, statusColorStyle, getStatusColor } from '@/constants/statusColors'
import { useEmitter } from '@/composables/useEmitter.js'
import { EMITTER_EVENTS } from '@/constants/emitterEvents.js'
import { handleHTTPError } from '@shared-ui/utils/http.js'
import api from '@/api'

const props = defineProps({
  statusId: { type: [Number, String], required: true },
  color: { type: String, default: 'gray' }
})

const emit = useEmitter()
const currentColor = ref(props.color || 'gray')

watch(() => props.color, (next) => {
  currentColor.value = next || 'gray'
})

const select = async (value) => {
  if (value === currentColor.value) return
  const previous = currentColor.value
  currentColor.value = value // optimistic
  try {
    await api.updateStatusColor(props.statusId, value)
    emit.emit(EMITTER_EVENTS.REFRESH_LIST, { model: 'status' })
  } catch (error) {
    currentColor.value = previous
    emit.emit(EMITTER_EVENTS.SHOW_TOAST, {
      variant: 'destructive',
      description: handleHTTPError(error).message
    })
  }
}
</script>
