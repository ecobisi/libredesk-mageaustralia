<template>
  <!-- Set fixed width only when not in fullscreen. -->
  <div class="flex flex-col h-full" :class="{ 'max-h-[600px]': !isFullscreen }">
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
          <TabsTrigger
            v-if="messageType === 'forward'"
            value="forward"
            class="px-3 py-1 rounded transition-colors duration-200"
            :class="{ 'bg-background text-foreground': messageType === 'forward' }"
          >
            Forward
          </TabsTrigger>
        </TabsList>
      </Tabs>
      <Button class="text-muted-foreground" variant="ghost" @click="toggleFullscreen">
        <component :is="isFullscreen ? Minimize2 : Maximize2" />
      </Button>
    </div>

    <!-- From, To, CC, and BCC fields (email only) -->
    <div v-if="!isEmailChannel && (messageType === 'reply')" :class="['px-2 py-1 mb-3 rounded bg-muted/50']">
      <p class="text-xs text-muted-foreground">
        <span class="font-medium">{{ conversationStore.current?.inbox_channel === 'instagram' ? 'Instagram' : 'Messenger' }}</span>
        &mdash; plain text only, no formatting
      </p>
    </div>
    <div
      :class="['space-y-3', isFullscreen ? 'p-4 border-b border-border' : 'mb-4']"
      v-if="isEmailChannel && (messageType === 'reply' || messageType === 'forward')"
    >
      <!-- From inbox selector -->
      <div class="flex items-center space-x-2" v-if="inboxes.length > 1">
        <label class="w-12 text-sm font-medium text-muted-foreground">FROM:</label>
        <select
          v-model="localSelectedInbox"
          class="flex-grow h-9 px-3 py-1 text-sm border rounded bg-background text-foreground focus:ring-2 focus:ring-ring outline-none"
        >
          <option
            v-for="inbox in inboxes"
            :key="inbox.id"
            :value="String(inbox.id)"
          >
            {{ inbox.name }} ({{ inbox.from }})
          </option>
        </select>
      </div>
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
          {{ showBcc ? 'Remove BCC' : 'BCC' }}
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

    <!-- Main tiptap editor -->
    <div class="flex-grow flex flex-col overflow-hidden">
      <Editor
        ref="editorRef"
        v-model:htmlContent="htmlContent"
        v-model:textContent="textContent"
        :placeholder="t('editor.newLine') + t('editor.send') + t('editor.ctrlK')"
        :aiPrompts="aiPrompts"
        :insertContent="insertContent"
        :autoFocus="true"
        :disabled="isDraftLoading"
        :enableMentions="messageType === 'private_note'"
        :getSuggestions="getSuggestions"
        @aiPromptSelected="handleAiPromptSelected"
        @send="handleSend"
        @mentionsChanged="handleMentionsChanged"
        @filesDropped="handleDroppedFiles"
      />
    </div>

    <!-- Quoted thread toggle -->
    <div v-if="quotedThreadHtml && (messageType === 'reply' || messageType === 'forward')" class="px-1">
      <button
        @click="threadExpanded = !threadExpanded"
        class="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
        type="button"
      >
        <span v-if="!threadExpanded" style="background-color:#f1f1f1;border:1px solid #ddd;line-height:1rem;display:inline-block;width:35px;text-align:center;border-radius:1rem;font-size:14px;color:#666;">&middot;&middot;&middot;</span>
        <span v-else>&#9662; Hide quoted text</span>
      </button>
      <div
        v-if="threadExpanded"
        contenteditable="true"
        class="text-sm text-muted-foreground border-l-2 border-muted pl-3 mt-1 mb-2 max-h-60 overflow-y-auto outline-none"
        v-html="quotedThreadHtml"
        @input="quotedThreadHtml = $event.target.innerHTML"
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
      :isGenerating="isGenerating"
      :enableSend="enableSend"
      :handleSend="handleSend"
      :messageType="messageType"
      :showGenerateButton="messageType === 'reply' || messageType === 'forward'"
      :showOrdersButton="ecommerceConfigured"
      :hasDraft="hasDraft"
      :sendStatuses="sendStatuses"
      @emojiSelect="handleEmojiSelect"
      @editorCommand="handleEditorCommand"
      @generateResponse="handleGenerateResponse"
      @generateWithOrders="handleGenerateWithOrders"
      @sendWithStatus="handleSendWithStatus"
      @deleteDraft="handleDeleteDraft"
    />
  </div>
</template>

<script setup>
import { ref, computed, nextTick, watch } from 'vue'
import { EMITTER_EVENTS } from '@/constants/emitterEvents.js'
import { MACRO_CONTEXT } from '@/constants/conversation'
import { Maximize2, Minimize2, X } from 'lucide-vue-next'
import Editor from '@/components/editor/TextEditor.vue'
import { useConversationStore } from '@/stores/conversation'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { useEmitter } from '@/composables/useEmitter'
import AttachmentsPreview from '@/features/conversation/message/attachment/AttachmentsPreview.vue'
import MacroActionsPreview from '@/features/conversation/MacroActionsPreview.vue'
import ReplyBoxMenuBar from '@/features/conversation/ReplyBoxMenuBar.vue'
import { useI18n } from 'vue-i18n'
import { validateEmail } from '@/utils/strings'
import { useMacroStore } from '@/stores/macro'
import { useUsersStore } from '@/stores/users'
import { useTeamStore } from '@/stores/team'
import EmailTagInput from '@/components/EmailTagInput.vue'

