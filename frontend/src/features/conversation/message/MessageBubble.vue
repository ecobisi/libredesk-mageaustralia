<template>
  <div class="flex flex-col text-left" :class="isOutgoing ? 'items-end' : 'items-start'">
    <!-- Sender Name + Timestamp -->
    <div class="mb-1 flex items-baseline gap-2" :class="isOutgoing ? 'pr-[47px] justify-end' : 'pl-[47px]'">
      <router-link
        v-if="!isOutgoing"
        :to="{ name: 'contact-detail', params: { id: message.author?.id } }"
        class="text-muted-foreground text-sm font-medium hover:underline hover:text-primary"
      >
        {{ getFullName }}
      </router-link>
      <p v-else class="text-muted-foreground text-sm font-medium">
        {{ getFullName }}
      </p>
      <Tooltip>
        <TooltipTrigger>
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
          <!-- PCI Data Warning -->
          <div v-if="message.has_pci_data" class="flex items-center gap-2 mb-2 px-3 py-2 bg-red-50 dark:bg-red-950 border border-red-200 dark:border-red-800 rounded-md">
            <ShieldAlert class="w-4 h-4 text-red-500 shrink-0" />
            <span class="text-xs text-red-700 dark:text-red-300 font-medium flex-1">This message contains credit card data</span>
            <button
              v-if="!redacting"
              class="text-xs font-medium text-red-600 dark:text-red-400 hover:text-red-800 dark:hover:text-red-200 underline"
              @click="redactPCI"
            >
              Redact Now
            </button>
            <Spinner v-if="redacting" size="w-3 h-3" />
          </div>

          <!-- Message Envelope -->
          <MessageEnvelope :message="message" v-if="showEnvelope" />

          <hr class="mb-2" v-if="showEnvelope" />

          <!-- Deleted note tombstone -->
          <div v-if="isDeleted" class="mb-1 text-sm text-muted-foreground italic flex items-center gap-1.5">
            <Trash2 class="w-3.5 h-3.5" />
            {{ message.content }}
          </div>

          <!-- Message Content -->
          <div
            v-if="!isDeleted && message.content_type === 'text'"
            class="mb-1 native-html whitespace-pre-wrap"
            :class="{ 'mb-3': message.attachments.length > 0 }"
          >
            {{ sanitizedContent }}
          </div>
          <div
            v-else-if="!isDeleted"
            ref="contentEl"
            @click="handleContentClick"
            class="mb-1 native-html break-words"
            :class="{ 'mb-3': message.attachments.length > 0 }"
          >
            <Letter
              :html="sanitizedContent"
              :rewriteExternalResources="rewriteResource"
              :allowedSchemas="['cid', 'https', 'http', 'mailto']"
            />
          </div>

          <!-- Quoted Text Toggle (incoming only) -->
          <div
            v-if="hasQuotedContent"
            @click="toggleQuote"
            class="text-xs cursor-pointer text-muted-foreground px-2 py-1 w-max hover:bg-muted hover:text-primary rounded transition-all"
          >
            {{ showQuotedText ? t('conversation.hideQuotedText') : t('conversation.showQuotedText') }}
          </div>

          <!-- Attachments -->
          <MessageAttachmentPreview :attachments="nonInlineAttachments" />

          <!-- Spinner for Pending Messages (outgoing only) -->
          <Spinner v-if="isOutgoing && message.status === 'pending'" size="w-4 h-4" />

          <!-- Status Icons (outgoing only) -->
          <div v-if="isOutgoing" class="flex items-center space-x-2 mt-2 self-end">
            <Lock :size="10" v-if="isPrivateMessage" class="text-muted-foreground" />
            <Check :size="14" v-if="showCheckCheck" class="text-green-500" />
            <RotateCcw
              size="10"
              @click="retryMessage(message)"
              class="cursor-pointer text-muted-foreground hover:text-foreground transition-colors duration-200"
              v-if="showRetry"
            />
          </div>

          <!-- Edit/Delete for private notes -->
          <div v-if="isPrivateMessage && !isDeleted" class="flex items-center gap-2 mt-1.5 self-end">
            <button
              v-if="!isEditing"
              @click="startEdit"
              class="text-xs text-muted-foreground/50 hover:text-muted-foreground transition-colors flex items-center gap-0.5"
            >
              <Pencil class="w-3 h-3" />
              <span>Edit</span>
            </button>
            <button
              v-if="!isEditing"
              @click="confirmDelete"
              class="text-xs text-muted-foreground/50 hover:text-red-500 transition-colors flex items-center gap-0.5"
            >
              <Trash2 class="w-3 h-3" />
              <span>Delete</span>
            </button>
          </div>

          <!-- Inline edit area -->
          <div v-if="isEditing" class="mt-2 w-full">
            <textarea
              ref="editTextarea"
              v-model="editContent"
              class="w-full min-h-[60px] p-2 text-sm border rounded bg-background text-foreground resize-y"
              @keydown.escape="cancelEdit"
            />
            <div class="flex items-center gap-2 mt-1 justify-end">
              <button @click="cancelEdit" class="text-xs text-muted-foreground hover:text-foreground">Cancel</button>
              <button @click="saveEdit" class="text-xs text-primary font-medium hover:underline" :disabled="isSaving">
                {{ isSaving ? 'Saving...' : 'Save' }}
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- Avatar (right for outgoing) -->
      <Avatar v-if="isOutgoing" class="cursor-pointer w-8 h-8">
        <AvatarImage :src="getAvatar" />
        <AvatarFallback class="font-medium">
          {{ avatarFallback }}
        </AvatarFallback>
      </Avatar>
    </div>

    <!-- Forward button (email only) -->
    <div
      v-if="!isActivity && !message.private && (!convStore.current?.inbox_channel || convStore.current?.inbox_channel === 'email')"
      class="flex items-center gap-1 mt-1"
      :class="isOutgoing ? 'pr-[47px] justify-end' : 'pl-[47px]'"
    >
      <button
        @click="forwardMessage"
        class="text-xs text-muted-foreground/50 hover:text-muted-foreground transition-colors flex items-center gap-1"
      >
        <Forward class="w-3 h-3" />
        <span>Forward</span>
      </button>
    </div>

    <!-- Forwarded badge -->
    <div
      v-if="forwardedTo"
      class="flex items-center gap-1 mt-1"
      :class="isOutgoing ? 'pr-[47px] justify-end' : 'pl-[47px]'"
    >
      <span class="text-xs text-muted-foreground/70 italic flex items-center gap-1">
        <Forward class="w-3 h-3" />
        Forwarded to: {{ forwardedTo }}
      </span>
    </div>

    <!-- Reply landed via the forwarded thread, internal-only — never auto-quoted to the customer. -->
    <div
      v-if="isFromForward"
      class="flex items-center gap-1 mt-1"
      :class="isOutgoing ? 'pr-[47px] justify-end' : 'pl-[47px]'"
    >
      <span class="text-xs text-muted-foreground/70 italic flex items-center gap-1">
        <Forward class="w-3 h-3" />
        Reply to forward (internal)
      </span>
    </div>

  </div>

  <!-- Inline image lightbox -->
  <Teleport to="body">
    <div
      v-if="inlineLightboxOpen"
      class="fixed inset-0 z-[9999] flex items-center justify-center bg-black/80"
      @click.self="zoomScale === 1 ? (inlineLightboxOpen = false) : resetZoom()"
      @keydown.escape="inlineLightboxOpen = false"
      @keydown.left="prevInlineImage"
      @keydown.right="nextInlineImage"
      @wheel.prevent="handleZoomWheel"
      tabindex="0"
      ref="inlineLightboxEl"
    >
      <!-- Top bar -->
      <div class="absolute top-4 right-4 flex items-center gap-3 z-10">
        <button class="text-white/70 hover:text-white flex items-center gap-1 text-sm" @click.stop="zoomIn" title="Zoom in">
          <ZoomIn :size="20" />
        </button>
        <button class="text-white/70 hover:text-white text-xs font-mono min-w-[3rem] text-center" @click.stop="resetZoom" title="Reset zoom">
          {{ Math.round(zoomScale * 100) }}%
        </button>
        <button class="text-white/70 hover:text-white flex items-center gap-1 text-sm" @click.stop="zoomOut" title="Zoom out">
          <ZoomOut :size="20" />
        </button>
        <a :href="currentInlineImage" download class="text-white/70 hover:text-white" title="Download" @click.stop>
          <Download :size="20" />
        </a>
        <button class="text-white hover:text-gray-300" @click="inlineLightboxOpen = false">
          <X :size="24" />
        </button>
      </div>
      <!-- Counter -->
      <div v-if="inlineImages.length > 1" class="absolute top-4 left-4 text-white/70 text-sm z-10">
        {{ inlineLightboxIndex + 1 }} / {{ inlineImages.length }}
      </div>
      <!-- Prev -->
      <button v-if="inlineImages.length > 1" class="absolute left-4 top-1/2 -translate-y-1/2 text-white hover:text-gray-300 z-10 p-2" @click.stop="prevInlineImage">
        <ChevronLeft :size="32" />
      </button>
      <!-- Next -->
      <button v-if="inlineImages.length > 1" class="absolute right-4 top-1/2 -translate-y-1/2 text-white hover:text-gray-300 z-10 p-2" @click.stop="nextInlineImage">
        <ChevronRight :size="32" />
      </button>
      <!-- Zoomable image -->
      <div
        :class="zoomScale > 1 ? 'overflow-visible' : 'overflow-hidden'"
        style="max-width: 90vw; max-height: 90vh;"
        @mousedown.prevent="startPan"
        @touchstart.prevent="handleTouchStart"
        @touchmove.prevent="handleTouchMove"
        @touchend="handleTouchEnd"
      >
        <img
          :src="currentInlineImage"
          class="max-w-[90vw] max-h-[90vh] object-contain rounded shadow-2xl select-none"
          :style="{ transform: `scale(${zoomScale}) translate(${panX / zoomScale}px, ${panY / zoomScale}px)`, cursor: zoomScale > 1 ? 'grab' : 'zoom-in', transition: isPanning ? 'none' : 'transform 0.15s ease' }"
          draggable="false"
          @click.stop="zoomScale === 1 ? zoomIn() : null"
          @dblclick.stop="resetZoom"
        />
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { computed, ref, nextTick } from 'vue'
import { useConversationStore } from '@/stores/conversation'
import { useUserStore } from '@/stores/user'
import { useAppSettingsStore } from '@/stores/appSettings'
import { useI18n } from 'vue-i18n'
import { Lock, RotateCcw, Check, ShieldAlert, Forward, Pencil, Trash2, X, Download, ChevronLeft, ChevronRight, ZoomIn, ZoomOut } from 'lucide-vue-next'
import { revertCIDToImageSrc } from '@/utils/strings'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { Spinner } from '@/components/ui/spinner'
import { formatMessageTimestamp, formatFullTimestamp } from '@/utils/datetime'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { Letter } from 'vue-letter'
import MessageAttachmentPreview from '@/features/conversation/message/attachment/MessageAttachmentPreview.vue'
import MessageEnvelope from './MessageEnvelope.vue'
import api from '@/api'
import { useEmitter } from '@/composables/useEmitter'
import { EMITTER_EVENTS } from '@/constants/emitterEvents.js'

