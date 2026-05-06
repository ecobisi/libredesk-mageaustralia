<template>
  <!--
    OVERRIDE: max-h-[600px] cap removed. The unified scroll container in
    the Conversation override means the editor can grow naturally and the
    page scrolls (v1.0.3 behaviour). Fullscreen path is unaffected — the
    Dialog sizes itself.
  -->
  <div class="flex flex-col h-full">
    <!-- Message type toggle -->
    <div
      class="flex justify-between items-center"
      :class="{ 'mb-4': !isFullscreen, 'border-b border-border pb-4': isFullscreen }"
    >
      <Tabs v-model="messageType" class="rounded border">
        <TabsList class="bg-muted p-1 rounded">
          <TabsTrigger
            value="reply"
            class="px-3 py-1 rounded transition-colors duration-200"
            :class="{ 'bg-background text-foreground': messageType === 'reply' }"
          >
            {{ $t('globals.terms.reply') }}
          </TabsTrigger>
          <TabsTrigger
            value="private_note"
            class="px-3 py-1 rounded transition-colors duration-200"
            :class="{ 'bg-background text-foreground': messageType === 'private_note' }"
          >
            {{ $t('globals.terms.privateNote') }}
          </TabsTrigger>
          <!-- The Forward tab is only visible after the user clicks Forward
               on a specific message. Switching to it from the bare list
               wouldn't make sense (there's no message to quote). -->
          <TabsTrigger
            v-if="messageType === 'forward'"
            value="forward"
            class="px-3 py-1 rounded transition-colors duration-200"
            :class="{ 'bg-background text-foreground': messageType === 'forward' }"
          >
            {{ $t('conversation.forward') }}
          </TabsTrigger>
        </TabsList>
      </Tabs>
      <!--
        EC6: Fullscreen toggle. Icon-only, so we surface the action via
        title/aria-label so screen readers and tooltip-on-hover users know
        what it does. Label flips between enter/exit based on current state.
      -->
      <Button
        class="text-muted-foreground"
        variant="ghost"
        :title="isFullscreen ? t('replyBox.fullscreen.exit') : t('replyBox.fullscreen.enter')"
        :aria-label="isFullscreen ? t('replyBox.fullscreen.exit') : t('replyBox.fullscreen.enter')"
        @click="toggleFullscreen"
      >
        <component :is="isFullscreen ? Minimize2 : Maximize2" />
      </Button>
    </div>

    <!-- From, To, CC, and BCC fields -->
    <div v-if="conversationStore.current.inbox_channel === 'email'">
      <div
        :class="['space-y-3', isFullscreen ? 'p-4 border-b border-border' : 'mb-4']"
        v-if="messageType === 'reply' || messageType === 'forward'"
      >
        <!--
          EC14: From switcher. Only renders when the inbox has at least one
          alias configured (fromOptions includes the primary + aliases).
          Empty selection means "use inbox primary" — the parent omits the
          override on send so we don't send a redundant payload field.
        -->
        <div v-if="fromOptions.length > 0" class="flex items-center space-x-2">
          <label class="w-12 text-sm font-medium text-muted-foreground">{{ $t('replyBox.from') }}:</label>
          <select
            v-model="selectedFrom"
            class="flex-grow h-9 px-3 py-1 text-sm border rounded bg-background text-foreground focus:ring-2 focus:ring-ring outline-none"
          >
            <option
              v-for="opt in fromOptions"
              :key="opt"
              :value="opt"
            >{{ opt }}</option>
          </select>
        </div>
        <!--
          EC7/EC8: Gmail-style chip inputs. Emails render as removable pills
          with per-chip remove (X). The chip-level remove fully supersedes the
          previous EC9 per-field clear-X — finer-grained, so we drop the
          all-or-nothing clear button and the wrapping div+Input it lived in.
          Model is still a comma-joined string, so validateEmails / parseTo /
          parseCC / parseBCC keep working untouched.
        -->
        <div class="flex items-center space-x-2">
          <label class="w-12 text-sm font-medium text-muted-foreground">TO:</label>
          <EmailTagInput
            v-model="to"
            :placeholder="t('replyBox.emailAddresess')"
            class="flex-grow"
            @blur="validateEmails"
          />
        </div>
        <div class="flex items-center space-x-2">
          <label class="w-12 text-sm font-medium text-muted-foreground">CC:</label>
          <EmailTagInput
            v-model="cc"
            :placeholder="t('replyBox.emailAddresess')"
            class="flex-grow"
            @blur="validateEmails"
          />
          <Button
            size="sm"
            @click="toggleBcc"
            class="text-sm bg-secondary text-secondary-foreground hover:bg-secondary/80"
          >
            {{ showBcc ? $t('replyBox.removeBCC') : $t('replyBox.bcc') }}
          </Button>
        </div>
        <div v-if="showBcc" class="flex items-center space-x-2">
          <label class="w-12 text-sm font-medium text-muted-foreground">BCC:</label>
          <EmailTagInput
            v-model="bcc"
            :placeholder="t('replyBox.emailAddresess')"
            class="flex-grow"
            @blur="validateEmails"
          />
        </div>
      </div>

      <!-- email errors -->
      <div
        v-if="emailErrors.length > 0"
        class="mb-4 px-2 py-1 bg-destructive/10 border border-destructive text-destructive rounded"
      >
        <p v-for="error in emailErrors" :key="error" class="text-sm">{{ error }}</p>
      </div>
    </div>

    <!-- Main tiptap editor -->
    <div class="flex-grow flex flex-col overflow-hidden">
      <Editor
        ref="editorRef"
        v-model:htmlContent="htmlContent"
        v-model:textContent="textContent"
        :message-type="messageType"
        :placeholder="t('editor.hint.full')"
        :aiPrompts="aiPrompts"
        :insertContent="insertContent"
        :autoFocus="true"
        :disabled="isDraftLoading"
        :enableMentions="messageType === 'private_note'"
        :getSuggestions="getSuggestions"
        @aiPromptSelected="handleAiPromptSelected"
        @send="handleSend"
        @mentionsChanged="handleMentionsChanged"
      />
    </div>

    <!-- Macro preview -->
    <MacroActionsPreview
      v-if="conversationStore.getMacro(MACRO_CONTEXT.REPLY)?.actions?.length > 0"
      :actions="conversationStore.getMacro(MACRO_CONTEXT.REPLY).actions"
      :onRemove="(action) => conversationStore.removeMacroAction(action, MACRO_CONTEXT.REPLY)"
      class="mt-2"
    />

    <!-- Attachments preview -->
    <AttachmentsPreview
      :attachments="uploadedFiles"
      :uploadingFiles="uploadingFiles"
      :onDelete="handleOnFileDelete"
      v-if="uploadedFiles.length > 0 || uploadingFiles.length > 0"
      class="mt-2"
    />

    <!-- Editor menu bar with send button -->
    <ReplyBoxMenuBar
      class="mt-1 shrink-0"
      :isFullscreen="isFullscreen"
      :handleFileUpload="handleFileUpload"
      :isSending="isSending"
      :enableSend="enableSend"
      :handleSend="handleSend"
      :hasDraft="hasDraft"
      :sendStatuses="sendStatuses"
      :macroPickerCommand="'apply-macro-to-existing-conversation'"
      @emojiSelect="handleEmojiSelect"
      @editorCommand="handleEditorCommand"
      @sendWithStatus="handleSendWithStatus"
      @deleteDraft="handleDeleteDraft"
    />
  </div>
