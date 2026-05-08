<template>
  <div class="flex flex-col text-left" :class="isOutgoing ? 'items-end' : 'items-start'">
    <!-- UX10: Sender name + timestamp on the same line. Tighter than a
      separate timestamp row beneath the bubble, and means the relative
      time is visible alongside the author when scanning the thread. -->
    <div
      class="mb-1 flex items-baseline gap-2"
      :class="isOutgoing ? 'pr-[47px] justify-end' : 'pl-[47px]'"
    >
      <router-link
        v-if="!isOutgoing"
        :to="{ name: 'contact-detail', params: { id: message.author?.id } }"
        class="cursor-pointer text-muted-foreground text-sm font-medium hover:underline hover:text-primary transition-colors duration-200"
      >
        {{ getFullName }}
      </router-link>
      <router-link
        v-else-if="canManageUsers"
        :to="{ name: 'edit-agent', params: { id: message.author?.id } }"
        class="cursor-pointer text-muted-foreground text-sm font-medium hover:underline hover:text-primary transition-colors duration-200"
      >
        {{ getFullName }}
      </router-link>
      <p v-else class="text-muted-foreground text-sm font-medium">
        {{ getFullName }}
      </p>
      <Tooltip>
        <TooltipTrigger as-child>
          <span class="text-muted-foreground/60 text-xs">
            {{ formatMessageTimestamp(message.created_at) }}
          </span>
        </TooltipTrigger>
        <TooltipContent>
          <p>{{ formatFullTimestamp(message.created_at) }}</p>
        </TooltipContent>
      </Tooltip>
    </div>

    <!-- Message Bubble -->
    <div class="flex flex-row gap-2 w-full" :class="{ 'justify-end': isOutgoing }">
      <!-- Avatar (left for incoming) -->
      <router-link
        v-if="!isOutgoing"
        :to="{ name: 'contact-detail', params: { id: message.author?.id } }"
        class="flex-shrink-0"
      >
        <Avatar class="cursor-pointer w-8 h-8 hover:opacity-80 transition-opacity">
          <AvatarImage :src="getAvatar" />
          <AvatarFallback class="font-medium">
            {{ avatarFallback }}
          </AvatarFallback>
        </Avatar>
      </router-link>

      <!-- Bubble Wrapper with max 80% width -->
      <div
        class="w-4/5"
        :class="{ 'flex justify-end': isOutgoing }"
        style="contain: inline-size"
      >
        <div
          class="flex flex-col justify-end message-bubble"
          :class="bubbleClasses"
        >
          <!-- T3y PCI Data Warning. Detected on ingest by go-pci-scrub;
               banner stays visible until the agent clicks Redact Now or
               the 7-day auto-redact safety net fires. -->
          <div
            v-if="message.has_pci_data"
            class="flex items-center gap-2 mb-2 px-3 py-2 bg-red-50 dark:bg-red-950 border border-red-200 dark:border-red-800 rounded-md"
          >
            <ShieldAlert class="w-4 h-4 text-red-500 shrink-0" />
            <span class="text-xs text-red-700 dark:text-red-300 font-medium flex-1">
              {{ t('conversation.pciDataWarning') }}
            </span>
            <button
              v-if="!redacting"
              type="button"
              class="text-xs font-medium text-red-600 dark:text-red-400 hover:text-red-800 dark:hover:text-red-200 underline"
              @click="redactPCI"
            >
              {{ t('conversation.pciRedactNow') }}
            </button>
            <Spinner v-if="redacting" size="sm" />
          </div>

          <!-- Message Envelope -->
          <MessageEnvelope :message="message" v-if="showEnvelope" />

          <hr class="mb-2" v-if="showEnvelope" />

          <!-- Message Content -->
          <div
            v-if="message.content_type === 'text'"
            class="mb-1 native-html whitespace-pre-wrap"
            :class="{ 'mb-3': message.attachments.length > 0 }"
          >
            {{ sanitizedContent }}
          </div>
          <div v-else ref="messageContentEl" @click="onMessageContentClick">
            <!--
              ER6: dropped whitespace-pre-wrap. HTML emails carry meaningful
              whitespace between tags (indentation in the source) that pre-wrap
              would otherwise render as visible blank space, producing big
              gaps. The Letter component already preserves intra-paragraph
              whitespace via its own normalisation; pre-wrap on top breaks
              email layouts that depend on the browser collapsing whitespace.
            -->
            <Letter
              :key="sanitizedContent"
              :html="sanitizedContent"
              :allowedSchemas="['cid', 'https', 'http', 'mailto']"
              class="mb-1 native-html break-words"
              :class="{ 'mb-3': message.attachments.length > 0 }"
            />
          </div>

          <ImageLightbox
            v-model="inlineLightboxOpen"
            :images="inlineImages"
            :start-index="inlineLightboxIndex"
          />

          <!-- Quoted Text Toggle (incoming only) -->
          <div
            v-if="!isOutgoing && hasQuotedContent"
            @click="toggleQuote"
            class="text-xs cursor-pointer text-muted-foreground px-2 py-1 w-max hover:bg-muted hover:text-primary rounded transition-colors duration-200"
          >
            {{ showQuotedText ? t('conversation.hideQuotedText') : t('conversation.showQuotedText') }}
          </div>

          <!-- Attachments -->
          <MessageAttachmentPreview :attachments="nonInlineAttachments" />

          <!-- CSAT Response -->
          <CSATResponseDisplay :message="message" />

          <!-- Spinner for Pending Messages (outgoing only) -->
          <Spinner v-if="isOutgoing && message.status === 'pending'" size="sm" />

          <!-- Status Icons (outgoing only) -->
          <div v-if="isOutgoing" class="flex items-center space-x-2 mt-2 self-end">
            <Lock :size="10" v-if="isPrivateMessage" class="text-muted-foreground" />
            <Check :size="14" v-if="showCheckCheck" class="text-green-500" />
            <Tooltip v-if="message.meta?.continuity_emailed">
              <TooltipTrigger>
                <Mail :size="12" class="text-muted-foreground" />
              </TooltipTrigger>
              <TooltipContent>
                <p>{{ t('conversation.sentViaEmail') }}</p>
              </TooltipContent>
            </Tooltip>
            <RotateCcw
              size="10"
              @click="retryMessage(message)"
              class="cursor-pointer text-muted-foreground hover:text-foreground transition-colors duration-200"
              v-if="showRetry"
            />
          </div>

          <!-- Edit/Delete actions for private notes (author only, hidden once deleted) -->
          <div
            v-if="canEditPrivateNote && !isEditing"
            class="flex items-center gap-3 mt-1.5 self-end"
          >
            <button
              type="button"
              @click="startEdit"
              class="text-xs text-muted-foreground/60 hover:text-foreground transition-colors flex items-center gap-1"
            >
              <Pencil :size="12" />
              <span>{{ t('globals.messages.edit') }}</span>
            </button>
            <button
              type="button"
              @click="confirmDelete"
              class="text-xs text-muted-foreground/60 hover:text-destructive transition-colors flex items-center gap-1"
            >
              <Trash2 :size="12" />
              <span>{{ t('globals.messages.delete') }}</span>
            </button>
          </div>

          <!-- Inline edit textarea -->
          <div v-if="isEditing" class="mt-2 w-full">
            <textarea
              ref="editTextareaEl"
              v-model="editContent"
              class="w-full min-h-[60px] p-2 text-sm border rounded bg-background text-foreground resize-y"
              @keydown.escape="cancelEdit"
            />
            <div class="flex items-center gap-3 mt-1 justify-end">
              <button
                type="button"
                @click="cancelEdit"
                class="text-xs text-muted-foreground hover:text-foreground"
              >
                {{ t('globals.messages.cancel') }}
              </button>
              <button
                type="button"
                @click="saveEdit"
                :disabled="isSavingEdit || !editContent.trim()"
                class="text-xs text-primary font-medium hover:underline disabled:opacity-50"
              >
                {{ isSavingEdit ? t('globals.messages.saving') : t('globals.messages.save') }}
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- Forwarded-to badge under outgoing forwards -->
      <div
        v-if="forwardedToLabel"
        class="flex items-center gap-1 mt-1 pr-[47px] justify-end"
      >
        <span class="text-xs text-muted-foreground/70 italic flex items-center gap-1">
          <Forward :size="12" />
          {{ t('conversation.forwardedTo', { recipients: forwardedToLabel }) }}
        </span>
      </div>

      <!-- Reply-to-forward badge under tagged incoming messages -->
      <div
        v-if="isFromForward"
        class="flex items-center gap-1 mt-1 pl-[47px]"
      >
        <span class="text-xs text-muted-foreground/70 italic flex items-center gap-1">
          <Forward :size="12" />
          {{ t('conversation.internalReplyToForward') }}
        </span>
      </div>

      <!-- Avatar (right for outgoing) -->
      <router-link
        v-if="isOutgoing && canManageUsers"
        :to="{ name: 'edit-agent', params: { id: message.author?.id } }"
        class="flex-shrink-0"
      >
        <Avatar class="cursor-pointer w-8 h-8 hover:opacity-80 transition-opacity">
          <AvatarImage :src="getAvatar" />
          <AvatarFallback class="font-medium">
            {{ avatarFallback }}
          </AvatarFallback>
        </Avatar>
      </router-link>
      <Avatar v-else-if="isOutgoing" class="w-8 h-8">
        <AvatarImage :src="getAvatar" />
        <AvatarFallback class="font-medium">
          {{ avatarFallback }}
        </AvatarFallback>
      </Avatar>
    </div>

    <!-- Forward action — email-channel, non-private, non-activity messages only. -->
    <div
      v-if="canForward"
      class="flex items-center gap-1 mt-1"
      :class="isOutgoing ? 'pr-[47px] justify-end' : 'pl-[47px]'"
    >
      <button
        type="button"
        @click="forwardMessage"
        class="text-xs text-muted-foreground/60 hover:text-foreground transition-colors flex items-center gap-1"
      >
        <Forward :size="12" />
        <span>{{ t('conversation.forward') }}</span>
      </button>
    </div>

    <!-- UX10: timestamp moved up to the sender name row above. -->
  </div>