const props = defineProps({
  message: Object,
  direction: {
    type: String,
    validator: (v) => ['incoming', 'outgoing'].includes(v)
  }
})

const emitter = useEmitter()

const convStore = useConversationStore()
const settingsStore = useAppSettingsStore()
const { t } = useI18n()

// Edit/delete state
const isEditing = ref(false)
const editContent = ref('')
const editTextarea = ref(null)
const isSaving = ref(false)

const isDeleted = computed(() => {
  return props.message.meta?.deleted === true
})

const startEdit = () => {
  // Strip HTML for textarea editing
  const doc = new DOMParser().parseFromString(props.message.content || '', 'text/html')
  editContent.value = doc.body.textContent || ''
  isEditing.value = true
  nextTick(() => editTextarea.value?.focus())
}

const cancelEdit = () => {
  isEditing.value = false
  editContent.value = ''
}

const saveEdit = async () => {
  if (!editContent.value.trim()) return
  isSaving.value = true
  try {
    const cuuid = convStore.current?.uuid
    const htmlContent = '<p>' + editContent.value.replace(/\n/g, '<br>') + '</p>'
    await api.updatePrivateNote(cuuid, props.message.uuid, htmlContent)
    isEditing.value = false
    // Update both the local reactive object and the cache
    props.message.content = htmlContent
  } catch (err) {
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
      variant: 'destructive',
      description: 'Failed to update note: ' + (err?.response?.data?.message || err.message)
    })
  } finally {
    isSaving.value = false
  }
}