</template>

<script setup>
import { ref, computed, nextTick, watch } from 'vue'
import { MACRO_CONTEXT } from '@main/constants/conversation'
import { Maximize2, Minimize2 } from 'lucide-vue-next'
import Editor from '@main/components/editor/TextEditor.vue'
import EmailTagInput from '@main/components/EmailTagInput.vue'
import { useConversationStore } from '@main/stores/conversation'
import { Button } from '@shared-ui/components/ui/button'
import { Tabs, TabsList, TabsTrigger } from '@shared-ui/components/ui/tabs'
import { useToast } from '@main/composables/useToast'
import AttachmentsPreview from '@/features/conversation/message/attachment/AttachmentsPreview.vue'
import MacroActionsPreview from '@/features/conversation/MacroActionsPreview.vue'
import ReplyBoxMenuBar from '@/features/conversation/ReplyBoxMenuBar.vue'
import { useI18n } from 'vue-i18n'
import { validateEmail, parseEmailList } from '@shared-ui/utils/string'
import { useMacroStore } from '@main/stores/macro'
import { useUsersStore } from '@main/stores/users'
import { useTeamStore } from '@main/stores/team'

const messageType = defineModel('messageType', { default: 'reply' })
const to = defineModel('to', { default: '' })
const cc = defineModel('cc', { default: '' })
const bcc = defineModel('bcc', { default: '' })
const showBcc = defineModel('showBcc', { default: false })
const emailErrors = defineModel('emailErrors', { default: () => [] })
const htmlContent = defineModel('htmlContent', { default: '' })
const textContent = defineModel('textContent', { default: '' })
const mentions = defineModel('mentions', { default: () => [] })
// EC14: chosen From alias (one entry from props.fromOptions). Empty
// string means "use inbox primary" — parent omits the override on send.
const selectedFrom = defineModel('selectedFrom', { default: '' })
const macroStore = useMacroStore()
const usersStore = useUsersStore()
const teamStore = useTeamStore()

