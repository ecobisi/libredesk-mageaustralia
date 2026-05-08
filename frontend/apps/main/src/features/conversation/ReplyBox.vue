<template>
  <AlertDialog :open="showContactEmailWarning" @update:open="showContactEmailWarning = $event">
    <AlertDialogContent>
      <AlertDialogHeader>
        <AlertDialogTitle>{{ $t('replyBox.contactEmailMissing') }}</AlertDialogTitle>
        <AlertDialogDescription>
          {{
            $t('replyBox.contactEmailMissingDescription', {
              email: conversationStore.current?.contact?.email
            })
          }}
        </AlertDialogDescription>
      </AlertDialogHeader>
      <AlertDialogFooter>
        <AlertDialogCancel>{{ $t('globals.messages.cancel') }}</AlertDialogCancel>
        <AlertDialogAction @click="processSend(true, true)">{{
          $t('replyBox.sendAnyway')
        }}</AlertDialogAction>
      </AlertDialogFooter>
    </AlertDialogContent>
  </AlertDialog>

  <AlertDialog :open="showMissingTagsWarning" @update:open="showMissingTagsWarning = $event">
    <AlertDialogContent>
      <AlertDialogHeader>
        <AlertDialogTitle>{{ $t('replyBox.missingTagsTitle') }}</AlertDialogTitle>
        <AlertDialogDescription>
          {{ $t('replyBox.missingTagsDescription') }}
        </AlertDialogDescription>
      </AlertDialogHeader>
      <AlertDialogFooter>
        <AlertDialogCancel>{{ $t('globals.messages.cancel') }}</AlertDialogCancel>
        <AlertDialogAction @click="processSend(false, true)">{{
          $t('replyBox.sendAnyway')
        }}</AlertDialogAction>
      </AlertDialogFooter>
    </AlertDialogContent>
  </AlertDialog>

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
        <AlertDialogTitle>{{ $t('replyBox.collision.title') }}</AlertDialogTitle>
        <AlertDialogDescription>
          {{ $t('replyBox.collision.description', { name: collisionAgentName }) }}
        </AlertDialogDescription>
      </AlertDialogHeader>
      <AlertDialogFooter>
        <AlertDialogCancel>{{ $t('globals.messages.cancel') }}</AlertDialogCancel>
        <AlertDialogAction @click="confirmSend">{{ $t('replyBox.sendAnyway') }}</AlertDialogAction>
      </AlertDialogFooter>
    </AlertDialogContent>
  </AlertDialog>

  <div class="text-foreground bg-background">
    <!-- Collision warning banner -->
    <div
      v-if="collisionWarning"
      class="flex items-center gap-2 px-3 py-2 mx-2 mt-2 rounded-md bg-amber-50 dark:bg-amber-950/40 text-amber-800 dark:text-amber-300 text-sm border border-amber-200 dark:border-amber-800"
    >
      <AlertTriangle class="w-4 h-4 shrink-0" />
      <span class="flex-1">{{ $t('replyBox.collision.banner', { name: collisionAgentName }) }}</span>
      <button @click="dismissCollisionWarning" class="text-amber-600 hover:text-amber-800 dark:hover:text-amber-200">
        <X class="w-3.5 h-3.5" />
      </button>
    </div>

    <!-- UX10: customer reply warning banner — informational (blue), not blocking. -->
    <div
      v-if="customerReplyWarning"
      class="flex items-center gap-2 px-3 py-2 mx-2 mt-2 rounded-md bg-blue-50 dark:bg-blue-950/40 text-blue-800 dark:text-blue-300 text-sm border border-blue-200 dark:border-blue-800"
    >
      <AlertTriangle class="w-4 h-4 shrink-0" />
      <span class="flex-1">{{ $t('replyBox.customerReply.banner') }}</span>
      <button @click="dismissCustomerReplyWarning" class="text-blue-600 hover:text-blue-800 dark:hover:text-blue-200">
        <X class="w-3.5 h-3.5" />
      </button>
    </div>

    <!--
      EC6: Fullscreen reply editor.
      Sized at 92% width / 88% height to match the Freshdesk-style "almost
      fullscreen" overlay agents are used to. The Radix Dialog renders via a
      portal at body-level with a backdrop, so the underlying conversation
      (including the EC4 sticky subject bar and EC3 undo-send banner) is
      visually obscured automatically — distraction-free composition without
      having to teleport siblings or unmount them. Per-session state only;
      not persisted (a fresh reload should always start in compact mode).
    -->
    <Dialog :open="isEditorFullscreen" @update:open="isEditorFullscreen = false">
      <DialogContent
        class="max-w-[92%] w-[92%] max-h-[90%] h-[88%] bg-card text-card-foreground p-4 flex flex-col"
        :class="{ '!bg-private': messageType === 'private_note' }"
        @escapeKeyDown="isEditorFullscreen = false"
        :hide-close-button="true"
      >
        <ReplyBoxContent
          v-if="isEditorFullscreen"
          ref="replyBoxContentRef"
          :isFullscreen="true"
          :aiPrompts="aiPrompts"
          :isSending="isSending"
          :isDraftLoading="isDraftLoading"
          :uploadingFiles="uploadingFiles"
          :uploadedFiles="mediaFiles"
          :hasDraft="hasDraftContent"
          :sendStatuses="availableSendStatuses"
          :fromOptions="fromOptions"
          v-model:htmlContent="htmlContent"
          v-model:textContent="textContent"
          v-model:to="to"
          v-model:cc="cc"
          v-model:bcc="bcc"
          v-model:emailErrors="emailErrors"
          v-model:messageType="messageType"
          v-model:showBcc="showBcc"
          v-model:mentions="mentions"
          v-model:selectedFrom="selectedFrom"
          @toggleFullscreen="isEditorFullscreen = !isEditorFullscreen"
          @send="processSend"
          @sendWithStatus="processSendWithStatus"
          @deleteDraft="handleDeleteDraft"
          @fileUpload="handleFileUpload"
          @fileDelete="handleFileDelete"
          @filesDropped="uploadFiles"
          @aiPromptSelected="handleAiPromptSelected"
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
        :uploadingFiles="uploadingFiles"
        :uploadedFiles="mediaFiles"
        :hasDraft="hasDraftContent"
        :sendStatuses="availableSendStatuses"
        :fromOptions="fromOptions"
        v-model:htmlContent="htmlContent"
        v-model:textContent="textContent"
        v-model:to="to"
        v-model:cc="cc"
        v-model:bcc="bcc"
        v-model:emailErrors="emailErrors"
        v-model:messageType="messageType"
        v-model:showBcc="showBcc"
        v-model:mentions="mentions"
        v-model:selectedFrom="selectedFrom"
        @toggleFullscreen="isEditorFullscreen = !isEditorFullscreen"
        @send="processSend"
        @sendWithStatus="processSendWithStatus"
        @deleteDraft="handleDeleteDraft"
        @fileUpload="handleFileUpload"
        @fileDelete="handleFileDelete"
        @filesDropped="uploadFiles"
        @aiPromptSelected="handleAiPromptSelected"
      />
    </div>
  </div>
