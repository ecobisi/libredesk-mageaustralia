<template>
  <div class="relative">
    <!--
      EC12: Formatting toolbar. Toggled by the "Aa" button below; collapsed by
      default to keep the menu bar quiet. Same six controls as the BubbleMenu
      (Bold / Italic / lists / Link / Image) plus a bonus Emoji slot so the
      formatting popover doubles as the entry point for emoji insertion.
    -->
    <div
      v-if="isToolbarVisible"
      class="flex items-center gap-1 px-2 py-1 border-t border-border bg-muted/30"
    >
      <Toggle class="px-2 py-1.5 h-7 border-0" variant="outline" @click="emitCommand('toggleBold')">
        <Bold class="h-3.5 w-3.5" />
      </Toggle>
      <Toggle class="px-2 py-1.5 h-7 border-0" variant="outline" @click="emitCommand('toggleItalic')">
        <Italic class="h-3.5 w-3.5" />
      </Toggle>
      <Toggle class="px-2 py-1.5 h-7 border-0" variant="outline" @click="emitCommand('toggleBulletList')">
        <List class="h-3.5 w-3.5" />
      </Toggle>
      <Toggle class="px-2 py-1.5 h-7 border-0" variant="outline" @click="emitCommand('toggleOrderedList')">
        <ListOrdered class="h-3.5 w-3.5" />
      </Toggle>
      <Toggle class="px-2 py-1.5 h-7 border-0" variant="outline" @click="emitCommand('openLink')">
        <LinkIcon class="h-3.5 w-3.5" />
      </Toggle>
      <Toggle class="px-2 py-1.5 h-7 border-0" variant="outline" @click="emitCommand('insertImage')">
        <ImageIcon class="h-3.5 w-3.5" />
      </Toggle>
      <div class="w-px h-4 bg-border mx-1" />
      <Toggle
        class="px-2 py-1.5 h-7 border-0"
        variant="outline"
        @click="toggleEmojiPicker"
        :pressed="isEmojiPickerVisible"
      >
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
      <!-- <input
        type="file"
        class="hidden"
        ref="inlineImageInput"
        accept="image/*"
        @change="handleInlineImageUpload"
      /> -->
      <!-- Editor buttons -->
      <Toggle
        class="px-2 py-2 border-0"
        variant="outline"
        @click="triggerFileUpload"
        :pressed="false"
      >
        <Paperclip class="h-4 w-4" />
      </Toggle>
      <!--
        EC12: Replaces the standalone emoji button with a formatting toggle.
        Emoji moved into the formatting toolbar above so the menu bar surfaces
        a single "show formatting" affordance rather than competing icons.
      -->
      <Toggle
        class="px-2 py-2 border-0"
        variant="outline"
        @click="isToolbarVisible = !isToolbarVisible"
        :pressed="isToolbarVisible"
        :title="$t('replyBox.formatting')"
        :aria-label="$t('replyBox.formatting')"
      >
        <ChevronUp v-if="isToolbarVisible" class="h-4 w-4" />
        <ALargeSmall v-else class="h-4 w-4" />
      </Toggle>
      <!--
        EC15: One-click macro picker. Opens the same nested command the
        Ctrl+K → Apply macro flow uses, so agents who don't know the
        shortcut can still reach canned responses. Lives next to the
        Aa formatting toggle (top-level menu bar, not inside the
        formatting toolbar) since macros are an editor-level action,
        not a typography one. The `macroPickerCommand` prop lets the
        new-conversation dialog target the `apply-macro-to-new-
        conversation` flow instead of the existing-conversation one.
      -->
      <Toggle
        class="px-2 py-2 border-0"
        variant="outline"
        @click="openMacroPicker"
        :pressed="false"
        :title="$t('replyBox.applyMacro')"
        :aria-label="$t('replyBox.applyMacro')"
      >
        <Zap class="h-4 w-4" />
      </Toggle>
    </div>
    <div class="flex items-center" v-if="showSendButton">
      <!-- Delete-draft button. Only surfaces when there's something to discard
           so the chrome doesn't add visual noise on an empty editor. -->
      <Button
        v-if="hasDraft"
        variant="ghost"
        size="sm"
        class="h-8 px-2 mr-1 text-muted-foreground hover:text-destructive"
        @click="$emit('deleteDraft')"
        :title="$t('replyBox.deleteDraft')"
      >
        <Trash2 class="h-4 w-4" />
      </Button>
      <!-- Split Send button: primary action stays on the left, status
           variants live behind a chevron on the right so an agent can
           "Send & Resolve" / "Send & Close" / etc. in one click. -->
      <div class="flex">
        <Button
          class="h-8 px-8 rounded-r-none"
          @click="handleSend"
          :disabled="!enableSend"
          :isLoading="isSending"
        >
          {{ $t('globals.messages.send') }}
        </Button>
        <DropdownMenu v-if="sendStatuses.length > 0">
          <DropdownMenuTrigger as-child>
            <Button
              class="h-8 px-1.5 rounded-l-none border-l border-primary-foreground/20"
              :disabled="!enableSend || isSending"
              :isLoading="isSending"
              :title="$t('replyBox.sendAndSetStatus')"
            >
              <ChevronDown class="h-3.5 w-3.5" />
            </Button>
          </DropdownMenuTrigger>
          <!-- UX6: width auto with min-w so long custom status names (now up
               to 50 chars) and translated "Send and set as ..." labels don't
               truncate. Smaller font + tighter padding keep the menu compact. -->
          <DropdownMenuContent align="end" class="w-auto min-w-[14rem] max-w-[24rem]">
            <DropdownMenuItem
              v-for="status in sendStatuses"
              :key="status"
              class="text-xs whitespace-nowrap py-1.5"
              @click="$emit('sendWithStatus', status)"
            >
              {{ $t('replyBox.sendAndSetAs', { status }) }}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </div>
    </div>
  </div>