</template>

<script setup>
import { computed, nextTick, ref } from 'vue'
import { useConversationStore } from '@main/stores/conversation'
import { useUserStore } from '@main/stores/user'
import { useEmitter } from '@main/composables/useEmitter'
import { useToast } from '@main/composables/useToast'
import { EMITTER_EVENTS } from '@main/constants/emitterEvents'
import { useI18n } from 'vue-i18n'
import { Lock, Mail, RotateCcw, Check, Pencil, Trash2, Forward, ShieldAlert } from 'lucide-vue-next'
import { Tooltip, TooltipContent, TooltipTrigger } from '@shared-ui/components/ui/tooltip'
import { Spinner } from '@shared-ui/components/ui/spinner'
import { formatMessageTimestamp, formatFullTimestamp } from '@shared-ui/utils/datetime.js'
import { Avatar, AvatarFallback, AvatarImage } from '@shared-ui/components/ui/avatar'
import { Letter } from 'vue-letter'
import ImageLightbox from '@/components/ImageLightbox.vue'
import MessageAttachmentPreview from '@main/features/conversation/message/attachment/MessageAttachmentPreview.vue'
import MessageEnvelope from './MessageEnvelope.vue'
import CSATResponseDisplay from './CSATResponseDisplay.vue'
import api from '@main/api'