</template>

<script setup>
import { ref, watch, computed, toRaw, onMounted, onBeforeUnmount } from 'vue'
import { useStorage } from '@vueuse/core'
import { parseEmailList } from '@shared-ui/utils/string'
import { useToast } from '@main/composables/useToast'
import { EMITTER_EVENTS } from '@main/constants/emitterEvents.js'
import { MACRO_CONTEXT } from '@main/constants/conversation'
import { useUserStore } from '@main/stores/user'
import { useDraftManager } from '@main/composables/useDraftManager'
import api from '@main/api'
import { useI18n } from 'vue-i18n'
import { useConversationStore } from '@main/stores/conversation'
import { useInboxStore } from '@main/stores/inbox'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle
} from '@shared-ui/components/ui/alert-dialog'
import { Button } from '@shared-ui/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle
} from '@shared-ui/components/ui/dialog'
import { Input } from '@shared-ui/components/ui/input'
import { useEmitter } from '@main/composables/useEmitter'
import { useFileUpload } from '@main/composables/useFileUpload'
import ReplyBoxContent from '@/features/conversation/ReplyBoxContent.vue'
import { UserTypeAgent } from '@/constants/user'
import {
  Form,
  FormField,
  FormItem,
  FormLabel,
  FormControl,
  FormMessage
} from '@shared-ui/components/ui/form'
import { toTypedSchema } from '@vee-validate/zod'
import * as z from 'zod'
import { AlertTriangle, X } from 'lucide-vue-next'

const formSchema = toTypedSchema(
  z.object({
    apiKey: z.string().min(1, 'API key is required')
  })
)

const { t } = useI18n()
const conversationStore = useConversationStore()
const inboxStore = useInboxStore()
const emitter = useEmitter()
const toast = useToast()
const userStore = useUserStore()

