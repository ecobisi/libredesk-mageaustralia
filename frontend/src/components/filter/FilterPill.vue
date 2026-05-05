<script setup>
import { ref, computed } from 'vue'
import { Popover, PopoverTrigger, PopoverContent } from '@/components/ui/popover'
import { X } from 'lucide-vue-next'
import FilterMultiSelect from './FilterMultiSelect.vue'
import FilterDatePicker from './FilterDatePicker.vue'
import FilterTextInput from './FilterTextInput.vue'

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
  isTextField: {
    type: Boolean,
    default: false
  },
  autoOpen: {
    type: Boolean,
    default: false
  }
})

const emit = defineEmits(['update:modelValue', 'remove'])

const popoverOpen = ref(props.autoOpen)

const summaryText = computed(() => {
  if (!props.modelValue) return props.field.label
  const label = props.field.label

  if (props.isTextField) {
    const val = props.modelValue.value
    if (!val) return label
    return `${label}: ${val}`
  }

  if (props.isDateField) {
    if (props.modelValue.operator === 'relative_date') {
      const presetLabels = {
        today: 'Today',
        yesterday: 'Yesterday',
        last_7_days: 'Last 7 days',
        last_30_days: 'Last 30 days',
        this_month: 'This month'
      }
      return `${label}: ${presetLabels[props.modelValue.value] || props.modelValue.value}`
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
    const prefix = (props.modelValue.operator === 'not_in' || props.modelValue.operator === 'not contains')
      ? 'Not: '
      : ''
    if (names.length <= 2) return `${label}: ${prefix}${names.join(', ')}`
    return `${label}: ${prefix}${names[0]}, +${names.length - 1}`
  } catch {
    return label
  }
})

function handleUpdate(filter) {
  emit('update:modelValue', filter)
}

function handleRemove() {
  popoverOpen.value = false
  emit('remove')
}

function handleRemoveClick(e) {
  e.stopPropagation()
  e.preventDefault()
  emit('remove')
}

// Only prevent focus-outside (caused by re-renders stealing focus)
// Allow pointer-down-outside (user clicking elsewhere) to close normally
function preventFocusClose(event) {
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
      <FilterTextInput
        v-else-if="isTextField"
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