const props = defineProps({
  message: Object,
  direction: {
    type: String,
    validator: (v) => ['incoming', 'outgoing'].includes(v)
  }
})

const convStore = useConversationStore()
const { t } = useI18n()
const userStore = useUserStore()

const isSystemUser = computed(() => props.message.author?.email === 'System')
const canManageUsers = computed(() => !isSystemUser.value && userStore.can('users:manage'))

// Direction helpers
const isOutgoing = computed(() => props.direction === 'outgoing')

// Author info from message
const getFullName = computed(() => {
  const author = props.message.author ?? {}
  const firstName = author.first_name ?? 'User'
  const lastName = author.last_name ?? ''
  return `${firstName} ${lastName}`.trim()
})

const getAvatar = computed(() => {
  return props.message.author?.avatar_url || ''
})

const avatarFallback = computed(() => {
  const firstName = props.message.author?.first_name ?? (isOutgoing.value ? 'A' : 'U')
  return firstName.toUpperCase().substring(0, 2)
})

const sanitizedContent = computed(() => {
  if (props.message.meta?.is_csat) {
    return t('globals.messages.pleaseRateConversation')
  }
  let content = props.message.content || ''
  // ER6: strip empty paragraphs (just &nbsp; / <br> / whitespace). Outlook +
  // some Gmail templates embed runs of these between content blocks; without
  // stripping, the conversation thread shows tall vertical gaps that make
  // long messages hard to read.
  if (props.message.content_type !== 'text') {
    content = content.replace(/<p[^>]*>\s*(&nbsp;| |\s|<br\s*\/?>)*\s*<\/p>/gi, '')
  }
  return content
})