// Setup file upload composable
const {
  uploadingFiles,
  handleFileUpload,
  handleFileDelete,
  // UX9: programmatic upload entry — wraps a File[] in a synthetic event and
  // pushes it through handleFileUpload. Used by the editor's drop handler so
  // non-image files dragged into the editor land in the same upload pipeline
  // as the paperclip-button uploads.
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
const messageType = useStorage('replyBoxMessageType', 'reply')
const to = ref('')
const cc = ref('')
const bcc = ref('')
const showBcc = ref(false)
const emailErrors = ref([])
const aiPrompts = ref([])
const replyBoxContentRef = ref(null)
const showContactEmailWarning = ref(false)
const showMissingTagsWarning = ref(false)
const mentions = ref([])

// Collision detection state
const isComposing = ref(false)
const collisionWarning = ref(false)
const collisionAgentName = ref('')
// UX10: separate banner for customer-replied-while-composing (blue, not
// blocking — purely informational). Agent-vs-agent races stay amber.
const customerReplyWarning = ref(false)
const showCollisionConfirm = ref(false)
let pendingSendAction = null

// EC14: per-message From override. When the inbox has aliases configured,
// the reply box surfaces a From dropdown so the agent can send as
// "orders@" instead of "support@" without leaving the conversation.
// `selectedFrom` is one entry from `fromOptions`; empty = use the inbox
// primary (no override sent). Backend re-validates on POST so a stale
// dropdown can't spoof a foreign address.
const selectedFrom = ref('')
const fromOptions = computed(() => {
  const inbox = inboxStore.inboxes.find(
    (i) => i.id === conversationStore.current?.inbox_id
  )
  if (!inbox || inbox.channel !== 'email') return []
  const aliases = Array.isArray(inbox?.config?.aliases) ? inbox.config.aliases : []
  if (aliases.length === 0) return []
  // Primary first so it stays the default; aliases follow in admin order.
  // Dedupe via Set so an admin who lists the primary inside aliases too (a
  // common typo) doesn't surface the same address twice in the dropdown.
  const primary = inbox.from || ''
  const combined = primary ? [primary, ...aliases] : aliases
  return [...new Set(combined)]
})

/**
 * Fetches AI prompts from the server.
 */
const fetchAiPrompts = async () => {
  try {
    const resp = await api.getAiPrompts()
    aiPrompts.value = resp.data.data
  } catch (error) {
    toast.error(error)
  }
}

fetchAiPrompts()

// MP1: per-inbox signature.
// Resolved server-side (placeholders → agent + customer values) and cached
// per-conversation. Auto-inserted when the editor is empty so a fresh reply
// already has the agent's standard sign-off; we don't overwrite an in-progress
// draft. Refetched on conversation switch because both inbox and customer can
// change.
const inboxSignature = ref('')

const fetchInboxSignature = async () => {
  const conv = conversationStore.current
  if (!conv?.inbox_id) {
    inboxSignature.value = ''
    return
  }
  try {
    const resp = await api.getInboxSignature(conv.inbox_id, conv.uuid)
    inboxSignature.value = resp.data?.data?.signature || ''
  } catch {
    inboxSignature.value = ''
  }
}

// EC19: marker so the messageType watcher can find and re-insert (or strip)
// the signature when the agent toggles between reply and private note. Lives
// in the rendered HTML as a comment, invisible to the editor + the customer.
const SIGNATURE_MARKER = '<!-- sig -->'

const insertSignatureIfEmpty = () => {
  // Only inject for fresh replies — never clobber a draft, an in-flight
  // forward body, or a private note (private notes don't go to the customer
  // so they never get a signature; the messageType watcher below also
  // strips it on switch-to-private and re-adds on switch-back).
  if (!inboxSignature.value) return
  if (messageType.value !== 'reply') return
  const stripped = (htmlContent.value || '').replace(/<[^>]+>/g, '').trim()
  if (stripped) return
  htmlContent.value = '<p><br></p><p><br></p>' + SIGNATURE_MARKER + inboxSignature.value
}

/**
 * Handles the AI prompt selection event.
 * Sends the selected prompt key and the current text content to the server for completion.
 * Sets the response as the new content in the editor.
 * @param {String} key - The key of the selected AI prompt
 */
const handleAiPromptSelected = async (key) => {
  try {
    const resp = await api.aiCompletion({
      prompt_key: key,
      content: textContent.value
    })
    htmlContent.value = resp.data.data.replace(/\n/g, '<br>')
  } catch (error) {
    // Check if user needs to enter OpenAI API key and has permission to do so.
    if (error.response?.status === 400 && userStore.can('ai:manage')) {
      openAIKeyPrompt.value = true
    }
    toast.error(error)
  }
}

/**
 * updateProvider updates the OpenAI API key.
 * @param {Object} values - The form values containing the API key
 */
const updateProvider = async (values) => {
  try {
    isOpenAIKeyUpdating.value = true
    await api.updateAIProvider({ api_key: values.apiKey, provider: 'openai' })
    openAIKeyPrompt.value = false
    toast.success(t('globals.messages.savedSuccessfully'))
  } catch (error) {
    toast.error(error)
  } finally {
    isOpenAIKeyUpdating.value = false
  }
}

/**
 * Returns true if the editor has text content.
 */
const hasTextContent = computed(() => {
  return textContent.value.trim().length > 0
})

/**
 * EC1: drives the delete-draft button visibility. We treat any text or
 * attached file as "draft worth discarding". An empty editor with no
 * attachments doesn't surface the button.
 */
const hasDraftContent = computed(() => {
  return hasTextContent.value || mediaFiles.value.length > 0 || mentions.value.length > 0
})

/**
 * EC1: status names exposed in the "Send & set as" dropdown. Filter by
 * category, not name — admins can rename "Snoozed" or add custom statuses
 * in those categories, and a name-based filter would silently fail open
 * (lets agents pick a status that has a dedicated UI flow). Categories
 * 'waiting' (snooze picker), 'spam' and 'trashed' (header menu actions)
 * are surfaced via dedicated UI; only 'open' and 'resolved' belong here.
 */
const availableSendStatuses = computed(() => {
  return conversationStore.statuses
    .filter((s) => s.category === 'open' || s.category === 'resolved')
    .map((s) => s.name)
})

// Track the in-flight set-status so collision-confirm dialog can resume it
// with the right action variant. Without this, clicking the Send chevron
// while another agent is composing would lose the status choice once the
// agent confirms the collision dialog.
// intentionally non-reactive (matches pendingSendAction); only read in confirmSend
let pendingSetStatus = ''

/**
 * Processes the send action.
 * If another agent replied while composing, show a confirmation dialog first.
 *
 * @param {boolean} skipContactEmailCheck - bypass the contact-not-in-recipients warning
 * @param {boolean} skipMissingTagsCheck - bypass the prompt-tags-on-reply warning
 * @param {string}  setStatus - if non-empty, transition the conversation to
 *                              this status name in the same backend request.
 *                              Powers the EC1 "Send & Resolve" / "Send & Close"
 *                              dropdown — single-action send-and-transition.
 */
const processSend = async (skipContactEmailCheck = false, skipMissingTagsCheck = false, setStatus = '') => {
  if (!skipContactEmailCheck && collisionWarning.value) {
    pendingSendAction = 'send'
    pendingSetStatus = setStatus
    showCollisionConfirm.value = true
    return
  }
  let hasMessageSendingErrored = false
  isEditorFullscreen.value = false

  // FS24: mention-only private notes (e.g. "@john") still send. TipTap mention
  // nodes don't always count toward textContent, so without this gate a
  // private note that is just an @mention would silently no-op.
  const hasContent = hasTextContent.value > 0 || mediaFiles.value.length > 0 || mentions.value.length > 0
  const convUUID = conversationStore.current.uuid
  const isPrivate = messageType.value === 'private_note'
  const isForward = messageType.value === 'forward'

  const currentInbox = inboxStore.inboxes.find(
    (i) => i.id === conversationStore.current.inbox_id
  )
  if (
    !isPrivate &&
    !skipMissingTagsCheck &&
    currentInbox?.prompt_tags_on_reply &&
    !(conversationStore.current.tags?.length > 0)
  ) {
    showMissingTagsWarning.value = true
    return
  }

  if (!isPrivate && conversationStore.current.inbox_channel === 'email') {
    // Require at least one recipient in `to`.
    if (!to.value.trim()) {
      toast.error(t('replyBox.toRequired'))
      return
    }

    // Warn if the contact's email is not in any recipient field. Skip for
    // forward mode — by design the customer is NOT a recipient on a forward.
    if (!skipContactEmailCheck && !isForward) {
      const contactEmail = conversationStore.current.contact?.email?.toLowerCase()
      if (contactEmail) {
        const allRecipients = [to.value, cc.value, bcc.value].join(',').toLowerCase()
        if (!parseEmailList(allRecipients).includes(contactEmail)) {
          showContactEmailWarning.value = true
          return
        }
      }
    }
  }
  // Nothing to send.
  if (!hasContent) {
    isSending.value = false
    return
  }

  const savedContent = htmlContent.value
  const savedTo = to.value
  const savedCC = cc.value
  const savedBCC = bcc.value
  const savedMessageType = messageType.value
  const savedMediaFiles = [...mediaFiles.value]
  const savedMentions = [...mentions.value]
  const macroSnapshot = conversationStore.getMacro(MACRO_CONTEXT.REPLY)
  const savedMacroID = macroSnapshot?.id || 0
  const savedMacroActions = macroSnapshot?.actions ? [...macroSnapshot.actions] : []

  const author = {
    id: userStore.userID,
    first_name: userStore.firstName,
    last_name: userStore.lastName,
    avatar_url: userStore.avatar,
    type: 'agent'
  }
  const parsedTo = !isPrivate && savedTo ? parseEmailList(savedTo) : []
  const parsedCC = !isPrivate && savedCC ? parseEmailList(savedCC) : []
  const parsedBCC = !isPrivate && savedBCC ? parseEmailList(savedBCC) : []
  const meta = {}
  if (parsedTo.length) meta.to = parsedTo
  if (parsedCC.length) meta.cc = parsedCC
  if (parsedBCC.length) meta.bcc = parsedBCC

  // Optimistically render the message in the thread so the agent gets
  // instant feedback. If they hit Undo, Conversation.vue will remove this
  // pending entry; otherwise the API response (or WS echo) replaces it.
  const tempUUID = conversationStore.addPendingMessage(
    convUUID,
    savedContent,
    isPrivate,
    author,
    savedMediaFiles,
    textContent.value,
    meta
  )

  const payload = {
    sender_type: UserTypeAgent,
    private: isPrivate,
    message: savedContent,
    attachments: savedMediaFiles.map((file) => file.id),
    mentions: isPrivate ? savedMentions : [],
    cc: parsedCC,
    bcc: parsedBCC,
    to: parsedTo,
    echo_id: isPrivate ? '' : tempUUID
  }
  // EC14: include the per-message From override only when the agent
  // explicitly picked an alias (not the empty default). Backend
  // re-validates against inbox.from + config.aliases.
  if (!isPrivate && selectedFrom.value) {
    payload.from = selectedFrom.value
  }
  // Forward mode routes to the typed addresses via `forwarded_to`; the
  // backend overrides the conversation's normal recipients with these
  // and tags meta.forwarded_to. CC/BCC pass through unchanged.
  if (isForward) {
    payload.forwarded_to = parsedTo
    payload.to = []
  }
  // EC1: tell the backend to transition status post-send in the same
  // request so the agent gets atomic Send-and-Resolve. Backend validates
  // the status name; we just pass it through. Empty string = no transition,
  // which is the default and matches a plain Send.
  if (setStatus && !isPrivate && !isForward) {
    payload.set_status = setStatus
  }

  // EC3: Save everything needed to restore the editor on Undo. Recipients +
  // message type + content + attachments + mentions + macro selection — the
  // full state the agent had at the moment they clicked Send.
  const restoreData = {
    htmlContent: savedContent,
    messageType: savedMessageType,
    to: savedTo,
    cc: savedCC,
    bcc: savedBCC,
    mediaFiles: savedMediaFiles,
    mentions: savedMentions,
    macroID: savedMacroID,
    macroActions: savedMacroActions,
    setStatus
  }

  // EC3: Briefly mark the editor as sending. Drives ReplyBoxContent's
  // :isSending prop (lines 119, 156) which disables the Send button so a
  // double-click during the synchronous setup below can't double-fire
  // SEND_QUEUED. Cleared at the end of processSend (a few lines below)
  // since the editor is itself cleared up-front and a new compose during
  // the banner window is a legitimate flow — Conversation.vue's
  // handleSendQueued explicitly handles a second queued send by flushing
  // the previous one first.
  isSending.value = true

  // EC3: Hand the queued send off to Conversation.vue, which owns the 5s
  // countdown + Undo banner. We deliberately clear local editor / draft
  // state up-front so the editor visually empties — Undo restores it.
  emitter.emit(EMITTER_EVENTS.SEND_QUEUED, {
    uuid: convUUID,
    tempUUID,
    payload,
    isPrivate,
    isForward,
    setStatus,
    macroID: savedMacroID,
    macroActions: savedMacroActions,
    draftKey: currentDraftKey.value,
    restoreData
  })

  // Clear editor / draft / recipients up-front. Conversation.vue drives the
  // delayed POST; we don't await it here. Any failure path is handled there.
  clearDraft(currentDraftKey.value)
  conversationStore.resetMacro(MACRO_CONTEXT.REPLY)
  clearMediaFiles()
  htmlContent.value = ''
  textContent.value = ''
  // Recipients are restored from conversation defaults via watchers; only
  // forward mode needs an explicit reset because forward defaults to empty.
  if (isForward) {
    to.value = ''
    cc.value = ''
    bcc.value = ''
    showBcc.value = false
    messageType.value = 'reply'
  }
  emailErrors.value = []
  mentions.value = []
  // Reset collision state.
  isComposing.value = false
  collisionWarning.value = false
  collisionAgentName.value = ''
  customerReplyWarning.value = false
  isSending.value = false
}

/**
 * EC1: Send & Set Status. Thin wrapper that forwards the chosen status name
 * to processSend, which packs it into the POST as `set_status`. Backend
 * does the actual transition after the reply is queued — single atomic
 * action, no second round-trip from the client.
 */
const processSendWithStatus = (status) => {
  return processSend(false, false, status)
}

/**
 * EC1: Discard the current draft. Clears persisted draft state, attached
 * files, and any pending validation errors. The toast is the agent's
 * confirmation that the action took effect.
 */
const handleDeleteDraft = () => {
  clearDraft(currentDraftKey.value)
  clearMediaFiles()
  emailErrors.value = []
  mentions.value = []
  htmlContent.value = ''
  toast.success(t('replyBox.draftDeleted'))
}

/**
 * Watches for changes in the conversation's macro id and inject macro body
 * into the editor at the cursor.
 *
 * FS8: Previous behaviour replaced the entire htmlContent with the macro
 * body, wiping any draft the agent had typed and ignoring cursor position.
 * Now we hand off to ReplyBoxContent.insertMacro which uses TipTap's
 * insertContent (placement at cursor, with placeholder substitution against
 * the loaded conversation's contact). This matches v1.0.3 behaviour and
 * means an agent can pad a macro between typed sentences instead of choosing
 * "macro xor draft".
 *
 * MP5: If the macro has attachments, ask the backend to clone them (each
 * gets a fresh UUID + DB row, model_type='messages' / model_id=NULL) and
 * append them to the reply's attachment list. Cloning rather than referencing
 * means the agent can later delete the macro (or its files) without breaking
 * the already-sent message.
 */
watch(
  () => conversationStore.getMacro('reply').id,
  async (newId) => {
    if (!newId) return
    const macro = conversationStore.getMacro('reply')
    const macroContent = macro.message_content
    if (macroContent) {
      replyBoxContentRef.value?.insertMacro(macroContent)
    }

    if (macro.attachments && macro.attachments.length > 0) {
      try {
        const resp = await api.cloneMacroAttachments(macro.id)
        const clonedFiles = resp.data.data
        if (clonedFiles && clonedFiles.length > 0) {
          // mediaFiles is exposed as readonly from useFileUpload, so we
          // re-set the full list rather than push() to it.
          setMediaFiles([...mediaFiles.value, ...clonedFiles])
        }
      } catch (err) {
        console.error('Error cloning macro attachments:', err)
      }
    }
  },
  { deep: true }
)

/**
 * Watch loaded macro actions from draft and update conversation store.
 */
watch(
  loadedMacroActions,
  (actions) => {
    if (actions.length > 0) {
      conversationStore.setMacroActions([...toRaw(actions)], MACRO_CONTEXT.REPLY)
    }
  },
  { deep: true }
)

/**
 * Watch for loaded attachments from draft and restore them to mediaFiles.
 */
watch(
  loadedAttachments,
  (attachments) => {
    if (attachments.length > 0) {
      setMediaFiles([...attachments])
    }
  },
  { deep: true }
)

// Initialize to, cc, and bcc fields with the current conversation's values.
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
    // Only show BCC field if it has content
    if (newBcc.length > 0) {
      showBcc.value = true
    }
  },
  { deep: true, immediate: true }
)

