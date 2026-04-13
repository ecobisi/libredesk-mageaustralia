<template>
  <Dialog :open="openAIKeyPrompt" @update:open="openAIKeyPrompt = false">
    <DialogContent class="sm:max-w-lg">
      <DialogHeader class="space-y-2">
        <DialogTitle>{{ $t('ai.enterOpenAIAPIKey') }}</DialogTitle>
        <DialogDescription>
          {{
            $t('ai.apiKey.description', {
              provider: 'OpenAI'
            })
          }}
        </DialogDescription>
      </DialogHeader>
      <Form v-slot="{ handleSubmit }" as="" keep-values :validation-schema="formSchema">
        <form id="apiKeyForm" @submit="handleSubmit($event, updateProvider)">
          <FormField v-slot="{ componentField }" name="apiKey">
            <FormItem>
              <FormLabel>{{ $t('globals.terms.apiKey') }}</FormLabel>
              <FormControl>
                <Input type="text" placeholder="sk-am1RLw7XUWGX.." v-bind="componentField" />
              </FormControl>
              <FormMessage />
            </FormItem>
          </FormField>
        </form>
        <DialogFooter>
          <Button
            type="submit"
            form="apiKeyForm"
            :is-loading="isOpenAIKeyUpdating"
            :disabled="isOpenAIKeyUpdating"
          >
            {{ $t('globals.messages.save') }}
          </Button>
        </DialogFooter>
      </Form>
    </DialogContent>
  </Dialog>

  <!-- Collision confirmation dialog -->
  <AlertDialog :open="showCollisionConfirm" @update:open="showCollisionConfirm = false">
    <AlertDialogContent>
      <AlertDialogHeader>
        <AlertDialogTitle>Another agent has replied</AlertDialogTitle>
        <AlertDialogDescription>
          {{ collisionAgentName }} sent a reply while you were composing. Review their message before sending to avoid a duplicate response.
        </AlertDialogDescription>
      </AlertDialogHeader>
      <AlertDialogFooter>
        <AlertDialogCancel>Cancel</AlertDialogCancel>
        <AlertDialogAction @click="confirmSend">Send anyway</AlertDialogAction>
      </AlertDialogFooter>
    </AlertDialogContent>
  </AlertDialog>

  <!-- Discard draft confirmation -->
  <AlertDialog :open="showDiscardDraft" @update:open="showDiscardDraft = false">
    <AlertDialogContent>
      <AlertDialogHeader>
        <AlertDialogTitle>Discard draft</AlertDialogTitle>
        <AlertDialogDescription>
          Your draft will be discarded. Are you sure?
        </AlertDialogDescription>
      </AlertDialogHeader>
      <AlertDialogFooter>
        <AlertDialogCancel>No</AlertDialogCancel>
        <AlertDialogAction @click="confirmDeleteDraft">Yes</AlertDialogAction>
      </AlertDialogFooter>
    </AlertDialogContent>
  </AlertDialog>

  <div class="text-foreground bg-background">
    <!-- Collision warning banner -->
    <div
      v-if="collisionWarning"
      class="flex items-center gap-2 px-3 py-2 mx-2 mt-2 rounded-md text-sm border" style="background-color: #fefce8; border-color: #fde047; color: #854d0e;"
    >
      <AlertTriangle class="w-4 h-4 shrink-0" />
      <span class="flex-1">{{ collisionAgentName }} just sent a reply. Check before sending yours.</span>
      <button @click="dismissCollisionWarning" style="color: #a16207;" class="hover:opacity-70">
        <X class="w-3.5 h-3.5" />
      </button>
    </div>

    <!-- Customer reply warning banner -->
    <div
      v-if="customerReplyWarning"
      class="flex items-center gap-2 px-3 py-2 mx-2 mt-2 rounded-md text-sm border" style="background-color: #eff6ff; border-color: #93c5fd; color: #1e40af;"
    >
      <AlertTriangle class="w-4 h-4 shrink-0" />
      <span class="flex-1">The customer sent a new message while you were composing. Scroll down to review before sending.</span>
      <button @click="dismissCustomerReplyWarning" style="color: #1e40af;" class="hover:opacity-70">
        <X class="w-3.5 h-3.5" />
      </button>
    </div>

    <!-- Fullscreen editor -->
    <Dialog :open="isEditorFullscreen" @update:open="isEditorFullscreen = false">
      <DialogContent
        class="max-w-[92%] w-[92%] max-h-[90%] h-[88%] bg-card text-card-foreground p-4 flex flex-col"
        :class="{ '!bg-private': messageType === 'private_note' }"
        @escapeKeyDown="isEditorFullscreen = false"
        :hide-close-button="true"
      >
        <ReplyBoxContent
          v-if="isEditorFullscreen"
          :isFullscreen="true"
          :aiPrompts="aiPrompts"
          :isSending="isSending"
          :isDraftLoading="isDraftLoading"
          :isGenerating="isGenerating"
          :uploadingFiles="uploadingFiles"
          :uploadedFiles="mediaFiles"
          :ecommerceConfigured="ecommerceConfigured"
          :inboxes="inboxes"
          :selectedInboxId="selectedInboxId"
          v-model:htmlContent="htmlContent"
          v-model:textContent="textContent"
          v-model:to="to"
          v-model:cc="cc"
          v-model:bcc="bcc"
          v-model:emailErrors="emailErrors"
          v-model:messageType="messageType"
          v-model:showBcc="showBcc"
          v-model:mentions="mentions"
          v-model:quotedThreadHtml="quotedThreadHtml"
          v-model:threadExpanded="threadExpanded"
          @toggleFullscreen="isEditorFullscreen = !isEditorFullscreen"
          @send="processSend"
          @fileUpload="handleFileUpload"
          @fileDelete="handleFileDelete"
          @filesDropped="(files) => uploadFiles(files)"
          @aiPromptSelected="handleAiPromptSelected"
          @generateResponse="handleGenerateResponse"
          @generateWithOrders="handleGenerateWithOrders"
          @inboxChange="handleInboxChange"
          @sendWithStatus="processSendWithStatus"
          @deleteDraft="handleDeleteDraft"
          :hasDraft="hasDraftContent"
          :sendStatuses="availableSendStatuses"
          class="h-full flex-grow"
        />
      </DialogContent>
    </Dialog>

    <!-- Main Editor non-fullscreen -->
    <div
      class="bg-background text-card-foreground box m-2 px-2 pt-2 flex flex-col"
      :class="{ '!bg-private': messageType === 'private_note' }"
      v-if="!isEditorFullscreen"
    >
      <ReplyBoxContent
        ref="replyBoxContentRef"
        :isFullscreen="false"
        :aiPrompts="aiPrompts"
        :isSending="isSending"
        :isDraftLoading="isDraftLoading"
        :isGenerating="isGenerating"
        :uploadingFiles="uploadingFiles"
        :uploadedFiles="mediaFiles"
        :ecommerceConfigured="ecommerceConfigured"
        :inboxes="inboxes"
        :selectedInboxId="selectedInboxId"
        v-model:htmlContent="htmlContent"
        v-model:textContent="textContent"
        v-model:to="to"
        v-model:cc="cc"
        v-model:bcc="bcc"
        v-model:emailErrors="emailErrors"
        v-model:messageType="messageType"
        v-model:showBcc="showBcc"
        v-model:mentions="mentions"
        v-model:quotedThreadHtml="quotedThreadHtml"
        v-model:threadExpanded="threadExpanded"
        @toggleFullscreen="isEditorFullscreen = !isEditorFullscreen"
        @send="processSend"
        @fileUpload="handleFileUpload"
        @fileDelete="handleFileDelete"
        @filesDropped="(files) => uploadFiles(files)"
        @aiPromptSelected="handleAiPromptSelected"
        @generateResponse="handleGenerateResponse"
        @generateWithOrders="handleGenerateWithOrders"
        @inboxChange="handleInboxChange"
        @sendWithStatus="processSendWithStatus"
        @deleteDraft="handleDeleteDraft"
        :hasDraft="hasDraftContent"
        :sendStatuses="availableSendStatuses"
      />
    </div>
  </div>
