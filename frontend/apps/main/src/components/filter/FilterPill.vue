<script setup>
// One filter pill — a button that summarises the current filter state and
// opens a popover with the appropriate editor (FilterMultiSelect or
// FilterDatePicker).
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Popover, PopoverTrigger, PopoverContent } from '@shared-ui/components/ui/popover'
import { X } from 'lucide-vue-next'
import FilterMultiSelect from './FilterMultiSelect.vue'
import FilterDatePicker from './FilterDatePicker.vue'

const props = defineProps({
  field: {
    type: Object,
    required: true
  },
  modelValue: {
    type: Object,
    required: true
  },
  isDateField: {
    type: Boolean,
    default: false
  },
  autoOpen: {
    type: Boolean,
    default: false
  }
})

const emit = defineEmits(['update:modelValue', 'remove'])
const { t } = useI18n()

const popoverOpen = ref(props.autoOpen)

const presetLabels = computed(() => ({
  today: t('filter.preset.today'),
  yesterday: t('filter.preset.yesterday'),
  last_7_days: t('filter.preset.last7Days'),
  last_30_days: t('filter.preset.last30Days'),
  this_month: t('filter.preset.thisMonth')
}))

const summaryText = computed(() => {
  if (!props.modelValue) return props.field.label
  const label = props.field.label

  if (props.isDateField) {
    if (props.modelValue.operator === 'relative_date') {
      return `${label}: ${presetLabels.value[props.modelValue.value] || props.modelValue.value}`
    }
    if (props.modelValue.operator === 'between') {
      const parts = (props.modelValue.value || '').split(',')
      return `${label}: ${parts[0]} - ${parts[1]}`
    }
    return label
  }

  try {
    const values = JSON.parse(props.modelValue.value)
    if (!values || values.length === 0) return label
    const options = props.field.options || []
    const names = values.map((v) => {
      const opt = options.find((o) => String(o.value) === String(v))
      return opt ? opt.label : v
    })
    const op = props.modelValue.operator
    const prefix = (op === 'not_in' || op === 'not contains' || op === 'not_contains')
      ? `${t('filter.notPrefix')} `
      : ''
    if (names.length <= 2) return `${label}: ${prefix}${names.join(', ')}`
    return `${label}: ${prefix}${names[0]}, +${names.length - 1}`
  } catch {
    return label
  }
})

function handleUpdate (filter) {
  emit('update:modelValue', filter)
}

function handleRemove () {
  popoverOpen.value = false
  emit('remove')
}

function handleRemoveClick (e) {
  e.stopPropagation()
  e.preventDefault()
  emit('remove')
}

// WebSocket re-renders steal focus from the popover content, which Radix
// reads as "user clicked outside" and closes the popover. Suppress the
// focus-outside dismissal but keep pointer-down-outside so explicit clicks
// elsewhere still close the popover normally.
function preventFocusClose (event) {
  event.preventDefault()
}
</script>

<template>
  <Popover v-model:open="popoverOpen">
    <PopoverTrigger as-child>
      <button
        class="inline-flex items-center gap-1 h-7 pl-2.5 pr-1 rounded-full border bg-background text-sm hover:bg-accent transition-colors max-w-xs"
      >
        <span class="truncate">{{ summaryText }}</span>
        <span
          class="inline-flex items-center justify-center h-4 w-4 rounded-full hover:bg-muted-foreground/20"
          @click="handleRemoveClick"
        >
          <X class="h-3 w-3" />
        </span>
      </button>
    </PopoverTrigger>
    <PopoverContent
      align="start"
      :side-offset="4"
      class="p-2 w-auto"
      @focus-outside="preventFocusClose"
    >
      <FilterDatePicker
        v-if="isDateField"
        :field="field"
        :model-value="modelValue"
        @update:model-value="handleUpdate"
        @remove="handleRemove"
      />
      <FilterMultiSelect
        v-else
        :field="field"
        :model-value="modelValue"
        @update:model-value="handleUpdate"
        @remove="handleRemove"
      />
    </PopoverContent>
  </Popover>
</template>