// Clear media files, recipients, and any in-progress forward state when
// switching conversations. Without the forward reset, an agent who clicked
// Forward on conv A and then switched to conv B would land in B with
// messageType=forward, empty recipient fields, and A's message pre-quoted
// in the editor — easy to accidentally send.
watch(
  () => conversationStore.current?.uuid,
  () => {
    clearMediaFiles()
    conversationStore.resetMacro(MACRO_CONTEXT.REPLY)
    // UX10: clear collision/customer-reply banners on conv switch — the
    // banners refer to events on the previous conversation, carrying them
    // over would mislead the agent on the new conversation.
    collisionWarning.value = false
    collisionAgentName.value = ''
    customerReplyWarning.value = false
    // EC14: reset From override on conversation switch. The new
    // conversation may belong to a different inbox with a different
    // alias set; carrying the previous selection over would either
    // pick a now-invalid alias or silently spoof the agent's intent.
    selectedFrom.value = ''
    if (messageType.value === 'forward') {
      messageType.value = 'reply'
      to.value = ''
      cc.value = ''
      bcc.value = ''
      showBcc.value = false
      htmlContent.value = ''
    }
    // Focus editor on conversation change
    setTimeout(() => {
      replyBoxContentRef.value?.focus()
    }, 100)
    // MP1: refetch signature for the new conversation's inbox + contact, then
    // inject if the editor's still empty after the conversation switch settled.
    fetchInboxSignature().then(() => {
      setTimeout(insertSignatureIfEmpty, 100)
    })
  }
)