</template>

<script setup>
import { ref, watch, computed, toRaw, onMounted, onBeforeUnmount } from 'vue'
import { useStorage } from '@vueuse/core'
import { handleHTTPError } from '@/utils/http'
import { EMITTER_EVENTS } from '@/constants/emitterEvents.js'
import { MACRO_CONTEXT } from '@/constants/conversation'
import { useUserStore } from '@/stores/user'
import { useDraftManager } from '@/composables/useDraftManager'
import api from '@/api'
import { useI18n } from 'vue-i18n'
import { useConversationStore } from '@/stores/conversation'
import { useTeamStore } from '@/stores/team'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { useEmitter } from '@/composables/useEmitter'
import { useFileUpload } from '@/composables/useFileUpload'
import ReplyBoxContent from '@/features/conversation/ReplyBoxContent.vue'
import { UserTypeAgent } from '@/constants/user'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle
} from '@/components/ui/alert-dialog'
import { AlertTriangle, X } from 'lucide-vue-next'
import {
  Form,
  FormField,
  FormItem,
  FormLabel,
  FormControl,
  FormMessage
} from '@/components/ui/form'
import { toTypedSchema } from '@vee-validate/zod'
import * as z from 'zod'

const formSchema = toTypedSchema(
  z.object({
    apiKey: z.string().min(1, 'API key is required')
  })
)

const { t } = useI18n()
const conversationStore = useConversationStore()
const teamStore = useTeamStore()
const emitter = useEmitter()
const userStore = useUserStore()

// Setup file upload composable
const {
  uploadingFiles,
  handleFileUpload,
  handleFileDelete,
  uploadFiles,
  mediaFiles,
  clearMediaFiles,
  setMediaFiles
} = useFileUpload({
  linkedModel: 'messages'
})

// Setup draft management composable
const currentDraftKey = computed(() => conversationStore.current?.uuid || null)
const {
  htmlContent,
  textContent,
  isLoading: isDraftLoading,
  clearDraft,
  loadedAttachments,
  loadedMacroActions
} = useDraftManager(currentDraftKey, mediaFiles)

