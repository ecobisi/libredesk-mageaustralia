<script setup>
// Horizontal pill-bar style filter UI for the conversation list.
//
// Each active filter renders as a pill (FilterPill). The "+ Filter" trigger
// opens a field picker (FilterFieldPicker) that adds a new pill. Pill values
// are edited in a popover hosted by FilterPill itself (FilterMultiSelect or
// FilterDatePicker depending on the field type).
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Popover, PopoverTrigger, PopoverContent } from '@shared-ui/components/ui/popover'
import { Button } from '@shared-ui/components/ui/button'
import { Plus, X } from 'lucide-vue-next'
import FilterFieldPicker from './FilterFieldPicker.vue'
import FilterPill from './FilterPill.vue'

const DATE_FIELDS = [
  'created_at',
  'last_message_at',
  'last_interaction_at',
  'waiting_since',
  'next_sla_deadline_at',
  'closed_at',
  'resolved_at'
]

const props = defineProps({
  fields: {
    type: Array,
    required: true
  },
  modelValue: {
    type: Array,
    default: () => []
  }
})

const emit = defineEmits(['update:modelValue'])
const { t } = useI18n()

const pickerOpen = ref(false)
// Track the most recently added field key so its pill auto-opens its
// editor popover. Cleared as soon as the user makes a selection or removes
// the pill, otherwise the popover would re-open on every parent re-render.
const newestFieldKey = ref(null)

const activeFieldKeys = computed(() => {
  return props.modelValue.map((f) => f.field)
})

const availableFields = computed(() => {
  const active = new Set(activeFieldKeys.value)
  return props.fields.filter((f) => !active.has(f.key))
})

function isDateField (fieldKey) {
  return DATE_FIELDS.includes(fieldKey)
}

function getFieldDef (fieldKey) {
  return props.fields.find((f) => f.key === fieldKey) || { key: fieldKey, label: fieldKey }
}

function handleFieldSelect (field) {
  pickerOpen.value = false
  if (activeFieldKeys.value.includes(field.key)) return

  newestFieldKey.value = field.key

  // Tag filters use the dedicated `contains` / `not contains` operators
  // because the SQL builder's tag handling is keyed on those names. Other
  // multi-selects use the generic `in` / `not_in` operators.
  const isTagField = field.key === 'tags'
  const newFilter = {
    field: field.key,
    operator: isDateField(field.key) ? 'relative_date' : (isTagField ? 'contains' : 'in'),
    value: isDateField(field.key) ? 'last_7_days' : '[]',
    model: field.model || ''
  }
  emit('update:modelValue', [...props.modelValue, newFilter])
}

function handleFilterUpdate (index, filter) {
  const updated = [...props.modelValue]
  updated[index] = filter
  newestFieldKey.value = null
  emit('update:modelValue', updated)
}

function handleFilterRemove (index) {
  newestFieldKey.value = null
  const updated = props.modelValue.filter((_, i) => i !== index)
  emit('update:modelValue', updated)
}

function handleClearAll () {
  newestFieldKey.value = null
  emit('update:modelValue', [])
}
</script>

<template>
  <div v-if="fields.length > 0" class="flex flex-wrap items-center gap-1.5">
    <!-- Active filter pills -->
    <FilterPill
      v-for="(filter, index) in modelValue"
      :key="filter.field"
      :field="getFieldDef(filter.field)"
      :model-value="filter"
      :is-date-field="isDateField(filter.field)"
      :auto-open="filter.field === newestFieldKey && !isDateField(filter.field)"
      @update:model-value="handleFilterUpdate(index, $event)"
      @remove="handleFilterRemove(index)"
    />

    <!-- Add filter button -->
    <Popover v-if="availableFields.length > 0" v-model:open="pickerOpen">
      <PopoverTrigger as-child>
        <Button variant="ghost" size="xs" class="gap-1 text-muted-foreground">
          <Plus class="h-3.5 w-3.5" />
          {{ t('filter.add') }}
        </Button>
      </PopoverTrigger>
      <PopoverContent align="start" :side-offset="4" class="p-0 w-auto">
        <FilterFieldPicker
          :fields="availableFields"
          :active-field-keys="activeFieldKeys"
          @select="handleFieldSelect"
          @close="pickerOpen = false"
        />
      </PopoverContent>
    </Popover>

    <!-- Clear all -->
    <Button
      v-if="modelValue.length > 0"
      variant="ghost"
      size="xs"
      class="gap-1 text-muted-foreground"
      @click="handleClearAll"
    >
      <X class="h-3.5 w-3.5" />
      {{ t('filter.clearAll') }}
    </Button>
  </div>
</template>