// Reset the forward-pre-quoted body if the agent switches away from forward
// mode. Otherwise a stale "---- Forwarded message ----" header would carry
// into a customer-facing reply.
//
// EC19: also toggle the inbox signature when switching between reply and
// private note. Private notes are agent-internal and don't go to the
// customer, so a signature there is noise. We stash and restore using the
// SIGNATURE_MARKER comment that insertSignatureIfEmpty injected on initial
// reply mount; if the marker is absent (e.g. the agent deleted it manually,
// or never had a signature configured) the toggle is a safe no-op.
watch(messageType, (newType, oldType) => {
  if (oldType === 'forward' && newType !== 'forward') {
    htmlContent.value = ''
    to.value = ''
    cc.value = ''
    bcc.value = ''
    showBcc.value = false
    return
  }

  if (newType === 'private_note') {
    // Strip from the marker onward — preserves anything the agent typed
    // ABOVE the signature (the actual reply content) and discards the
    // signature block + anything after it.
    if (htmlContent.value && htmlContent.value.includes(SIGNATURE_MARKER)) {
      htmlContent.value = htmlContent.value.substring(0, htmlContent.value.indexOf(SIGNATURE_MARKER))
    }
    return
  }

  if (oldType === 'private_note' && newType === 'reply' && inboxSignature.value) {
    if (!htmlContent.value || !htmlContent.value.includes(SIGNATURE_MARKER)) {
      const sigBlock = SIGNATURE_MARKER + inboxSignature.value
      const stripped = (htmlContent.value || '').replace(/<[^>]+>/g, '').trim()
      if (!stripped) {
        // Editor is empty — match the fresh-reply layout from
        // insertSignatureIfEmpty so the agent gets the same vertical spacing.
        htmlContent.value = '<p><br></p><p><br></p>' + sigBlock
      } else {
        // Agent typed a private note then switched back. Append the
        // signature below their text rather than overwriting it.
        htmlContent.value = htmlContent.value + '<p><br></p>' + sigBlock
      }
    }
  }
})