// Rest of existing state
const openAIKeyPrompt = ref(false)
const isOpenAIKeyUpdating = ref(false)
const isEditorFullscreen = ref(false)
const isSending = ref(false)
const isGenerating = ref(false)
const messageType = useStorage('replyBoxMessageType', 'reply')
const to = ref('')
const cc = ref('')
const bcc = ref('')
const showBcc = ref(false)
const emailErrors = ref([])
const aiPrompts = ref([])
const replyBoxContentRef = ref(null)
const mentions = ref([])
const ecommerceConfigured = ref(false)
const threadExpanded = ref(false)

const quotedThreadHtml = ref('')
const _threadInitialized = ref(false)

// Build quoted thread from messages
const _buildThread = () => {
  if (messageType.value !== 'reply') return ''
  const msgs = conversationStore.conversationMessages
    ?.filter(m => !m.private && (m.type === 'incoming' || m.type === 'outgoing') && m.content)
    ?.sort((a, b) => new Date(b.created_at) - new Date(a.created_at))
    ?.slice(0, 3)
  if (!msgs || !msgs.length) return ''
  const conv = conversationStore.current
  let html = ''
  for (const m of msgs) {
    const name = ((m.author?.first_name || '') + ' ' + (m.author?.last_name || '')).trim() || 'Unknown'
    const email = m.type === 'incoming' ? (conv?.contact?.email || '') : ''
    const date = new Date(m.created_at).toLocaleString('en-US', {
      weekday: 'short', year: 'numeric', month: 'short', day: 'numeric',
      hour: 'numeric', minute: '2-digit'
    })
    const emailDisplay = email ? ' &lt;' + email + '&gt;' : ''
    let content = m.content || ''
    const quoteIdx = content.indexOf('<div class="gmail_quote">')
    if (quoteIdx > -1) {
      content = content.substring(0, quoteIdx)
    }
    html += '<div style="margin:8px 0;"><div style="color:#666;font-size:12px;margin-bottom:4px;">On ' + date + ', ' + name + emailDisplay + ' wrote:</div><blockquote style="margin:0 0 0 .8ex;border-left:1px solid #ccc;padding-left:1ex;">' + content + '</blockquote></div>'
  }
  return html
}

// Initialize thread when messages load (only once per conversation)
watch(() => conversationStore.conversationMessages, (msgs) => {
  if (!_threadInitialized.value && msgs?.length > 0 && (messageType.value === 'reply' || messageType.value === 'forward')) {
    quotedThreadHtml.value = _buildThread()
    _threadInitialized.value = true
  }
}, { immediate: true })

// Reset when switching conversations
watch(() => conversationStore.current?.uuid, () => {
  _threadInitialized.value = false
  quotedThreadHtml.value = ''
})

// Collision detection state
const composingStartedAt = ref(null)
const collisionWarning = ref(false)
const collisionAgentName = ref('')
const customerReplyWarning = ref(false)
const showCollisionConfirm = ref(false)
let pendingSendAction = null
let pendingStatusAfterSend = null

// Inbox switcher state
const inboxes = ref([])
const selectedInboxId = ref(null)

/**
 * Fetches AI prompts from the server.
 */
const fetchAiPrompts = async () => {
  try {
    const resp = await api.getAiPrompts()
    aiPrompts.value = resp.data.data
  } catch (error) {
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
      variant: 'destructive',
      description: handleHTTPError(error).message
    })
  }
}

/**
 * Fetches ecommerce configuration status.
 */
const fetchEcommerceStatus = async () => {
  try {
    const resp = await api.getEcommerceStatus()
    ecommerceConfigured.value = resp.data?.data?.configured || false
  } catch (error) {
    ecommerceConfigured.value = false
  }
}

/**
 * Fetches available inboxes for the From switcher.
 */
const fetchInboxes = async () => {
  try {
    const resp = await api.getInboxes()
    inboxes.value = (resp.data?.data || []).filter(i => i.enabled)
  } catch (error) {
    inboxes.value = []
  }
}

// Signature for current inbox
const inboxSignature = ref('')

// Fetch signature for a given inbox
const fetchInboxSignature = async (inboxId) => {
  if (!inboxId) return
  const conv = conversationStore.current
  try {
    const resp = await api.getInboxSignature(inboxId, conv?.uuid || '')
    inboxSignature.value = resp.data?.data?.signature || ''
  } catch (err) {
    inboxSignature.value = ''
  }
}

/**
 * Insert signature into editor content.
 * Replaces existing signature div if present, otherwise appends.
 */