// Get suggestions for the mention dropdown
const getSuggestions = async (query) => {
  // Only show suggestions in private note mode
  if (messageType.value !== 'private_note') {
    return []
  }

  await Promise.all([usersStore.fetchUsers(), teamStore.fetchTeams()])

  const q = query.toLowerCase()

  const users = usersStore.users
    .filter((u) => u.enabled)
    .filter((u) => `${u.first_name} ${u.last_name}`.toLowerCase().includes(q))
    .map((u) => ({
      id: u.id,
      type: 'agent',
      label: `${u.first_name} ${u.last_name}`.trim(),
      avatar_url: u.avatar_url
    }))

  const teams = teamStore.teams
    .filter((t) => t.name.toLowerCase().includes(q))
    .map((t) => ({
      id: t.id,
      type: 'team',
      label: t.name,
      emoji: t.emoji
    }))

  return [...users, ...teams].slice(0, 25)
}

// Handle mentions changed from editor
const handleMentionsChanged = (newMentions) => {
  mentions.value = newMentions
}

const props = defineProps({
  isFullscreen: {
    type: Boolean,
    default: false
  },
  aiPrompts: {
    type: Array,
    required: true
  },
  isSending: {
    type: Boolean,
    required: true
  },
  uploadingFiles: {
    type: Array,
    required: true
  },
  uploadedFiles: {
    type: Array,
    required: false,
    default: () => []
  },
  isDraftLoading: {
    type: Boolean,
    required: false,
    default: false
  },
  // EC1: drives the delete-draft button visibility in the menu bar.
  // Parent owns the source-of-truth (does the editor have content or
  // attached files?) so the menu bar stays presentational.
  hasDraft: {
    type: Boolean,
    default: false
  },
  // EC1: status names for the "Send & set as" dropdown. Parent filters
  // to the valid set; ReplyBoxContent just relays.
  sendStatuses: {
    type: Array,
    default: () => []
  },
  // EC14: From-switcher options. Inbox primary first then aliases.
  // Empty array hides the dropdown (no aliases configured).
  fromOptions: {
    type: Array,
    default: () => []
  }
})

const emit = defineEmits([
  'toggleFullscreen',
  'send',
  'sendWithStatus',
  'deleteDraft',
  'fileUpload',
  'inlineImageUpload',
  'fileDelete',
  'aiPromptSelected'
])

const conversationStore = useConversationStore()
const toast = useToast()
const { t } = useI18n()
const insertContent = ref(null)
const editorRef = ref(null)

const toggleBcc = async () => {
  showBcc.value = !showBcc.value
  await nextTick()
  // If hiding BCC field, clear the content and validate email bcc so it doesn't show errors.
  if (!showBcc.value) {
    bcc.value = ''
    await nextTick()
    validateEmails()
  }
}

const toggleFullscreen = () => {
  emit('toggleFullscreen')
}

const enableSend = computed(() => {
  return (
    (textContent.value.trim().length > 0 ||
      conversationStore.getMacro('reply')?.actions?.length > 0 ||
      props.uploadedFiles.length > 0) &&
    emailErrors.value.length === 0 &&
    !props.uploadingFiles.length && !props.isDraftLoading
  )
})