// Forward mode: triggered from a per-message Forward button on MessageBubble.
// Switches the reply box into forward mode, clears the recipient fields so
// the agent enters the external recipient, and pre-populates the editor with
// a quoted copy of the original message.
function handleForwardMessage (message) {
  messageType.value = 'forward'
  to.value = ''
  cc.value = ''
  bcc.value = ''
  showBcc.value = false

  const author = message.author || {}
  const authorName = ((author.first_name || '') + ' ' + (author.last_name || '')).trim() || 'Unknown'
  const date = new Date(message.created_at).toLocaleString('en-US', {
    weekday: 'short',
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit'
  })

  // Strip any nested gmail_quote so we don't double-quote a thread.
  let body = message.content || ''
  const quoteIdx = body.indexOf('<div class="gmail_quote">')
  if (quoteIdx > -1) body = body.substring(0, quoteIdx)

  const header =
    '<p><br></p>' +
    '<p style="color:#666;font-size:12px;margin:0 0 8px 0;">' +
    '---------- Forwarded message ----------<br>' +
    'From: ' + authorName + '<br>' +
    'Date: ' + date +
    '</p>'
  htmlContent.value = header + body
}

// Wrap the emitter subscription in lifecycle hooks so each ReplyBox mount
// registers exactly one handler. Without the matching .off, switching
// conversations N times stacks N listeners and a single Forward click
// fires the handler N times.
onMounted(() => {
  emitter.on(EMITTER_EVENTS.FORWARD_MESSAGE, handleForwardMessage)
  emitter.on(EMITTER_EVENTS.NEW_MESSAGE, handleNewMessageCollision)
  emitter.on(EMITTER_EVENTS.RESTORE_SEND, handleRestoreSend)
  emitter.on(EMITTER_EVENTS.SHORTCUT_REPLY, handleShortcutReply)
  emitter.on(EMITTER_EVENTS.SHORTCUT_NOTE, handleShortcutNote)
  // EC14: ensure inboxes are loaded so fromOptions can be computed for
  // the From switcher. fetchInboxes is a no-op if already populated.
  inboxStore.fetchInboxes()
  // MP1: fetch the inbox signature for the conversation that's already loaded
  // when ReplyBox mounts (deep-link, refresh). The conversation-uuid watcher
  // handles every subsequent switch.
  fetchInboxSignature().then(() => insertSignatureIfEmpty())
})
onBeforeUnmount(() => {
  emitter.off(EMITTER_EVENTS.FORWARD_MESSAGE, handleForwardMessage)
  emitter.off(EMITTER_EVENTS.NEW_MESSAGE, handleNewMessageCollision)
  emitter.off(EMITTER_EVENTS.RESTORE_SEND, handleRestoreSend)
  emitter.off(EMITTER_EVENTS.SHORTCUT_REPLY, handleShortcutReply)
  emitter.off(EMITTER_EVENTS.SHORTCUT_NOTE, handleShortcutNote)
})