const insertSignature = () => {
  if (!inboxSignature.value) return

  // Use a marker comment so we can find and replace the signature later.
  // TipTap strips div wrappers, so we use an HTML comment as a boundary.
  const sigMarker = '<!-- sig -->'
  const sigBlock = sigMarker + inboxSignature.value

  // If editor has existing signature, replace it
  if (htmlContent.value && htmlContent.value.includes(sigMarker)) {
    // Replace everything from marker to end
    htmlContent.value = htmlContent.value.substring(0, htmlContent.value.indexOf(sigMarker)) + sigBlock
    return
  }

  // If editor is empty or only whitespace/br tags, set signature as content
  const strippedContent = htmlContent.value
    ? htmlContent.value.replace(/<[^>]*>/g, '').trim()
    : ''

  if (!strippedContent) {
    htmlContent.value = '<p><br></p><p>' + sigBlock + '</p>'
  } else {
    // Append signature with a blank line separator
    htmlContent.value = htmlContent.value + '<p><br></p><p>' + sigBlock + '</p>'
  }
}

/**
 * Handle inbox change from the From switcher.
 */
const handleInboxChange = async (newInboxId) => {
  selectedInboxId.value = newInboxId
  await fetchInboxSignature(newInboxId)
  // Replace signature in editor
  const sigMarker = '<!-- sig -->'
  if (inboxSignature.value) {
    const sigBlock = sigMarker + inboxSignature.value
    if (htmlContent.value && htmlContent.value.includes(sigMarker)) {
      htmlContent.value = htmlContent.value.substring(0, htmlContent.value.indexOf(sigMarker)) + sigBlock
    } else {
      htmlContent.value = (htmlContent.value || '') + '<p><br></p><p>' + sigBlock + '</p>'
    }
  } else {
    // Remove existing signature if new inbox has none
    if (htmlContent.value && htmlContent.value.includes(sigMarker)) {
      htmlContent.value = htmlContent.value.substring(0, htmlContent.value.indexOf(sigMarker))
    }
  }
}

// Fetch data on mount
onMounted(() => {
  fetchAiPrompts()
  fetchEcommerceStatus()
  fetchInboxes()

  // Listen for new messages to detect other agent replies while composing
  emitter.on(EMITTER_EVENTS.NEW_MESSAGE, handleNewMessageCollision)
  emitter.on('set-reply-type', (type) => { messageType.value = type })
  emitter.on('populate-forward', handleForwardMessage)
  emitter.on('shortcut-discard-or-collapse', handleEscapeShortcut)
  emitter.on('restore-send', handleRestoreSend)
})

// Clean up listener
onBeforeUnmount(() => {
  emitter.off(EMITTER_EVENTS.NEW_MESSAGE, handleNewMessageCollision)
  emitter.off('shortcut-discard-or-collapse', handleEscapeShortcut)
  emitter.off('populate-forward', handleForwardMessage)
  emitter.off('restore-send', handleRestoreSend)
})

function handleForwardMessage(messageData) {
  // Switch to forward mode
  messageType.value = 'forward'
  // Clear TO field — agent enters the forward recipient
  to.value = ''
  cc.value = ''
  bcc.value = ''
  
  // Build the forwarded message header
  const author = messageData.author || {}
  const authorName = ((author.first_name || '') + ' ' + (author.last_name || '')).trim() || 'Unknown'
  const date = new Date(messageData.created_at).toLocaleString('en-US', {
    weekday: 'short', year: 'numeric', month: 'short', day: 'numeric',
    hour: 'numeric', minute: '2-digit'
  })
  
  // Strip any existing quoted thread from the message content (gmail_quote blocks)
  let msgContent = messageData.content || ''
  const quoteIdx = msgContent.indexOf('<div class="gmail_quote">')
  if (quoteIdx > -1) {
    msgContent = msgContent.substring(0, quoteIdx)
  }
  
  let fwdContent = '<p><br></p>'
  fwdContent += '<p style="color:#666;font-size:12px;margin:0 0 8px 0;">---------- Forwarded message ----------<br>'
  fwdContent += 'From: ' + authorName + '<br>'
  fwdContent += 'Date: ' + date + '</p>'
  fwdContent += msgContent
  
  // Insert signature if available
  const sigMarker = '<!-- sig -->'
  if (inboxSignature.value) {
    htmlContent.value = '<p><br></p><p>' + sigMarker + inboxSignature.value + '</p>' + fwdContent
  } else {
    htmlContent.value = fwdContent
  }
  
  // Build quoted thread from messages BEFORE the forwarded one (collapsed via ... toggle)
  const fwdTime = new Date(messageData.created_at).getTime()
  const conv = conversationStore.current
  const priorMsgs = conversationStore.conversationMessages
    ?.filter(m => !m.private && (m.type === 'incoming' || m.type === 'outgoing') && m.content)
    ?.filter(m => new Date(m.created_at).getTime() < fwdTime)
    ?.sort((a, b) => new Date(b.created_at) - new Date(a.created_at))
    ?.slice(0, 5)
  
  if (priorMsgs && priorMsgs.length > 0) {
    let threadHtml = ''
    for (const m of priorMsgs) {
      const name = ((m.author?.first_name || '') + ' ' + (m.author?.last_name || '')).trim() || 'Unknown'
      const email = m.type === 'incoming' ? (conv?.contact?.email || '') : ''
      const mDate = new Date(m.created_at).toLocaleString('en-US', {
        weekday: 'short', year: 'numeric', month: 'short', day: 'numeric',
        hour: 'numeric', minute: '2-digit'
      })
      // Strip gmail_quote from prior messages too — avoid nested thread duplication
      let priorContent = m.content || ''
      const priorQuoteIdx = priorContent.indexOf('<div class="gmail_quote">')
      if (priorQuoteIdx > -1) {
        priorContent = priorContent.substring(0, priorQuoteIdx)
      }
      const emailDisplay = email ? ' &lt;' + email + '&gt;' : ''
      threadHtml += '<div style="margin:8px 0;"><div style="color:#666;font-size:12px;margin-bottom:4px;">On ' + mDate + ', ' + name + emailDisplay + ' wrote:</div><blockquote style="margin:0 0 0 .8ex;border-left:1px solid #ccc;padding-left:1ex;">' + priorContent + '</blockquote></div>'
    }
    quotedThreadHtml.value = threadHtml
    threadExpanded.value = false
  } else {
    quotedThreadHtml.value = ''
  }
}