const nonInlineAttachments = computed(() =>
  props.message.attachments.filter((attachment) => attachment.disposition !== 'inline')
)

// Forward-related: outgoing forward (meta.forwarded_to) OR incoming reply
// to a forward (meta.from_forward, tagged at IMAP intake). Both share the
// private-note bubble color so the internal forward thread visually groups
// distinct from customer-facing messages.
const isForwardRelated = computed(
  () => Boolean(props.message.meta?.forwarded_to) || Boolean(props.message.meta?.from_forward)
)

// Bubble classes - conditional based on direction
// UX6: agent (outgoing non-private) and customer (incoming) get distinct
// subtle background fills so the conversation reads as a back-and-forth
// rather than a wall of identically-styled bubbles. Tokens (--bubble-agent /
// --bubble-customer) live in shared-ui/main.scss so dark-mode is handled.
const bubbleClasses = computed(() => ({
  // Outgoing-specific: private message styling
  'bg-private': (isOutgoing.value && props.message.private) || isForwardRelated.value,
  'border border-border': isOutgoing.value && !props.message.private && !isForwardRelated.value,
  'bg-bubble-agent': isOutgoing.value && !props.message.private && !isForwardRelated.value,
  'bg-bubble-customer': !isOutgoing.value,
  'opacity-50 animate-pulse': isOutgoing.value && props.message.status === 'pending',
  'border-destructive': isOutgoing.value && props.message.status === 'failed',
  relative: isOutgoing.value,
  // Incoming-specific: quoted text visibility
  'show-quoted-text': !isOutgoing.value && showQuotedText.value,
  'hide-quoted-text': !isOutgoing.value && !showQuotedText.value
}))

// Outgoing-only computed properties
const isPrivateMessage = computed(() => isOutgoing.value && props.message.private)
const showCheckCheck = computed(
  () => isOutgoing.value && props.message.status === 'sent' && !isPrivateMessage.value
)
const showRetry = computed(() => isOutgoing.value && props.message.status === 'failed' && props.message.sender_id === userStore.userID)

const retryMessage = (msg) => {
  api.retryMessage(convStore.current.uuid, msg.uuid)
}

// T3y manual redact. Confirm-then-fire because the redaction is irreversible
// at the DB level (the original content is rewritten in-place; even rolling
// back from a backup would expose the card again). Reload after success so
// the scrubbed content + cleared has_pci_data flag both refresh — cheaper
// than a per-message store mutation since this is a rare/destructive path.
const redacting = ref(false)
const redactPCI = async () => {
  if (
    !confirm(
      t('conversation.pciRedactConfirm') ||
        'This will permanently redact credit card data from this message and attempt to delete the original email. This cannot be undone.'
    )
  ) {
    return
  }
  redacting.value = true
  try {
    await api.redactMessagePCI(convStore.current.uuid, props.message.uuid)
    window.location.reload()
  } catch (err) {
    redacting.value = false
    toast.error(err?.response?.data?.message || err.message || 'Redaction failed')
  }
}

// Edit/delete private notes — only the author of the note can mutate it.
// Hide the controls once meta.deleted is set so an already-tombstoned note
// can't be re-edited or re-deleted.
const emitter = useEmitter()
const toast = useToast()
const isNoteDeleted = computed(() => Boolean(props.message.meta?.deleted))
const isNoteAuthor = computed(() => props.message.sender_id === userStore.userID)
const canEditPrivateNote = computed(
  () => isPrivateMessage.value && isNoteAuthor.value && !isNoteDeleted.value
)

const isEditing = ref(false)
const isSavingEdit = ref(false)
const editContent = ref('')
const editTextareaEl = ref(null)

const startEdit = () => {
  // Strip surrounding tags to get plain text — the textarea is a deliberate
  // simplification (no rich-text editor for notes). Saving wraps in <p> and
  // converts newlines to <br>, mirroring how the message was originally sent.
  const tmp = document.createElement('div')
  tmp.innerHTML = props.message.content || ''
  editContent.value = (tmp.textContent || '').trim()
  isEditing.value = true
  nextTick(() => editTextareaEl.value?.focus())
}

const cancelEdit = () => {
  isEditing.value = false
  editContent.value = ''
}

