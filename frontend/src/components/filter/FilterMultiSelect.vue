<script setup>
import { ref, computed, watch } from 'vue'
import { Checkbox } from '@/components/ui/checkbox'
import { Button } from '@/components/ui/button'

const props = defineProps({
  field: {
    type: Object,
    required: true
  },
  modelValue: {
    type: Object,
    default: null
  }
})

const emit = defineEmits(['update:modelValue', 'remove'])

const searchQuery = ref('')

// Determine operators based on field type
const includeOp = computed(() => props.field.key === 'tags' ? 'contains' : 'in')
const excludeOp = computed(() => props.field.key === 'tags' ? 'not contains' : 'not_in')

// Parse initial state from modelValue
const mode = ref('include')
const selected = ref([])

function initFromModelValue() {
  if (props.modelValue) {
    const op = props.modelValue.operator
    if (op === excludeOp.value) {
      mode.value = 'exclude'
    } else {
      mode.value = 'include'
    }
    try {
      const parsed = JSON.parse(props.modelValue.value)
      selected.value = Array.isArray(parsed) ? parsed : []
    } catch {
      selected.value = []
    }
  } else {
    mode.value = 'include'
    selected.value = []
  }
}

initFromModelValue()

watch(() => props.modelValue, initFromModelValue, { deep: true })

const filteredOptions = computed(() => {
  if (!searchQuery.value) return props.field.options || []
  const q = searchQuery.value.toLowerCase()
  return (props.field.options || []).filter((opt) =>
    opt.label.toLowerCase().includes(q)
  )
})

function isSelected(value) {
  return selected.value.includes(String(value))
}

function toggleOption(value) {
  const strVal = String(value)
  const idx = selected.value.indexOf(strVal)
  if (idx >= 0) {
    selected.value.splice(idx, 1)
  } else {
    selected.value.push(strVal)
  }
  emitUpdate()
}

function setMode(newMode) {
  mode.value = newMode
  if (selected.value.length > 0) {
    emitUpdate()
  }
}

function emitUpdate() {
  emit('update:modelValue', {
    field: props.field.key,
    operator: mode.value === 'include' ? includeOp.value : excludeOp.value,
    value: JSON.stringify(selected.value),
    model: props.field.model || ''
  })
}

function handleClear() {
  selected.value = []
  emit('remove')
}
</script>

<template>
  <div class="w-64">
    <!-- Mode toggle (segmented control) -->
    <div class="flex border-b mb-2">
      <button
        class="flex-1 py-1.5 text-xs font-medium text-center transition-colors"
        :class="mode === 'include'
          ? 'text-primary border-b-2 border-primary'
          : 'text-muted-foreground hover:text-foreground'"
        @click="setMode('include')"
      >
        Is any of
      </button>
      <button
        class="flex-1 py-1.5 text-xs font-medium text-center transition-colors"
        :class="mode === 'exclude'
          ? 'text-primary border-b-2 border-primary'
          : 'text-muted-foreground hover:text-foreground'"
        @click="setMode('exclude')"
      >
        Is none of
      </button>
    </div>

    <!-- Search -->
    <div class="px-2 pb-2">
      <input
        v-model="searchQuery"
        type="text"
        placeholder="Search..."
        class="w-full h-8 px-2 text-sm border rounded-md bg-transparent outline-none focus:ring-1 focus:ring-ring"
      />
    </div>

    <!-- Options list -->
    <div class="max-h-48 overflow-y-auto px-1">
      <label
        v-for="opt in filteredOptions"
        :key="opt.value"
        class="flex items-center gap-2 px-2 py-1.5 rounded-sm text-sm cursor-pointer hover:bg-accent"
      >
        <Checkbox
          :checked="isSelected(opt.value)"
          @update:checked="toggleOption(opt.value)"
        />
        <span class="truncate">{{ opt.label }}</span>
      </label>
      <div v-if="filteredOptions.length === 0" class="py-4 text-center text-sm text-muted-foreground">
        No options found.
      </div>
    </div>

    <!-- Footer -->
    <div class="flex justify-end border-t mt-2 pt-2 px-2 pb-1">
      <Button variant="ghost" size="xs" @click="handleClear">
        Clear
      </Button>
    </div>
  </div>
</template>