function handleRestoreSend(data) {
  if (data.htmlContent) htmlContent.value = data.htmlContent
  if (data.quotedThreadHtml !== undefined) quotedThreadHtml.value = data.quotedThreadHtml
  if (data.messageType) messageType.value = data.messageType
  if (data.to) to.value = data.to
  if (data.cc) cc.value = data.cc
  if (data.bcc) bcc.value = data.bcc
}

function handleEscapeShortcut() {
  // If fullscreen editor is open, close it first
  if (isEditorFullscreen.value) {
    isEditorFullscreen.value = false
    return
  }
  // If there's draft content, show discard confirmation
  if (hasDraftContent.value) {
    showDiscardDraft.value = true
  } else {
    // No content, just collapse
    emitter.emit('collapse-reply')
  }
}

/**
 * Handles collision detection when a new message arrives while composing.
 */
function handleNewMessageCollision({ conversation_uuid, message }) {
  if (!composingStartedAt.value) return
  if (conversation_uuid !== conversationStore.current?.uuid) return
  // Ignore private notes
  if (message?.private) return

  if (message?.type === 'incoming') {
    // Customer sent a new message while agent is composing
    customerReplyWarning.value = true
  } else if (message?.type === 'outgoing') {
    // Another agent replied while composing
    if (message?.sender_id === userStore.userID) return
    collisionWarning.value = true
    collisionAgentName.value = message?.sender?.first_name || 'Another agent'
  }
}

function dismissCollisionWarning() {
  collisionWarning.value = false
}

function dismissCustomerReplyWarning() {
  customerReplyWarning.value = false
      emitter.emit("collapse-reply")
}

function confirmSend() {
  showCollisionConfirm.value = false
  collisionWarning.value = false
  if (pendingSendAction === 'send') {
    doSend()
  } else if (pendingSendAction) {
    doSendWithStatus(pendingSendAction)
  }
  pendingSendAction = null
}

// When conversation changes, set selected inbox and fetch signature
watch(() => conversationStore.current?.uuid, async (newUuid) => {
  if (!newUuid) return

  // Ensure inboxes are loaded
  if (!inboxes.value.length) {
    await fetchInboxes()
  }

  const conv = conversationStore.current

  // Immediately set to conversation inbox (never leave as null)
  selectedInboxId.value = conv?.inbox_id || (inboxes.value.length ? inboxes.value[0].id : null)

  // Then try to resolve team default inbox override
  const teamId = conv?.assigned_team_id
  if (teamId) {
    try {
      const resp = await api.getTeamsCompact()
      const teams = resp.data?.data || []
      const team = teams.find(t => t.id === teamId)
      if (team?.default_inbox_id) {
        selectedInboxId.value = team.default_inbox_id
      }
    } catch (e) {
      // Keep conversation inbox as fallback
    }
  }

  // Only fetch signatures for email channels
  const inboxChannel = conv?.inbox_channel
  if (!inboxChannel || inboxChannel === 'email') {
    await fetchInboxSignature(selectedInboxId.value)
  } else {
    inboxSignature.value = ''
  }

  // Wait for draft to load, then insert signature if editor is empty
  setTimeout(() => {
    const strippedContent = htmlContent.value
      ? htmlContent.value.replace(/<[^>]*>/g, '').trim()
      : ''
    if (!strippedContent && inboxSignature.value && messageType.value !== 'private_note') {
      insertSignature()
    }
    // Insert quoted thread for replies (after signature)
    if (messageType.value === 'reply') {
    }
  }, 200)
}, { immediate: true })