</template>

<script setup>
import { ref, defineAsyncComponent } from 'vue'
import { onClickOutside } from '@vueuse/core'
import { Button } from '@shared-ui/components/ui/button'
import { Toggle } from '@shared-ui/components/ui/toggle'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger
} from '@shared-ui/components/ui/dropdown-menu'
import {
  Paperclip,
  Smile,
  ChevronDown,
  ChevronUp,
  ALargeSmall,
  Bold,
  Italic,
  List,
  ListOrdered,
  Link as LinkIcon,
  Image as ImageIcon,
  Trash2,
  Zap
} from 'lucide-vue-next'
import { useEmitter } from '@main/composables/useEmitter'
import { EMITTER_EVENTS } from '@main/constants/emitterEvents.js'

const emitter = useEmitter()

const EmojiPicker = defineAsyncComponent(async () => {
  const [mod] = await Promise.all([
    import('vue3-emoji-picker'),
    import('vue3-emoji-picker/css'),
  ])
  return mod.default
})

const attachmentInput = ref(null)
// const inlineImageInput = ref(null)
const isEmojiPickerVisible = ref(false)
// EC12: Toggle for the formatting toolbar row that sits above the menu bar.
// Collapsed by default — agents who don't need formatting controls don't see
// chrome they have to ignore.
const isToolbarVisible = ref(false)
const emojiPickerRef = ref(null)
const emit = defineEmits(['emojiSelect', 'sendWithStatus', 'deleteDraft', 'editorCommand'])

// Using defineProps for props that don't need two-way binding
const props = defineProps({
  isFullscreen: Boolean,
  isSending: Boolean,
  enableSend: Boolean,
  handleSend: Function,
  showSendButton: {
    type: Boolean,
    default: true
  },
  handleFileUpload: Function,
  handleInlineImageUpload: Function,
  // Whether the editor has anything worth discarding. Drives visibility of
  // the delete-draft button — no point surfacing it on an empty box.
  hasDraft: {
    type: Boolean,
    default: false
  },
  // Status names to expose in the "Send & set as" dropdown. Parent decides
  // which statuses are valid for this conversation (typically excludes
  // Snoozed, Spam, Trashed since those are surfaced via dedicated UI).
  sendStatuses: {
    type: Array,
    default: () => []
  },
  // EC15: Which nested command the Zap button fires. Reply box defaults to
  // the existing-conversation flow; CreateConversation overrides this with
  // 'apply-macro-to-new-conversation' so its macro picker hits the right
  // CommandBox branch.
  macroPickerCommand: {
    type: String,
    default: 'apply-macro-to-existing-conversation'
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

// EC12: Forward formatting toolbar clicks up to the parent, which then calls
// the editor's exposed runCommand(). Keeps this component presentational —
// it doesn't need a ref to the editor.
function emitCommand(command) {
  emit('editorCommand', command)
}

// EC15: Open the macro picker via the command bus the Ctrl+K flow already
// listens on (CommandBox.vue subscribes to SET_NESTED_COMMAND). The default
// 'apply-macro-to-existing-conversation' is the reply-box flow; the new-
// conversation dialog overrides via the macroPickerCommand prop.
function openMacroPicker() {
  emitter.emit(EMITTER_EVENTS.SET_NESTED_COMMAND, {
    command: props.macroPickerCommand,
    open: true
  })
}
</script>