const messageType = defineModel('messageType', { default: 'reply' })
const to = defineModel('to', { default: '' })
const cc = defineModel('cc', { default: '' })
const bcc = defineModel('bcc', { default: '' })
const showBcc = defineModel('showBcc', { default: false })
const emailErrors = defineModel('emailErrors', { default: () => [] })
const htmlContent = defineModel('htmlContent', { default: '' })
const textContent = defineModel('textContent', { default: '' })
const mentions = defineModel('mentions', { default: () => [] })
const threadExpanded = defineModel('threadExpanded', { default: false })
const quotedThreadHtml = defineModel('quotedThreadHtml', { default: '' })
const macroStore = useMacroStore()
const usersStore = useUsersStore()
const teamStore = useTeamStore()

// Get suggestions for the mention dropdown
const getSuggestions = async (query) => {
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
  isGenerating: {
    type: Boolean,
    default: false
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
  ecommerceConfigured: {
    type: Boolean,
    default: false
  },
  inboxes: {
    type: Array,
    default: () => []
  },
  selectedInboxId: {
    type: [Number, null],
    default: null
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

const emit = defineEmits([
  'toggleFullscreen',
  'send',
  'sendWithStatus',
  'deleteDraft',
  'fileUpload',
  'inlineImageUpload',
  'fileDelete',
  'aiPromptSelected',
  'generateResponse',
  'generateWithOrders',
  'inboxChange',
  'filesDropped'
])

const conversationStore = useConversationStore()
const isEmailChannel = computed(() => {
  const ch = conversationStore.current?.inbox_channel
  return !ch || ch === 'email'
})
const emitter = useEmitter()
const { t } = useI18n()
const insertContent = ref(null)
const editorRef = ref(null)

const onInboxChange = (newId) => {
  emit('inboxChange', Number(newId))
}

// Local two-way binding for the FROM select — syncs prop down and emits changes up
const localSelectedInbox = computed({
  get: () => String(props.selectedInboxId || ''),
  set: (val) => emit('inboxChange', Number(val))
})

const toggleBcc = async () => {
  showBcc.value = !showBcc.value
  await nextTick()
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

const validateEmails = async () => {
  emailErrors.value = []
  await nextTick()

  const fields = ['to', 'cc', 'bcc']
  const values = { to: to.value, cc: cc.value, bcc: bcc.value }

  fields.forEach((field) => {
    const invalid = values[field]
      .split(',')
      .map((e) => e.trim())
      .filter((e) => e && !validateEmail(e))

    if (invalid.length)
      emailErrors.value.push(`${t('replyBox.invalidEmailsIn')} '${field}': ${invalid.join(', ')}`)
  })
}

const handleSend = async () => {
  await validateEmails()
  if (emailErrors.value.length > 0) {
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
      variant: 'destructive',
      description: t('globals.messages.correctEmailErrors')
    })
    return
  }
  emit('send')
}

const handleSendWithStatus = async (status) => {
  await validateEmails()
  if (emailErrors.value.length > 0) {
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
      variant: 'destructive',
      description: t('globals.messages.correctEmailErrors')
    })
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

const handleDroppedFiles = (files) => {
  emit('filesDropped', files)
}

const handleOnFileDelete = (uuid) => {
  emit('fileDelete', uuid)
}

const handleEmojiSelect = (emoji) => {
  insertContent.value = undefined
  nextTick(() => (insertContent.value = emoji))
}

const handleEditorCommand = (payload) => {
  if (typeof payload === 'string') {
    editorRef.value?.runCommand(payload)
  } else {
    editorRef.value?.runCommand(payload.command, payload.arg)
  }
}

const handleAiPromptSelected = (key) => {
  emit('aiPromptSelected', key)
}

const handleGenerateResponse = () => {
  emit('generateResponse')
}

const handleGenerateWithOrders = () => {
  emit('generateWithOrders')
}

watch(
  messageType,
  (newType, oldType) => {
    if (newType === 'reply') {
      macroStore.setCurrentView('replying')
    } else if (newType === 'private_note') {
      macroStore.setCurrentView('adding_private_note')
    }
    setTimeout(() => {
      editorRef.value?.focus()
    }, 50)
  },
  { immediate: true }
)

const focus = () => {
  editorRef.value?.focus()
}

function replacePlaceholders(html) {
  const contact = conversationStore.current?.contact || {}
  const agent = conversationStore.current?.assignee || {}
  const replacements = {
    '{{contact.first_name}}': contact.first_name || '',
    '{{contact.last_name}}': contact.last_name || '',
    '{{contact.full_name}}': [contact.first_name, contact.last_name].filter(Boolean).join(' ') || '',
    '{{contact.email}}': contact.email || '',
    '{{contact.phone}}': contact.phone_number || '',
  }
  let result = html
  for (const [placeholder, value] of Object.entries(replacements)) {
    result = result.replaceAll(placeholder, value)
  }
  return result
}

function insertMacro(html) {
  insertContent.value = undefined
  nextTick(() => {
    insertContent.value = replacePlaceholders(html)
  })
}

defineExpose({ focus, insertMacro })
</script>
