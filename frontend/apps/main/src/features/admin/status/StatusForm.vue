<template>
  <form>
    <FormField v-slot="{ componentField }" name="name">
      <FormItem>
        <FormLabel>{{ $t('globals.terms.name') }}</FormLabel>
        <FormControl>
          <Input type="text" placeholder="Spam" v-bind="componentField" />
        </FormControl>
        <FormMessage />
      </FormItem>
    </FormField>

    <FormField v-slot="{ componentField }" name="category">
      <FormItem class="mt-4">
        <FormLabel>{{ $t('globals.terms.category') }}</FormLabel>
        <FormControl>
          <Select v-bind="componentField">
            <SelectTrigger class="w-full">
              <SelectValue :placeholder="$t('admin.conversationStatus.category.placeholder')" />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                <SelectItem value="open">{{ $t('globals.terms.open') }}</SelectItem>
                <SelectItem value="waiting">{{ $t('globals.terms.waiting') }}</SelectItem>
                <SelectItem value="resolved">{{ $t('globals.terms.resolved') }}</SelectItem>
              </SelectGroup>
            </SelectContent>
          </Select>
        </FormControl>
        <FormMessage />
      </FormItem>
    </FormField>

    <!-- Colour picker: emits the palette key (e.g. "blue"); backend resolves it
         to bg/text via the shared frontend palette in constants/statusColors.js. -->
    <FormField v-slot="{ componentField }" name="color">
      <FormItem class="mt-4">
        <FormLabel>{{ $t('globals.terms.color') }}</FormLabel>
        <FormControl>
          <input type="hidden" v-bind="componentField" />
        </FormControl>
        <Popover>
          <PopoverTrigger asChild>
            <button
              type="button"
              class="h-9 px-3 text-sm border rounded cursor-pointer flex items-center gap-2 w-full"
              :style="previewStyle(componentField.modelValue)"
            >
              <span
                class="w-3 h-3 rounded-full border border-black/10"
                :style="{ backgroundColor: getStatusColor(componentField.modelValue || 'gray').text }"
              ></span>
              <span>{{ labelOf(componentField.modelValue || 'gray') }}</span>
              <ChevronDown class="w-3.5 h-3.5 ml-auto opacity-50" />
            </button>
          </PopoverTrigger>
          <PopoverContent class="w-[200px] p-1" align="start">
            <button
              v-for="opt in STATUS_COLOR_OPTIONS"
              :key="opt.value"
              type="button"
              class="flex items-center gap-2 w-full px-2 py-1.5 text-xs rounded hover:bg-muted cursor-pointer"
              :class="{ 'bg-muted': (componentField.modelValue || 'gray') === opt.value }"
              @click="componentField['onUpdate:modelValue'](opt.value)"
            >
              <span
                class="w-4 h-4 rounded border border-black/10"
                :style="{ backgroundColor: opt.bg }"
              ></span>
              <span :style="{ color: opt.text, fontWeight: (componentField.modelValue || 'gray') === opt.value ? 600 : 400 }">
                {{ opt.label }}
              </span>
            </button>
          </PopoverContent>
        </Popover>
        <FormMessage />
      </FormItem>
    </FormField>

    <slot name="footer"></slot>
  </form>
</template>

<script setup>
import { ChevronDown } from 'lucide-vue-next'
import {
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage
} from '@shared-ui/components/ui/form'
import { Input } from '@shared-ui/components/ui/input'
import {
  Popover,
  PopoverContent,
  PopoverTrigger
} from '@shared-ui/components/ui/popover'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue
} from '@shared-ui/components/ui/select'
import { STATUS_COLOR_OPTIONS, getStatusColor, statusColorStyle } from '@/constants/statusColors'

const previewStyle = (key) => statusColorStyle(key || 'gray')
const labelOf = (key) => getStatusColor(key).label
</script>