// Toggle signature when switching between reply and private note
watch(messageType, (newType, oldType) => {
  const sigMarker = '<!-- sig -->'
  if (newType === 'forward') {
    // Forward mode — no thread, just the forwarded message content
    quotedThreadHtml.value = ''
  } else if (newType === 'private_note') {
    // Remove signature
    if (htmlContent.value && htmlContent.value.includes(sigMarker)) {
      htmlContent.value = htmlContent.value.substring(0, htmlContent.value.indexOf(sigMarker))
    }
    // Hide quoted thread for private notes
    quotedThreadHtml.value = ''
  } else if (oldType === 'private_note') {
    // Rebuild quoted thread when switching back to reply
    if (!quotedThreadHtml.value && conversationStore.conversationMessages?.length > 0) {
      quotedThreadHtml.value = _buildThread()
    }
    // Re-add signature when switching back to reply
    if (inboxSignature.value && (!htmlContent.value || !htmlContent.value.includes(sigMarker))) {
      const sigBlock = sigMarker + inboxSignature.value
      const strippedContent = htmlContent.value
        ? htmlContent.value.replace(/<[^>]*>/g, '').trim()
        : ''
      if (!strippedContent) {
        htmlContent.value = '<p><br></p><p>' + sigBlock + '</p>'
      } else {
        htmlContent.value = htmlContent.value + '<p><br></p><p>' + sigBlock + '</p>'
      }
    }
    // Re-insert quoted thread
  }
})

/**
 * Handles the AI prompt selection event.
 */
const handleAiPromptSelected = async (key) => {
  try {
    const resp = await api.aiCompletion({
      prompt_key: key,
      content: textContent.value
    })
    htmlContent.value = resp.data.data.replace(/\n/g, '<br>')
  } catch (error) {
    if (error.response?.status === 400 && userStore.can('ai:manage')) {
      emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
        variant: 'default',
        description: 'Please configure an AI provider in Settings > AI Settings'
      })
      return
    }
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
      variant: 'destructive',
      description: handleHTTPError(error).message
    })
  }
}

/**
 * Handles generating a response using RAG.
 */
const handleGenerateResponse = async (includeEcommerce = false) => {
  isGenerating.value = true
  try {
    const allMessages = conversationStore.conversationMessages
      .filter(m => m.content)
      .sort((a, b) => new Date(a.created_at) - new Date(b.created_at))

    // Public messages for conversation thread (last 7, oldest-first so AI sees chronology)
    const publicMessages = allMessages
      .filter(m => !m.private)
      .slice(-7)

    // Private/internal notes (last 5) — shown to AI as internal context only
    const privateNotes = allMessages
      .filter(m => m.private)
      .slice(-5)

    if (!publicMessages.length) {
      emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
        variant: "destructive",
        description: "No messages found in conversation"
      })
      return
    }

    const formatDate = (iso) => {
      try {
        return new Date(iso).toLocaleString('en-AU', { day: 'numeric', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit' })
      } catch { return '' }
    }

    const conversationText = publicMessages.map((m, idx) => {
      const doc = new DOMParser().parseFromString(m.content || "", "text/html")
      const text = doc.body.textContent || ""
      const role = m.type === "incoming" ? "Customer" : "Agent"
      const date = formatDate(m.created_at)
      const isLatest = idx === publicMessages.length - 1 ? " [MOST RECENT MESSAGE — respond to this]" : ""
      return `[${date}] ${role}${isLatest}: ${text.trim()}`
    }).join("\n\n")

    // Append private/internal notes as separate context block
    let internalNotes = ""
    if (privateNotes.length > 0) {
      internalNotes = "\n\n---\nINTERNAL AGENT NOTES (not visible to customer — use to inform your tone and response):\n"
      internalNotes += privateNotes.map(m => {
        const doc = new DOMParser().parseFromString(m.content || "", "text/html")
        const text = doc.body.textContent || ""
        return `[${formatDate(m.created_at)}] ${text.trim()}`
      }).join("\n\n")
    }

    if (!conversationText.trim()) {
      emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
        variant: "destructive",
        description: "Conversation content is empty"
      })
      return
    }

    // Extract agent instructions from editor (text typed before clicking Generate)
    let agentInstructions = ''
    if (htmlContent.value) {
      const sigMarker = '<!-- sig -->'
      let editorText = htmlContent.value
      // Strip signature
      if (editorText.includes(sigMarker)) {
        editorText = editorText.substring(0, editorText.indexOf(sigMarker))
      }
      // Strip HTML and get plain text
      const doc = new DOMParser().parseFromString(editorText, 'text/html')
      agentInstructions = (doc.body.textContent || '').trim()
    }

    const resp = await api.ragGenerate({
      conversation_id: conversationStore.current.id,
      inbox_id: selectedInboxId.value || conversationStore.current?.inbox_id || 0,
      customer_message: conversationText + internalNotes,
      include_ecommerce: includeEcommerce,
      agent_instructions: agentInstructions
    })

    if (resp.data?.data?.response) {
      const response = resp.data.data.response
      let generatedHtml
      if (/<[a-z][\s\S]*>/i.test(response)) {
        // HTML response: strip newlines between/around tags (source formatting),
        // convert remaining newlines (within text) to <br>, then clean up empties.
        generatedHtml = response
          .replace(/>\s*\n\s*/g, '>')   // strip newlines after closing >
          .replace(/\s*\n\s*</g, '<')   // strip newlines before opening <
          .replace(/\n/g, '<br>')         // remaining newlines are within text
        // Clean up empty elements that TipTap would render as blank bullets/lines
        generatedHtml = generatedHtml
          .replace(/<li>\s*(<br\s*\/?>\s*)*<\/li>/gi, '')
          .replace(/<p>\s*(<br\s*\/?>\s*)*<\/p>/gi, '')
      } else {
        // Plain text: convert double newlines to paragraphs, single to line breaks
        generatedHtml = '<p>' + response.replace(/\n{2,}/g, '</p><p>').replace(/\n/g, '<br>') + '</p>'
      }
      // Sanitize AI response: strip script tags and event handlers
      generatedHtml = generatedHtml.replace(/<script[\s\S]*?<\/script>/gi, '')
      generatedHtml = generatedHtml.replace(/\son\w+\s*=\s*("[^"]*"|'[^']*'|[^\s>]*)/gi, '')

      // Preserve signature if present
      const sigMarker = '<!-- sig -->'
      if (htmlContent.value && htmlContent.value.includes(sigMarker)) {
        const sigContent = htmlContent.value.substring(htmlContent.value.indexOf(sigMarker))
        htmlContent.value = generatedHtml + '<p><br></p><p>' + sigContent + '</p>'
      } else if (inboxSignature.value) {
        // Add signature after generated content
        htmlContent.value = generatedHtml + '<p><br></p><p>' + sigMarker + inboxSignature.value + '</p>'
      } else {
        htmlContent.value = generatedHtml
      }

      const successMsg = includeEcommerce
        ? "Response generated with order data"
        : "Response generated from knowledge base"
      emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
        description: successMsg
      })
    }
  } catch (error) {
    if (error.response?.status === 400 && userStore.can("ai:manage")) {
      emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
        variant: "default",
        description: "Please configure an AI provider and knowledge sources in Settings"
      })
      return
    }
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
      variant: "destructive",
      description: handleHTTPError(error).message
    })
  } finally {
    isGenerating.value = false
  }
}