// UX10: global R / N shortcuts. App.vue's keydown handler emits these
// (only when no input/editor is focused); we flip the messageType tab and
// focus the editor so the agent can start typing immediately.
function handleShortcutReply() {
  if (!conversationStore.current?.uuid) return
  messageType.value = 'reply'
  // nextTick not needed — the Tab change is reactive but the focus call
  // doesn't depend on the editor being remounted (ReplyBoxContent is always
  // present, only the toolbar/recipient fields change with messageType).
  setTimeout(() => replyBoxContentRef.value?.focus(), 0)
}
function handleShortcutNote() {
  if (!conversationStore.current?.uuid) return
  messageType.value = 'private_note'
  setTimeout(() => replyBoxContentRef.value?.focus(), 0)
}

/**
 * EC3: Restore the editor state after the agent clicks Undo on the queued
 * send banner. Re-hydrates content + recipients + message type + attachments
 * + mentions + macro selection — exactly what processSend captured.
 *
 * The set_status from a Send & Resolve isn't restored to a UI control here
 * because v2's set-status dropdown is a transient action menu (no sticky
 * "next send will be Send & Resolve" mode). If the agent re-clicks Send &
 * Resolve after Undo, they reselect from the chevron — which is consistent
 * with how the dropdown works for any non-undo flow.
 */
