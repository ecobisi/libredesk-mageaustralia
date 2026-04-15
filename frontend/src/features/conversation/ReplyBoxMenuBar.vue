<template>
  <div class="relative">
    <!-- Formatting toolbar (toggled) -->
    <div v-if="isToolbarVisible" class="flex items-center gap-1 px-2 py-1 border-t border-border bg-muted/30">
      <Toggle class="px-2 py-1.5 h-7 border-0" variant="outline" :pressed="isBoldActive" @click="emitCommand('toggleBold')">
        <Bold class="h-3.5 w-3.5" />
      </Toggle>
      <Toggle class="px-2 py-1.5 h-7 border-0" variant="outline" :pressed="isItalicActive" @click="emitCommand('toggleItalic')">
        <Italic class="h-3.5 w-3.5" />
      </Toggle>
      <Toggle class="px-2 py-1.5 h-7 border-0" variant="outline" :pressed="isUnderlineActive" @click="emitCommand('toggleUnderline')">
        <UnderlineIcon class="h-3.5 w-3.5" />
      </Toggle>
      <Toggle class="px-2 py-1.5 h-7 border-0" variant="outline" :pressed="isStrikeActive" @click="emitCommand('toggleStrike')">
        <Strikethrough class="h-3.5 w-3.5" />
      </Toggle>
      <DropdownMenu>
        <DropdownMenuTrigger as-child>
          <Toggle class="px-2 py-1.5 h-7 border-0" variant="outline">
            <Palette class="h-3.5 w-3.5" />
          </Toggle>
        </DropdownMenuTrigger>
        <DropdownMenuContent class="p-2 w-auto">
          <div class="text-xs text-muted-foreground mb-1">Text color</div>
          <div class="grid grid-cols-8 gap-1">
            <button v-for="c in paletteColors" :key="'fg-' + c"
              class="w-5 h-5 rounded border border-gray-300 hover:scale-110 transition"
              :style="{ backgroundColor: c }"
              @click.prevent="emitCommand('setColor', c)" />
            <button class="w-5 h-5 rounded border border-gray-300 text-xs" title="Clear"
              @click.prevent="emitCommand('unsetColor')">×</button>
          </div>
        </DropdownMenuContent>
      </DropdownMenu>
      <DropdownMenu>
        <DropdownMenuTrigger as-child>
          <Toggle class="px-2 py-1.5 h-7 border-0" variant="outline">
            <Highlighter class="h-3.5 w-3.5" />
          </Toggle>
        </DropdownMenuTrigger>
        <DropdownMenuContent class="p-2 w-auto">
          <div class="text-xs text-muted-foreground mb-1">Highlight</div>
          <div class="grid grid-cols-8 gap-1">
            <button v-for="c in highlightColors" :key="'bg-' + c"
              class="w-5 h-5 rounded border border-gray-300 hover:scale-110 transition"
              :style="{ backgroundColor: c }"
              @click.prevent="emitCommand('toggleHighlight', c)" />
            <button class="w-5 h-5 rounded border border-gray-300 text-xs" title="Clear"
              @click.prevent="emitCommand('unsetHighlight')">×</button>
          </div>
        </DropdownMenuContent>
      </DropdownMenu>
      <Toggle class="px-2 py-1.5 h-7 border-0" variant="outline" @click="emitCommand('setTextAlign', 'left')">
        <AlignLeft class="h-3.5 w-3.5" />
      </Toggle>
      <Toggle class="px-2 py-1.5 h-7 border-0" variant="outline" @click="emitCommand('setTextAlign', 'center')">
        <AlignCenter class="h-3.5 w-3.5" />
      </Toggle>
      <Toggle class="px-2 py-1.5 h-7 border-0" variant="outline" @click="emitCommand('setTextAlign', 'right')">
        <AlignRight class="h-3.5 w-3.5" />
      </Toggle>
      <Toggle class="px-2 py-1.5 h-7 border-0" variant="outline" :pressed="isBulletActive" @click="emitCommand('toggleBulletList')">
        <List class="h-3.5 w-3.5" />
      </Toggle>
      <Toggle class="px-2 py-1.5 h-7 border-0" variant="outline" :pressed="isOrderedActive" @click="emitCommand('toggleOrderedList')">
        <ListOrdered class="h-3.5 w-3.5" />
      </Toggle>
      <Toggle class="px-2 py-1.5 h-7 border-0" variant="outline" @click="emitCommand('openLink')">
        <LinkIcon class="h-3.5 w-3.5" />
      </Toggle>
      <Toggle class="px-2 py-1.5 h-7 border-0" variant="outline" @click="emitCommand('insertImage')">
        <ImageIcon class="h-3.5 w-3.5" />
      </Toggle>
      <div class="w-px h-4 bg-border mx-1" />
      <Toggle class="px-2 py-1.5 h-7 border-0" variant="outline" @click="toggleEmojiPicker" :pressed="isEmojiPickerVisible">
        <Smile class="h-3.5 w-3.5" />
      </Toggle>
    </div>
    <EmojiPicker
      ref="emojiPickerRef"
      :native="true"
      @select="onSelectEmoji"
      class="absolute bottom-14 left-14"
      v-if="isEmojiPickerVisible"
    />
    <div
      class="flex justify-between h-14"
      :class="{ 'items-end': isFullscreen, 'items-center': !isFullscreen }"
    >
    <div class="flex justify-items-start gap-2">
      <!-- File inputs -->
      <input type="file" class="hidden" ref="attachmentInput" multiple @change="handleFileUpload" />
      <!-- Editor buttons -->
      <Toggle
        class="px-2 py-2 border-0"
        variant="outline"
        @click="triggerFileUpload"
        :pressed="false"
      >
        <Paperclip class="h-4 w-4" />
      </Toggle>
      <Toggle
        class="px-2 py-2 border-0"
        variant="outline"
        @click="isToolbarVisible = !isToolbarVisible"
        :pressed="isToolbarVisible"
      >
        <ChevronUp v-if="isToolbarVisible" class="h-4 w-4" />
        <ALargeSmall v-else class="h-4 w-4" />
      </Toggle>
      <Toggle
        class="px-2 py-2 border-0"
        variant="outline"
        @click="openMacroPicker"
        :pressed="false"
      >
        <Zap class="h-4 w-4" />
      </Toggle>
      <!-- Generate Response Button -->
      <Button
        v-if="showGenerateButton"
        type="button"
        variant="outline"
        size="sm"
        class="h-8 px-3 text-xs"
        @click="handleGenerate"
        :disabled="isGenerating"
      >
        <Sparkles class="h-3.5 w-3.5 mr-1.5" :class="{ 'animate-pulse': isGenerating }" />
        {{ isGenerating ? 'Generating...' : generateLabel }}
      </Button>
      <!-- Generate with Orders Button (only shows when ecommerce is configured) -->
      <Button
        v-if="showGenerateButton && showOrdersButton"
        type="button"
        variant="outline"
        size="sm"
        class="h-8 px-3 text-xs"
        @click="handleGenerateWithOrders"
        :disabled="isGenerating"
      >
        <ShoppingCart class="h-3.5 w-3.5 mr-1.5" :class="{ 'animate-pulse': isGenerating }" />
        {{ isGenerating ? 'Generating...' : '+ Orders' }}
      </Button>
    </div>
    <div class="flex items-center" v-if="showSendButton">
      <!-- Delete draft button -->
      <Button
        v-if="hasDraft"
        variant="ghost"
        size="sm"
        class="h-8 px-2 mr-1 text-muted-foreground hover:text-destructive"
        @click="$emit('deleteDraft')"
        title="Delete draft"
      >
        <Trash2 class="h-4 w-4" />
      </Button>
      <!-- Split send button with status dropdown -->
      <div class="flex">
        <Button class="h-8 px-8 rounded-r-none" @click="handleSend" :disabled="!enableSend" :isLoading="isSending">
          {{ messageType === 'private_note' ? 'Add note' : $t('globals.messages.send') }}
        </Button>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button class="h-8 px-1.5 rounded-l-none border-l border-primary-foreground/20" :disabled="!enableSend || isSending">
              <ChevronDown class="h-3.5 w-3.5" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" class="w-auto min-w-[20rem]">
            <DropdownMenuItem
              v-for="status in sendStatuses"
              :key="status"
              @click="$emit('sendWithStatus', status)" class="text-xs whitespace-nowrap py-1.5"
            >
              {{ messageType === 'private_note' ? 'Add note' : 'Send' }} and set as {{ status }}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onBeforeUnmount } from 'vue'