const saveEdit = async () => {
  const trimmed = editContent.value.trim()
  if (!trimmed) return
  isSavingEdit.value = true
  try {
    const html = '<p>' + trimmed.replace(/\n/g, '<br>') + '</p>'
    await api.updatePrivateNote(convStore.current.uuid, props.message.uuid, html)
    convStore.mergeMessageUpdate({
      conversation_uuid: convStore.current.uuid,
      uuid: props.message.uuid,
      content: html
    })
    isEditing.value = false
  } catch (err) {
    toast.error(err)
  } finally {
    isSavingEdit.value = false
  }
}

const confirmDelete = async () => {
  if (!window.confirm(t('conversation.deletePrivateNoteConfirm'))) return
  try {
    await api.deletePrivateNote(convStore.current.uuid, props.message.uuid)
    // Mirror the server-side tombstone locally via the store so the bubble
    // updates without a refetch. The server replaces content with
    // "This note was deleted by <name>".
    convStore.mergeMessageUpdate({
      conversation_uuid: convStore.current.uuid,
      uuid: props.message.uuid,
      content: t('conversation.privateNoteDeleted'),
      meta: { ...(props.message.meta || {}), deleted: true }
    })
  } catch (err) {
    toast.error(err)
  }
}

// Forward UI: emit a global event so ReplyBox can pick it up and switch
// into forward mode with the original message pre-quoted.
const forwardedToLabel = computed(() => {
  const fwd = props.message.meta?.forwarded_to
  if (Array.isArray(fwd) && fwd.length > 0) return fwd.join(', ')
  return ''
})
const isFromForward = computed(() => Boolean(props.message.meta?.from_forward))
const canForward = computed(
  () =>
    convStore.current?.inbox_channel === 'email' &&
    !props.message.private
)
const forwardMessage = () => {
  emitter.emit(EMITTER_EVENTS.FORWARD_MESSAGE, props.message)
}

// Incoming-only: quoted text toggle
const showQuotedText = ref(false)
// ER2: extend quote detection beyond <blockquote> to cover Gmail's
// gmail_quote wrapper and Outlook's #divRplyFwdMsg. Both surface in incoming
// replies but Outlook in particular doesn't use blockquote — the original
// "include quoted text" toggle missed them entirely, so the prior thread
// always rendered inline.
const hasQuotedContent = computed(
  () =>
    !isOutgoing.value &&
    (sanitizedContent.value.includes('<blockquote') ||
      sanitizedContent.value.includes('gmail_quote') ||
      sanitizedContent.value.includes('divRplyFwdMsg'))
)
const toggleQuote = () => {
  showQuotedText.value = !showQuotedText.value
}

// Inline image lightbox: click an <img> in the rendered email body to open it.
// We enumerate images from the rendered DOM rather than the HTML source so we
// inherit vue-letter's sanitization and don't have to parse HTML with regex
// (which trips on attributes containing '>' and similar edge cases).
const messageContentEl = ref(null)
const inlineLightboxOpen = ref(false)
const inlineLightboxIndex = ref(0)
const inlineImages = ref([])

// Re-walk the rendered <img> set on click. Cheaper than maintaining a watcher
// on sanitizedContent, and always reflects what the user actually sees.
const refreshInlineImages = () => {
  const root = messageContentEl.value
  if (!root) {
    inlineImages.value = []
    return
  }
  inlineImages.value = Array.from(root.querySelectorAll('img'))
    .map((el) => ({ url: el.getAttribute('src'), name: el.getAttribute('alt') || '' }))
    .filter((img) => img.url)
}

const onMessageContentClick = (event) => {
  // Walk up so clicks on nested wrappers (e.g. <a><img></a>) still resolve.
  const img = event.target?.closest?.('img')
  if (!img || !messageContentEl.value?.contains(img)) return

  // If the image is inside an anchor, suppress the navigation so the
  // lightbox can take over.
  const wrappingAnchor = img.closest('a')
  if (wrappingAnchor && messageContentEl.value.contains(wrappingAnchor)) {
    event.preventDefault()
  }

  refreshInlineImages()
  const src = img.getAttribute('src')
  const idx = inlineImages.value.findIndex((entry) => entry.url === src)
  inlineLightboxIndex.value = idx >= 0 ? idx : 0
  inlineLightboxOpen.value = true
}

// Envelope visibility (both directions)
const showEnvelope = computed(() => {
  return (
    props.message.meta?.from?.length ||
    props.message.meta?.to?.length ||
    props.message.meta?.cc?.length ||
    props.message.meta?.bcc?.length ||
    props.message.meta?.subject
  )
})
</script>