function handleRestoreSend (data) {
  if (data == null) return
  if (data.htmlContent != null) htmlContent.value = data.htmlContent
  if (data.messageType != null) messageType.value = data.messageType
  if (data.to != null) to.value = data.to
  if (data.cc != null) cc.value = data.cc
  if (data.bcc != null) {
    bcc.value = data.bcc
    if (data.bcc) showBcc.value = true
  }
  // Skip the empty-array case: setMediaFiles([]) would clear any files the
  // agent uploaded after the original send was queued (rare but possible
  // during the 5s window). Restore is additive — we only repopulate when
  // the queued send had attachments.
  if (Array.isArray(data.mediaFiles) && data.mediaFiles.length > 0) {
    setMediaFiles(data.mediaFiles)
  }
  if (Array.isArray(data.mentions)) {
    mentions.value = data.mentions
  }
  if (data.macroID && data.macroActions?.length > 0) {
    conversationStore.setMacroActions([...data.macroActions], MACRO_CONTEXT.REPLY)
  }
}

/**
 * Handles collision detection when a new message arrives while composing.
 * Two cases:
 *  - outgoing (agent) reply from another agent → collisionWarning (amber)
 *  - incoming customer reply → customerReplyWarning (blue, UX10)
 * Private notes are ignored either way; agents posting notes shouldn't
 * derail an in-progress reply.
 */
function handleNewMessageCollision({ conversation_uuid, message }) {
  if (!isComposing.value) return
  if (conversation_uuid !== conversationStore.current?.uuid) return
  if (message?.private) return

  if (message?.type === 'incoming') {
    // UX10: the customer replied while the agent was composing. Surface a
    // banner so the agent can scroll down and review before sending into
    // a thread that's just shifted. Distinct from collisionWarning (amber)
    // because this isn't an agent-vs-agent race, it's just useful context.
    customerReplyWarning.value = true
    return
  }
  if (message?.type === 'outgoing') {
    // Ignore messages from the current user. Fall back to author.id because
    // the WS-broadcast payload may not carry a top-level sender_id.
    const senderId = message?.sender_id ?? message?.author?.id
    if (senderId === userStore.userID) return
    collisionWarning.value = true
    collisionAgentName.value = message?.author?.first_name || message?.sender?.first_name || t('replyBox.collision.anotherAgent')
  }
}

function dismissCollisionWarning() {
  collisionWarning.value = false
}

function dismissCustomerReplyWarning() {
  customerReplyWarning.value = false
}

function confirmSend() {
  showCollisionConfirm.value = false
  collisionWarning.value = false
  if (pendingSendAction === 'send') {
    // agent confirmed past one guard; don't bounce them through more
    // skipContactEmailCheck=true AND skipMissingTagsCheck=true: agent
    // already passed the collision dialog, don't make them re-confirm
    // the contact-email and missing-tags warnings on the way through.
    // Pass-through any pending set_status so a chevron pick before the
    // dialog still resolves the conversation post-send.
    processSend(true, true, pendingSetStatus)
  }
  pendingSendAction = null
  pendingSetStatus = ''
}

// Track when the agent starts composing (used to detect collision window).
watch(textContent, (newVal) => {
  if (newVal && newVal.trim().length > 0 && !isComposing.value) {
    isComposing.value = true
  } else if (!newVal || newVal.trim().length === 0) {
    isComposing.value = false
  }
})
</script>
