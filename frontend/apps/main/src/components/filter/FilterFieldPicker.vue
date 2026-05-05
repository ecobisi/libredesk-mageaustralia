<script setup>
// Field picker shown when "+ Filter" is clicked. Lists fields not already
// active so each filter pill is unique per field.
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  Command,
  CommandInput,
  CommandList,
  CommandEmpty,
  CommandGroup,
  CommandItem
} from '@shared-ui/components/ui/command'

const props = defineProps({
  fields: {
    type: Array,
    required: true
  },
  activeFieldKeys: {
    type: Array,
    default: () => []
  }
})

const emit = defineEmits(['select', 'close'])
const { t } = useI18n()

const searchTerm = ref('')

const availableFields = computed(() => {
  return props.fields.filter((f) => !props.activeFieldKeys.includes(f.key))
})

function handleSelect (field) {
  emit('select', field)
}
</script>

<template>
  <Command
    v-model:search-term="searchTerm"
    class="w-56"
    :filter-function="(list, term) => list.filter((item) => {
      const field = availableFields.find((f) => f.key === item)
      return field && field.label.toLowerCase().includes(term.toLowerCase())
    })"
  >
    <CommandInput :placeholder="t('filter.searchFields')" />
    <CommandList>
      <CommandEmpty>{{ t('filter.noFields') }}</CommandEmpty>
      <CommandGroup>
        <CommandItem
          v-for="field in availableFields"
          :key="field.key"
          :value="field.key"
          @select="handleSelect(field)"
        >
          {{ field.label }}
        </CommandItem>
      </CommandGroup>
    </CommandList>
  </Command>
</template>