/**
 * Validates email addresses in To, CC, and BCC fields.
 * Populates `emailErrors` with invalid emails grouped by field.
 */
const validateEmails = async () => {
  emailErrors.value = []
  await nextTick()

  const fields = ['to', 'cc', 'bcc']
  const values = { to: to.value, cc: cc.value, bcc: bcc.value }

  fields.forEach((field) => {
    const invalid = parseEmailList(values[field]).filter((e) => !validateEmail(e))

    if (invalid.length)
      emailErrors.value.push(`${t('replyBox.invalidEmailsIn')} '${field}': ${invalid.join(', ')}`)
  })
}

/**
 * Send the reply or private note
 */
const handleSend = async () => {
  await validateEmails()
  if (emailErrors.value.length > 0) {
    toast.error(t('globals.messages.correctEmailErrors'))
    return
  }
  emit('send')
}

// EC1: Send-and-set-status variant. Same email validation guard as handleSend
// — we don't want a chevron click to bypass the recipient sanity check that
// the primary Send applies.
const handleSendWithStatus = async (status) => {
  await validateEmails()
  if (emailErrors.value.length > 0) {
    toast.error(t('globals.messages.correctEmailErrors'))
    return
  }
  emit('sendWithStatus', status)
}

const handleDeleteDraft = () => {
  emit('deleteDraft')
}

const handleFileUpload = (event) => {
  emit('fileUpload', event)
}

const handleOnFileDelete = (uuid) => {
  emit('fileDelete', uuid)
}

const handleEmojiSelect = (emoji) => {
  insertContent.value = undefined
  // Force reactivity so the user can select the same emoji multiple times
  nextTick(() => (insertContent.value = emoji))
}

// EC12: Bridge from ReplyBoxMenuBar's formatting toolbar to the editor's
// exposed runCommand(). The menu bar is presentational; the editor ref lives
// here, so this component owns the wiring.
const handleEditorCommand = (command) => {
  editorRef.value?.runCommand(command)
}

const handleAiPromptSelected = (key) => {
  emit('aiPromptSelected', key)
}

// Watch and update macro view based on message type this filters our macros.
watch(
  messageType,
  (newType, oldType) => {
    if (newType === 'reply') {
      macroStore.setCurrentView('replying')
    } else if (newType === 'private_note') {
      macroStore.setCurrentView('adding_private_note')
    }
    // Focus editor on tab change
    setTimeout(() => {
      editorRef.value?.focus()
    }, 50)
  },
  { immediate: true }
)

// Expose focus method for parent components.
// EC10: Forwards a position arg so ReplyBox.vue's conv-switch focus can opt
// for cursor-at-start (default in TextEditor.focus()) without poking at the
// editor ref directly.
const focus = (position = 'start') => {
  editorRef.value?.focus(position)
}

// FS8: Resolve the {{contact.x}} placeholder shortcut subset that v1.0.3
// supports for at-the-cursor macro insertion. Server-side macro rendering
// has its own template engine; this is a client-side fast-path so an agent
// can pick a macro mid-reply and have the contact's name land immediately
// without a round-trip. Keep this list deliberately narrow — anything more
// elaborate (case attributes, custom fields) belongs on the backend.
function replacePlaceholders (html) {
  const contact = conversationStore.current?.contact || {}
  const fullName = [contact.first_name, contact.last_name].filter(Boolean).join(' ')
  const replacements = {
    '{{contact.first_name}}': contact.first_name || '',
    '{{contact.last_name}}': contact.last_name || '',
    '{{contact.full_name}}': fullName,
    '{{contact.email}}': contact.email || '',
    '{{contact.phone}}': contact.phone_number || ''
  }
  let result = html
  for (const [placeholder, value] of Object.entries(replacements)) {
    result = result.replaceAll(placeholder, value)
  }
  return result
}

// FS8: Push a macro body through the editor's `insertContent` prop watcher.
// The undefined → nextTick → value pattern forces reactivity so applying the
// same macro twice in a row still triggers the editor watcher. TextEditor's
// watcher focuses the editor (if needed) and inserts at cursor.
function insertMacro (html) {
  insertContent.value = undefined
  nextTick(() => {
    insertContent.value = replacePlaceholders(html)
  })
}

defineExpose({ focus, insertMacro })
</script>