import { onClickOutside } from '@vueuse/core'
import { Button } from '@/components/ui/button'
import { Toggle } from '@/components/ui/toggle'
import { Paperclip, Smile, Sparkles, ShoppingCart, Zap, ChevronDown, ChevronUp, ALargeSmall, Bold, Italic, Underline as UnderlineIcon, Strikethrough, Palette, Highlighter, AlignLeft, AlignCenter, AlignRight, List, ListOrdered, Link as LinkIcon, Image as ImageIcon, Trash2 } from 'lucide-vue-next'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger
} from '@/components/ui/dropdown-menu'
import EmojiPicker from 'vue3-emoji-picker'
import 'vue3-emoji-picker/css'
import { useEmitter } from '@/composables/useEmitter'
import { EMITTER_EVENTS } from '@/constants/emitterEvents'

const emitter = useEmitter()
const attachmentInput = ref(null)
const isEmojiPickerVisible = ref(false)
const isToolbarVisible = ref(false)
const emojiPickerRef = ref(null)
const emit = defineEmits(['emojiSelect', 'generateResponse', 'generateWithOrders', 'sendWithStatus', 'deleteDraft', 'editorCommand'])

// Using defineProps for props that don't need two-way binding
const props = defineProps({
  editor: { type: Object, default: null },
  isFullscreen: Boolean,
  isSending: Boolean,
  isGenerating: {
    type: Boolean,
    default: false
  },
  enableSend: Boolean,
  handleSend: Function,
  showSendButton: {
    type: Boolean,
    default: true
  },
  showGenerateButton: {
    type: Boolean,
    default: true
  },
  generateLabel: {
    type: String,
    default: 'Generate Response'
  },
  showOrdersButton: {
    type: Boolean,
    default: false
  },
  handleFileUpload: Function,
  handleInlineImageUpload: Function,
  messageType: {
    type: String,
    default: 'reply'
  },
  hasDraft: {
    type: Boolean,
    default: false
  },
  sendStatuses: {
    type: Array,
    default: () => ['Resolved', 'Closed', 'Open']
  }
})

