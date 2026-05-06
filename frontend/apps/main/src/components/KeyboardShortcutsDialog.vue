<template>
  <Dialog :open="open" @update:open="$emit('update:open', $event)">
    <DialogContent class="sm:max-w-2xl max-h-[85vh] overflow-y-auto">
      <DialogHeader>
        <DialogTitle>{{ t('shortcuts.title') }}</DialogTitle>
      </DialogHeader>
      <div class="grid grid-cols-2 gap-x-8 gap-y-1 text-sm">
        <template v-for="section in sections" :key="section.title">
          <div class="col-span-2 font-semibold text-xs uppercase tracking-wide text-muted-foreground mt-4 first:mt-2 mb-1 border-b pb-1">
            {{ section.title }}
          </div>
          <div
            v-for="(item, i) in section.items"
            :key="i"
            class="flex justify-between items-center py-1"
          >
            <span>{{ item.desc }}</span>
            <span class="flex gap-1 ml-4 shrink-0">
              <kbd
                v-for="(k, ki) in item.keys"
                :key="ki"
                class="px-1.5 py-0.5 text-xs font-mono bg-muted border rounded shadow-sm"
              >{{ k }}</kbd>
            </span>
          </div>
        </template>
      </div>
      <p class="text-xs text-muted-foreground mt-4">
        {{ t('shortcuts.dialogFooter', { meta: metaKey }) }}
      </p>
    </DialogContent>
  </Dialog>
</template>

<script setup>
import { computed } from 'vue'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle
} from '@shared-ui/components/ui/dialog'
import { useI18n } from 'vue-i18n'

defineProps({
  open: { type: Boolean, default: false }
})
defineEmits(['update:open'])

const { t } = useI18n()

const isMac = navigator.platform?.toLowerCase().includes('mac')
const metaKey = computed(() => isMac ? 'Cmd' : 'Ctrl')

const sections = computed(() => [
  {
    title: t('shortcuts.section.navigation'),
    items: [
      { desc: t('shortcuts.toggleSidebar'), keys: [metaKey.value, 'B'] },
      { desc: t('shortcuts.commandPalette'), keys: [metaKey.value, 'K'] },
      { desc: t('shortcuts.search'), keys: ['/'] }
    ]
  },
  {
    title: t('shortcuts.section.conversation'),
    items: [
      { desc: t('shortcuts.reply'), keys: ['R'] },
      { desc: t('shortcuts.note'), keys: ['N'] },
      { desc: t('shortcuts.send'), keys: [metaKey.value, 'Enter'] }
    ]
  },
  {
    title: t('shortcuts.section.formatting'),
    items: [
      { desc: t('shortcuts.bold'), keys: [metaKey.value, 'B'] },
      { desc: t('shortcuts.italic'), keys: [metaKey.value, 'I'] },
      { desc: t('shortcuts.underline'), keys: [metaKey.value, 'U'] },
      { desc: t('shortcuts.strikethrough'), keys: [metaKey.value, 'Shift', 'X'] },
      { desc: t('shortcuts.code'), keys: [metaKey.value, 'E'] },
      { desc: t('shortcuts.bulletList'), keys: [metaKey.value, 'Shift', '8'] },
      { desc: t('shortcuts.orderedList'), keys: [metaKey.value, 'Shift', '7'] },
      { desc: t('shortcuts.blockquote'), keys: [metaKey.value, 'Shift', 'B'] },
      { desc: t('shortcuts.undo'), keys: [metaKey.value, 'Z'] },
      { desc: t('shortcuts.redo'), keys: [metaKey.value, 'Shift', 'Z'] }
    ]
  },
  {
    title: t('shortcuts.section.other'),
    items: [
      { desc: t('shortcuts.selectRange'), keys: ['Shift', 'Click'] },
      { desc: t('shortcuts.showDialog'), keys: ['?'] }
    ]
  }
])
</script>