const handleGenerateWithOrders = () => {
  handleGenerateResponse(true)
}





const updateProvider = async (values) => {
  try {
    isOpenAIKeyUpdating.value = true
    await api.updateAIProvider({ api_key: values.apiKey, provider: 'openai' })
    openAIKeyPrompt.value = false
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
      description: t('globals.messages.savedSuccessfully', {
        name: t('globals.terms.apiKey')
      })
    })
  } catch (error) {
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
      variant: 'destructive',
      description: handleHTTPError(error).message
    })
  } finally {
    isOpenAIKeyUpdating.value = false
  }
}

const hasTextContent = computed(() => {
  return textContent.value.trim().length > 0
})

// Track when agent starts composing
watch(textContent, (newVal) => {
  if (newVal && newVal.trim().length > 0 && !composingStartedAt.value) {
    composingStartedAt.value = new Date()
  }
})

/**
 * Checks for collision before sending. If another agent replied while composing, shows confirmation.
 */
const processSend = async () => {
  if (collisionWarning.value) {
    pendingSendAction = 'send'
    showCollisionConfirm.value = true
    return
  }
  await doSend()
}

/**
 * Actually sends the message.
 */
const doSend = async () => {
  isEditorFullscreen.value = false

  if (hasTextContent.value > 0 || mediaFiles.value.length > 0 || mentions.value.length > 0) {
    // Merge editor content with quoted thread for sending
    let sendHtml = htmlContent.value
    if (quotedThreadHtml.value && (messageType.value === 'reply' || messageType.value === 'forward')) {
      sendHtml += '<!-- thread --><div class="gmail_quote">' + quotedThreadHtml.value + '</div>'
    }
    const message = sendHtml
    const payload = {
      sender_type: UserTypeAgent,
      private: messageType.value === 'private_note',
      message: message,
      attachments: mediaFiles.value.map((file) => file.id),
      mentions: messageType.value === 'private_note' ? mentions.value : [],
      cc: cc.value
        .split(',')
        .map((email) => email.trim())
        .filter((email) => email),
      bcc: bcc.value
        ? bcc.value
            .split(',')
            .map((email) => email.trim())
            .filter((email) => email)
        : [],
      to: to.value
        ? to.value
            .split(',')
            .map((email) => email.trim())
            .filter((email) => email)
        : []
    }

    // Handle forward mode: set forwarded_to and clear regular to
    if (messageType.value === 'forward') {
      payload.forwarded_to = payload.to
      payload.to = []
      payload.private = false
    }

    // Include inbox_id if agent selected a different inbox
    if (selectedInboxId.value && selectedInboxId.value !== conversationStore.current?.inbox_id) {
      payload.inbox_id = selectedInboxId.value
    }

    // Collect macro info
    const macroID = conversationStore.getMacro(MACRO_CONTEXT.REPLY)?.id
    const macroActions = conversationStore.getMacro(MACRO_CONTEXT.REPLY)?.actions || []

    // Save restore data for undo (editor content WITHOUT thread, thread saved separately)
    const restoreData = {
      htmlContent: htmlContent.value,
      quotedThreadHtml: quotedThreadHtml.value,
      messageType: messageType.value,
      to: to.value,
      cc: cc.value,
      bcc: bcc.value
    }

    // Queue the send (Conversation.vue handles the delayed send + undo)
    emitter.emit('send-queued', {
      uuid: conversationStore.current.uuid,
      payload,
      macroID: macroID > 0 ? macroID : null,
      macroActions: macroID > 0 ? macroActions : [],
      statusAfterSend: pendingStatusAfterSend || null,
      restoreData,
      isPrivateNote: messageType.value === 'private_note',
      isForward: messageType.value === 'forward'
    })

    // Clear editor state immediately — must happen synchronously before collapse
    // so remounting the editor doesn't reload stale draft content
    const draftKey = currentDraftKey.value
    if (draftKey) {
      conversationStore.removeDraft(draftKey)
      // Also clear raw localStorage directly — useStorage reactive proxy may not
      // sync before component unmounts and a new instance reads from storage
      try {
        const rawDrafts = JSON.parse(localStorage.getItem('libredesk_drafts') || '{}')
        delete rawDrafts[draftKey]
        localStorage.setItem('libredesk_drafts', JSON.stringify(rawDrafts))
      } catch (e) { /* ignore */ }
    }
    clearDraft(draftKey)
    conversationStore.resetMacro(MACRO_CONTEXT.REPLY)
    clearMediaFiles()
    htmlContent.value = ''
    textContent.value = ''
    to.value = ''
    cc.value = ''
    bcc.value = ''
    emailErrors.value = []
    mentions.value = []
    composingStartedAt.value = null
    collisionWarning.value = false
    collisionAgentName.value = ''
    customerReplyWarning.value = false
    emitter.emit('collapse-reply')
  }
}