onClickOutside(emojiPickerRef, () => {
  isEmojiPickerVisible.value = false
})

const triggerFileUpload = () => {
  if (attachmentInput.value) {
    // Clear the value to allow the same file to be uploaded again.
    attachmentInput.value.value = ''
    attachmentInput.value.click()
  }
}

const toggleEmojiPicker = () => {
  isEmojiPickerVisible.value = !isEmojiPickerVisible.value
}

function onSelectEmoji(emoji) {
  emit('emojiSelect', emoji.i)
}

function handleGenerate() {
  emit('generateResponse')
}

function handleGenerateWithOrders() {
  emit('generateWithOrders')
}

function emitCommand(command, arg) {
  emit('editorCommand', { command, arg })
}

// Reactive editor-state tracking for toolbar button active states.
// The Tiptap editor instance isn't auto-reactive when passed across components,
// so we bump a tick ref on every selection/transaction and drive computeds from it.
const editorTick = ref(0)
let attachedEditor = null
const attachListener = (ed) => {
  if (attachedEditor === ed) return
  if (attachedEditor) {
    attachedEditor.off('selectionUpdate', onEditorTick)
    attachedEditor.off('transaction', onEditorTick)
  }
  attachedEditor = ed || null
  if (attachedEditor) {
    attachedEditor.on('selectionUpdate', onEditorTick)
    attachedEditor.on('transaction', onEditorTick)
    editorTick.value++
  }
}
const onEditorTick = () => { editorTick.value++ }
watch(() => props.editor, attachListener, { immediate: true })
onBeforeUnmount(() => attachListener(null))

const isBulletActive = computed(() => { editorTick.value; return !!props.editor?.isActive('bulletList') })
const isOrderedActive = computed(() => { editorTick.value; return !!props.editor?.isActive('orderedList') })
const isBoldActive = computed(() => { editorTick.value; return !!props.editor?.isActive('bold') })
const isItalicActive = computed(() => { editorTick.value; return !!props.editor?.isActive('italic') })
const isUnderlineActive = computed(() => { editorTick.value; return !!props.editor?.isActive('underline') })
const isStrikeActive = computed(() => { editorTick.value; return !!props.editor?.isActive('strike') })

const paletteColors = [
  '#000000', '#434343', '#666666', '#999999', '#b7b7b7', '#cccccc', '#d9d9d9', '#ffffff',
  '#980000', '#ff0000', '#ff9900', '#ffff00', '#00ff00', '#00ffff', '#4a86e8', '#0000ff',
  '#9900ff', '#ff00ff', '#e6b8af', '#f4cccc', '#fce5cd', '#fff2cc', '#d9ead3', '#d0e0e3',
  '#c9daf8', '#cfe2f3', '#d9d2e9', '#ead1dc'
]
const highlightColors = [
  '#ffff00', '#ffcc00', '#ff9900', '#ff6600', '#ff0000', '#ff00ff', '#9900ff', '#0000ff',
  '#00ffff', '#00ff00', '#99cc00', '#cccccc', '#fce5cd', '#fff2cc', '#d9ead3', '#c9daf8'
]

function openMacroPicker() {
  emitter.emit(EMITTER_EVENTS.SET_NESTED_COMMAND, {
    command: 'apply-macro-to-existing-conversation',
    open: true
  })
}
</script>