const confirmDelete = async () => {
  if (!confirm('Delete this private note? The content will be removed and cannot be recovered.')) return
  try {
    const cuuid = convStore.current?.uuid
    await api.deletePrivateNote(cuuid, props.message.uuid)
    // Update local reactive object directly
    props.message.content = 'This note was deleted'
    if (!props.message.meta) props.message.meta = {}
    props.message.meta.deleted = true
  } catch (err) {
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
      variant: 'destructive',
      description: 'Failed to delete note: ' + (err?.response?.data?.message || err.message)
    })
  }
}

// Activity check
const isActivity = computed(() => props.message.type === 'activity')

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

// Content sanitization - different processing for incoming vs outgoing
const sanitizedContent = computed(() => {
  let content = props.message.content || ''

  if (isOutgoing.value) {
    return revertCIDToImageSrc(content)
  } else {
    const baseUrl = settingsStore.settings['app.root_url']
    content = props.message.attachments.reduce(
      (acc, { content_id, url }) => acc.replace(new RegExp(`cid:${content_id}`, 'g'), url),
      content
    )
    content = content.replace(/src="\/uploads\//g, `src="${baseUrl}/uploads/`)
    // Strip runs of 3+ consecutive empty paragraphs down to one (preserve intentional spacing)
    content = content.replace(/(<p[^>]*>\s*(&nbsp;|\u00a0|\s|<br\s*\/?>)*\s*<\/p>\s*){3,}/gi, '<p>&nbsp;</p>')
    return content
  }
})

const rewriteResource = (url) => {
  const baseUrl = settingsStore.settings['app.root_url'] || ''
  // Rewrite relative /uploads/ paths to absolute
  if (url.startsWith('/uploads/')) {
    return baseUrl + url
  }
  return url
}

const nonInlineAttachments = computed(() =>
  props.message.attachments.filter((attachment) => attachment.disposition !== 'inline')
)

// Bubble classes - conditional based on direction
const redacting = ref(false)

const redactPCI = async () => {
  if (!confirm('This will permanently redact credit card data from this message and attempt to delete the original email. This cannot be undone.')) return
  redacting.value = true
  try {
    const cuuid = convStore.current?.uuid
    await api.redactMessagePCI(cuuid, props.message.uuid)
    window.location.reload()
  } catch (err) {
    redacting.value = false
    alert('Failed to redact: ' + (err?.response?.data?.message || err.message))
  }
}

const bubbleClasses = computed(() => ({
  // Outgoing-specific: private message styling
  'bg-private': isOutgoing.value && props.message.private,
  'border border-border': isOutgoing.value && !props.message.private,
  'bg-agent-bubble': isOutgoing.value && !props.message.private,
  'bg-customer-bubble': !isOutgoing.value,
  'opacity-50 animate-pulse': isOutgoing.value && props.message.status === 'pending',
  'border-red-400': isOutgoing.value && props.message.status === 'failed',
  relative: isOutgoing.value,
  // Incoming-specific: quoted text visibility
  'show-quoted-text': showQuotedText.value,
  'hide-quoted-text': hasQuotedContent.value && !showQuotedText.value
}))

// Outgoing-only computed properties
const isPrivateMessage = computed(() => isOutgoing.value && props.message.private)
const showCheckCheck = computed(
  () => isOutgoing.value && props.message.status === 'sent' && !isPrivateMessage.value
)
const userStore = useUserStore()
const showRetry = computed(() => isOutgoing.value && props.message.status === 'failed' && props.message.sender_id === userStore.userID)

const retryMessage = (msg) => {
  api.retryMessage(convStore.current.uuid, msg.uuid)
}

// Incoming-only: quoted text toggle
const showQuotedText = ref(false)
const hasQuotedContent = computed(
  () => sanitizedContent.value.includes('<blockquote') || sanitizedContent.value.includes('gmail_quote') || sanitizedContent.value.includes('divRplyFwdMsg')
)
const toggleQuote = () => {
  showQuotedText.value = !showQuotedText.value
}

// Forward functionality
const forwardedTo = computed(() => {
  if (!props.message.meta?.forwarded) return null
  const fwdTo = props.message.meta?.forwarded_to
  if (Array.isArray(fwdTo)) return fwdTo.join(', ')
  return null
})

// True when this incoming message is a reply (or chained reply) to a
// previously-forwarded email. Tagged server-side at IMAP intake; the reply
// box's auto-thread builder filters these out so internal back-and-forth
// with a forward recipient never leaks into a customer-facing email.
const isFromForward = computed(() => Boolean(props.message.meta?.from_forward))

const forwardMessage = () => {
  emitter.emit('forward-message', props.message)
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

// Inline image lightbox
const contentEl = ref(null)
const inlineLightboxOpen = ref(false)
const inlineLightboxIndex = ref(0)
const inlineLightboxEl = ref(null)
const inlineImages = ref([])

// Zoom & pan state
const zoomScale = ref(1)
const panX = ref(0)
const panY = ref(0)
const isPanning = ref(false)
let panStart = { x: 0, y: 0, panX: 0, panY: 0 }
let lastTouchDist = 0

const currentInlineImage = computed(() => inlineImages.value[inlineLightboxIndex.value] || '')

function handleContentClick(e) {
  const img = e.target.closest('img')
  if (!img) return
  e.preventDefault()
  const imgs = Array.from(contentEl.value.querySelectorAll('img'))
    .map(el => el.src)
    .filter(src => src && !src.startsWith('data:'))
  if (imgs.length === 0) return
  inlineImages.value = imgs
  const idx = imgs.indexOf(img.src)
  inlineLightboxIndex.value = idx >= 0 ? idx : 0
  resetZoom()
  inlineLightboxOpen.value = true
  nextTick(() => inlineLightboxEl.value?.focus())
}

function resetZoom() {
  zoomScale.value = 1
  panX.value = 0
  panY.value = 0
}

function zoomIn() {
  zoomScale.value = Math.min(zoomScale.value * 1.4, 8)
}

function zoomOut() {
  zoomScale.value = Math.max(zoomScale.value / 1.4, 1)
  if (zoomScale.value === 1) { panX.value = 0; panY.value = 0 }
}

function handleZoomWheel(e) {
  if (e.deltaY < 0) {
    zoomScale.value = Math.min(zoomScale.value * 1.15, 8)
  } else {
    zoomScale.value = Math.max(zoomScale.value / 1.15, 1)
    if (zoomScale.value === 1) { panX.value = 0; panY.value = 0 }
  }
}

// Mouse drag to pan
function startPan(e) {
  if (zoomScale.value <= 1) return
  isPanning.value = true
  panStart = { x: e.clientX, y: e.clientY, panX: panX.value, panY: panY.value }
  const onMove = (ev) => {
    panX.value = panStart.panX + (ev.clientX - panStart.x)
    panY.value = panStart.panY + (ev.clientY - panStart.y)
  }
  const onUp = () => {
    isPanning.value = false
    window.removeEventListener('mousemove', onMove)
    window.removeEventListener('mouseup', onUp)
  }
  window.addEventListener('mousemove', onMove)
  window.addEventListener('mouseup', onUp)
}

// Touch: pinch-to-zoom + drag-to-pan
function handleTouchStart(e) {
  if (e.touches.length === 2) {
    lastTouchDist = Math.hypot(
      e.touches[0].clientX - e.touches[1].clientX,
      e.touches[0].clientY - e.touches[1].clientY
    )
  } else if (e.touches.length === 1 && zoomScale.value > 1) {
    isPanning.value = true
    panStart = { x: e.touches[0].clientX, y: e.touches[0].clientY, panX: panX.value, panY: panY.value }
  }
}

function handleTouchMove(e) {
  if (e.touches.length === 2) {
    const dist = Math.hypot(
      e.touches[0].clientX - e.touches[1].clientX,
      e.touches[0].clientY - e.touches[1].clientY
    )
    if (lastTouchDist > 0) {
      const delta = dist / lastTouchDist
      zoomScale.value = Math.max(1, Math.min(8, zoomScale.value * delta))
      if (zoomScale.value === 1) { panX.value = 0; panY.value = 0 }
    }
    lastTouchDist = dist
  } else if (e.touches.length === 1 && isPanning.value) {
    panX.value = panStart.panX + (e.touches[0].clientX - panStart.x)
    panY.value = panStart.panY + (e.touches[0].clientY - panStart.y)
  }
}

function handleTouchEnd() {
  isPanning.value = false
  lastTouchDist = 0
}

function prevInlineImage() {
  if (inlineImages.value.length <= 1) return
  resetZoom()
  inlineLightboxIndex.value = (inlineLightboxIndex.value - 1 + inlineImages.value.length) % inlineImages.value.length
}

function nextInlineImage() {
  if (inlineImages.value.length <= 1) return
  resetZoom()
  inlineLightboxIndex.value = (inlineLightboxIndex.value + 1) % inlineImages.value.length
}
</script>