/**
 * Send message and set conversation status in one action.
 */
const processSendWithStatus = async (status) => {
  if (collisionWarning.value) {
    pendingSendAction = status
    showCollisionConfirm.value = true
    return
  }
  await doSendWithStatus(status)
}

const doSendWithStatus = async (status) => {
  pendingStatusAfterSend = status
  await doSend()
  pendingStatusAfterSend = null
}

const showDiscardDraft = ref(false)

/**
 * Show confirmation before deleting draft.
 */
const handleDeleteDraft = () => {
  showDiscardDraft.value = true
}

/**
 * Actually delete the draft after confirmation, then collapse the reply box.
 */
const confirmDeleteDraft = () => {
  clearDraft(currentDraftKey.value)
  clearMediaFiles()
  emailErrors.value = []
  mentions.value = []
  showDiscardDraft.value = false
  emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
    description: 'Draft discarded'
  })
  // Collapse the reply box (Fresh theme)
  emitter.emit('collapse-reply')
}

/**
 * Whether the editor has draft content.
 */
const hasDraftContent = computed(() => {
  return hasTextContent.value > 0 || mediaFiles.value.length > 0 || mentions.value.length > 0
})

/**
 * Statuses available for "Send and set as" dropdown.
 */
const availableSendStatuses = computed(() => {
  return conversationStore.statuses
    .filter(s => s.show_on_send)
    .map(s => s.name)
})

watch(
  () => conversationStore.getMacro('reply').id,
  async (newId) => {
    if (!newId) return
    const macro = conversationStore.getMacro('reply')
    const macroContent = macro.message_content
    if (macroContent) {
      // Insert at cursor position via ReplyBoxContent
      replyBoxContentRef.value?.insertMacro(macroContent)
    }

    // Clone macro attachments and add to reply
    if (macro.attachments && macro.attachments.length > 0) {
      try {
        const resp = await api.cloneMacroAttachments(macro.id)
        const clonedFiles = resp.data.data
        if (clonedFiles && clonedFiles.length > 0) {
          setMediaFiles([...mediaFiles.value, ...clonedFiles])
        }
      } catch (err) {
        console.error('Error cloning macro attachments:', err)
      }
    }
  },
  { deep: true }
)

watch(
  loadedMacroActions,
  (actions) => {
    if (actions.length > 0) {
      conversationStore.setMacroActions([...toRaw(actions)], MACRO_CONTEXT.REPLY)
    }
  },
  { deep: true }
)

watch(
  loadedAttachments,
  (attachments) => {
    if (attachments.length > 0) {
      setMediaFiles([...attachments])
    }
  },
  { deep: true }
)

watch(
  () => conversationStore.currentCC,
  (newVal) => {
    cc.value = newVal?.join(', ') || ''
  },
  { deep: true, immediate: true }
)

watch(
  () => conversationStore.currentTo,
  (newVal) => {
    to.value = newVal?.join(', ') || ''
  },
  { immediate: true }
)

watch(
  () => conversationStore.currentBCC,
  (newVal) => {
    const newBcc = newVal?.join(', ') || ''
    bcc.value = newBcc
    if (newBcc.length > 0) {
      showBcc.value = true
    }
  },
  { deep: true, immediate: true }
)

watch(
  () => conversationStore.current?.uuid,
  () => {
    clearMediaFiles()
    conversationStore.resetMacro(MACRO_CONTEXT.REPLY)
    // Reset collision state
    composingStartedAt.value = null
    collisionWarning.value = false
    collisionAgentName.value = ''
    customerReplyWarning.value = false
      emitter.emit("collapse-reply")
    setTimeout(() => {
      replyBoxContentRef.value?.focus()
    }, 100)
  }
)
</script>
