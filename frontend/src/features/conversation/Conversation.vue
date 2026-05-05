<template>
  <div class="flex flex-col h-full">
    <!-- Header -->
    <div class="h-12 flex-shrink-0 px-2 border-b flex items-center justify-between">
      <div class="flex items-center gap-2">
        <span v-if="!conversationStore.conversation.loading">
          {{ conversationStore.currentContactName }}
        </span>
        <Skeleton class="w-[130px] h-6" v-else />
        <!-- Presence: other agents viewing -->
        <div v-if="otherViewers.length > 0" class="flex items-center gap-1 ml-2">
          <Eye class="w-3.5 h-3.5 text-blue-500 animate-blink" />
          <TooltipProvider :delay-duration="200">
            <div class="flex -space-x-1.5">
              <Tooltip v-for="viewer in otherViewers.slice(0, 3)" :key="viewer.user_id">
                <TooltipTrigger asChild>
                  <Avatar class="w-5 h-5 rounded-full border border-background cursor-default">
                    <AvatarImage :src="viewer.avatar_url || ''" v-if="viewer.avatar_url" />
                    <AvatarFallback class="text-[8px]">{{ (viewer.first_name || '?').substring(0, 1) }}</AvatarFallback>
                  </Avatar>
                </TooltipTrigger>
                <TooltipContent side="bottom" class="text-xs">
                  {{ viewer.first_name || 'Unknown' }}
                </TooltipContent>
              </Tooltip>
            </div>
          </TooltipProvider>
          <span v-if="otherViewers.length > 3" class="text-[10px] text-muted-foreground">+{{ otherViewers.length - 3 }}</span>
        </div>
      </div>
      <div class="flex items-center gap-1">
        <!-- Quick Close button -->
        <Button
          v-if="!conversationStore.conversation.loading && conversationStore.current?.status !== 'Closed' && conversationStore.current?.status !== 'Trashed'"
          variant="outline"
          size="sm"
          class="h-7 text-xs"
          @click="handleUpdateStatus('Closed')"
        >
          Close
        </Button>

        <DropdownMenu>
          <DropdownMenuTrigger>
            <div
              class="flex items-center space-x-1 cursor-pointer bg-primary px-2 py-1 rounded text-sm"
              v-if="!conversationStore.conversation.loading"
            >
              <span class="text-secondary font-medium inline-block">
                {{ conversationStore.current?.status }}
              </span>
              <ChevronDown class="w-3 h-3 text-secondary" />
            </div>
            <Skeleton class="w-[70px] h-6 rounded-full" v-else />
          </DropdownMenuTrigger>
          <DropdownMenuContent>
            <DropdownMenuItem
              v-for="status in conversationStore.statusOptions"
              :key="status.value"
              @click="handleUpdateStatus(status.label)"
            >
              {{ status.label }}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>

        <!-- More Actions Dropdown -->
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon" class="h-7 w-7">
              <MoreHorizontal class="w-4 h-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem
              v-if="conversationStore.conversation.data?.status !== 'Trashed'" 
              @click="handleMoveToTrash"
            >
              <Trash2 class="w-4 h-4 mr-2" />
              Move to Trash
            </DropdownMenuItem>
            <DropdownMenuItem
              v-if="conversationStore.conversation.data?.status === 'Trashed'" 
              @click="handleRestore"
            >
              <RotateCcw class="w-4 h-4 mr-2" />
              Restore
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              v-if="conversationStore.conversation.data?.status !== 'Spam'" 
              @click="handleMarkAsSpam"
            >
              <ShieldAlert class="w-4 h-4 mr-2" />
              Mark as Spam
            </DropdownMenuItem>
            <DropdownMenuItem
              v-if="conversationStore.conversation.data?.status === 'Spam'" 
              @click="handleMarkAsNotSpam"
            >
              <ShieldCheck class="w-4 h-4 mr-2" />
              Not Spam
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem @click="toggleFollow">
              <EyeOff v-if="isFollowing" class="w-4 h-4 mr-2" />
              <Eye v-else class="w-4 h-4 mr-2" />
              {{ isFollowing ? 'Unfollow' : 'Follow' }}
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              v-if="!conversationStore.current?.merged_into_id"
              @click="showMergeDialog = true"
            >
              <GitMerge class="w-4 h-4 mr-2" />
              Merge
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </div>

    <!-- Merge dialog -->
    <MergeDialog
      v-model:open="showMergeDialog"
      :initialConversation="conversationStore.conversation.data"
    />

    <!-- Merge banner -->
    <div
      v-if="conversationStore.current?.merged_into_id"
      class="flex items-center gap-2 px-4 py-2 bg-blue-50 dark:bg-blue-950/40 text-blue-800 dark:text-blue-200 text-sm border-b border-blue-200 dark:border-blue-800"
    >
      <GitMerge class="w-4 h-4 shrink-0" />
      <span>
        This ticket was merged into
        <router-link
          v-if="conversationStore.current?.merged_into_uuid"
          :to="{ name: 'inbox-conversation', params: { uuid: conversationStore.current.merged_into_uuid } }"
          class="font-semibold underline text-blue-600 dark:text-blue-300 hover:text-blue-900 dark:hover:text-blue-100"
        >#{{ conversationStore.current.merged_into_ref }}</router-link>
      </span>
    </div>

    <!-- Fresh theme: unified scroll with collapsible reply -->
    <template v-if="isFresh">
      <!-- Scrollable area: messages + expanded reply -->
      <div class="flex-1 overflow-y-auto fresh-unified-scroll" ref="scrollContainer">
        <!-- Sticky subject bar -->
        <div
          v-if="conversationStore.current?.subject && !conversationStore.conversation.loading"
          class="sticky top-0 z-10 bg-background/95 backdrop-blur-sm border-b px-4 py-2"
        >
          <div class="group flex items-center gap-2 min-w-0">
            <span class="text-xs font-medium text-muted-foreground shrink-0">#{{ conversationStore.current?.reference_number }}</span>
            <h2 v-if="!editingHeaderSubject" class="text-sm font-semibold truncate">{{ conversationStore.current?.subject }}</h2>
            <input
              v-if="editingHeaderSubject"
              ref="headerSubjectInput"
              v-model="headerSubjectDraft"
              class="flex-1 text-sm font-semibold border rounded px-2 py-0.5 bg-transparent"
              @keyup.enter="saveHeaderSubject"
              @keyup.escape="editingHeaderSubject = false"
            />
            <button
              v-if="!editingHeaderSubject"
              class="opacity-0 group-hover:opacity-100 transition-opacity text-muted-foreground hover:text-foreground shrink-0"
              @click="startEditHeaderSubject"
            >
              <Pencil :size="13" />
            </button>
            <button
              v-if="editingHeaderSubject"
              class="text-muted-foreground hover:text-foreground shrink-0"
              @click="saveHeaderSubject"
            >
              <Check :size="14" />
            </button>
          </div>
        </div>
        <MessageList />
        <!-- Expanded reply box flows inline with messages -->
        <div v-if="replyExpanded" class="border-t">
          <ReplyBox />
        </div>
      </div>

      <!-- Undo send banner -->
      <div
        v-if="!replyExpanded && pendingSend"
        class="flex-shrink-0 border-t"
      >
        <div class="flex items-center justify-between px-4 py-2.5" style="background-color: #fdf0d5; border-bottom: 2px solid #f0c36d;">
          <div class="flex items-center gap-2">
            <CheckCircle2 class="w-4 h-4" style="color: #6f8b2e;" />
            <span class="text-sm font-semibold" style="color: #6f4400;">{{ pendingSend.isPrivateNote ? 'Note added' : pendingSend.isForward ? 'Message forwarded' : 'Reply sent' }}</span>
          </div>
          <Button size="sm" variant="ghost" class="font-bold uppercase tracking-wide" style="color: #1979c3;" @click="undoSend">
            Undo
          </Button>
        </div>
        <div class="h-1" style="background-color: #f0c36d;">
          <div class="h-full transition-all ease-linear" style="background-color: #e07000;" :style="{ width: undoProgress + '%' }" />
        </div>
      </div>

      <!-- Collapsed reply bar: fixed at bottom, outside scroll -->
      <div
        v-if="!replyExpanded && !pendingSend"
        class="flex-shrink-0 bg-background border-t px-4 py-2.5 mb-4 flex gap-2"
      >
        <Button size="sm" variant="outline" @click="expandReply('reply')">
          <Reply class="h-4 w-4 mr-1.5" />
          Reply
        </Button>
        <Button size="sm" variant="outline" @click="expandReply('private_note')">
          <StickyNote class="h-4 w-4 mr-1.5" />
          Private note
        </Button>
      </div>
    </template>

    <!-- Default theme: original layout with sticky reply -->
    <template v-else>
      <div class="flex flex-col flex-grow overflow-hidden">
        <MessageList class="flex-1 overflow-y-auto" />
        <div class="sticky bottom-0">
          <ReplyBox />
        </div>
      </div>
    </template>
  </div>
</template>

<script setup>
import { ref, computed, nextTick, watch, onMounted, onBeforeUnmount, provide } from 'vue'
import { useConversationStore } from '@/stores/conversation'
import { useMacroStore } from '@/stores/macro'
import { usePresenceStore } from '@/stores/presence'
import { useTheme } from '@/composables/useTheme'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger
} from '@/components/ui/dropdown-menu'
import { Button } from '@/components/ui/button'
import MessageList from '@/features/conversation/message/MessageList.vue'
import ReplyBox from "./ReplyBox.vue"
import MergeDialog from "./MergeDialog.vue"
import { Reply, StickyNote, Forward, MoreHorizontal, Trash2, RotateCcw, ShieldAlert, ShieldCheck, ChevronDown, GitMerge, Eye, EyeOff, CheckCircle2, Pencil, Check } from 'lucide-vue-next'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import { sendMessage as wsSendMessage } from '@/websocket'
import { EMITTER_EVENTS } from '@/constants/emitterEvents.js'
import { CONVERSATION_DEFAULT_STATUSES } from '@/constants/conversation'
import { useEmitter } from '@/composables/useEmitter'
import { Skeleton } from '@/components/ui/skeleton'
import { useRouter } from 'vue-router'
import api from '@/api'
import { handleHTTPError } from '@/utils/http'

import { useUserStore } from '@/stores/user'

const conversationStore = useConversationStore()
const macroStore = useMacroStore()
const presenceStore = usePresenceStore()
const userStore = useUserStore()
const emitter = useEmitter()
const router = useRouter()
const { currentTheme } = useTheme()

// Follow/unfollow state
const isFollowing = ref(false)

const checkFollowStatus = async () => {
  const uuid = conversationStore.current?.uuid
  if (!uuid) return
  try {
    const res = await api.getConversationParticipants(uuid)
    const participants = res.data?.data || []
    isFollowing.value = participants.some(p => p.id === userStore.userID)
  } catch { /* ignore */ }
}

const toggleFollow = async () => {
  const uuid = conversationStore.current?.uuid
  if (!uuid) return
  try {
    if (isFollowing.value) {
      await api.unfollowConversation(uuid)
      isFollowing.value = false
    } else {
      await api.followConversation(uuid)
      isFollowing.value = true
    }
  } catch (err) {
    handleHTTPError(err)
  }
}


// Header subject inline edit
const editingHeaderSubject = ref(false)
const headerSubjectDraft = ref('')
const headerSubjectInput = ref(null)

const startEditHeaderSubject = () => {
  headerSubjectDraft.value = conversationStore.current?.subject || ''
  editingHeaderSubject.value = true
  nextTick(() => headerSubjectInput.value?.focus())
}

const saveHeaderSubject = async () => {
  const trimmed = headerSubjectDraft.value.trim()
  if (!trimmed || trimmed === conversationStore.current?.subject) {
    editingHeaderSubject.value = false
    return
  }
  try {
    await api.updateConversationSubject(conversationStore.current.uuid, trimmed)
    conversationStore.current.subject = trimmed
    editingHeaderSubject.value = false
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, { description: 'Subject updated' })
  } catch (error) {
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
      variant: 'destructive',
      description: handleHTTPError(error).message
    })
  }
}
// Presence tracking
const currentViewingUUID = ref('')

const otherViewers = computed(() => {
  const uuid = conversationStore.current?.uuid
  if (!uuid) return []
  return presenceStore.getViewers(uuid, userStore.userID)
})

const pendingSend = ref(null)
const undoProgress = ref(100)
let undoTimer = null
let undoProgressInterval = null
const UNDO_DELAY_MS = 5000

async function executePendingSend() {
  clearTimeout(undoTimer)
  clearInterval(undoProgressInterval)
  const send = pendingSend.value
  if (!send) return
  pendingSend.value = null

  try {
    await api.sendMessage(send.uuid, send.payload)

    // Apply macro if any
    if (send.macroID && send.macroActions.length > 0) {
      try {
        await api.applyMacro(send.uuid, send.macroID, send.macroActions)
        macroStore.markUsedLocal(send.macroID)
      } catch (_) { /* macro errors are non-fatal */ }
    }

    // Update status if "Send and Close" etc.
    if (send.statusAfterSend) {
      try {
        await api.updateConversationStatus(send.uuid, { status: send.statusAfterSend })
      } catch (_) { /* status update errors shown by WS */ }
    }
  } catch (error) {
    // 409 = duplicate rejected by server dedup — first send succeeded, ignore
    if (error?.response?.status === 409) return

    const errMsg = handleHTTPError(error).message

    // Show persistent error toast
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
      variant: 'destructive',
      description: 'Message failed to send: ' + errMsg
    })

    // Restore the editor content so the user can retry
    if (send.restoreData) {
      replyExpanded.value = true
      nextTick(() => {
        emitter.emit('restore-send', send.restoreData)
        if (send.isPrivateNote) {
          emitter.emit('set-reply-type', 'private_note')
        } else if (send.isForward) {
          emitter.emit('set-reply-type', 'forward')
        } else {
          emitter.emit('set-reply-type', 'reply')
        }
      })
    }
  }
}

function sendViewingPresence(uuid) {
  if (currentViewingUUID.value === uuid) return
  currentViewingUUID.value = uuid
  wsSendMessage({ type: 'view_conversation', data: { conversation_uuid: uuid || '' } })
}

// Watch for conversation changes and send presence
watch(
  () => conversationStore.current?.uuid,
  (newUUID, oldUUID) => {
    // Execute pending send if switching conversations
    if (oldUUID && pendingSend.value) {
      executePendingSend()
    }
    if (newUUID) {
      sendViewingPresence(newUUID)
      checkFollowStatus()
    }
  },
  { immediate: true }
)

onBeforeUnmount(() => {
  // Clear presence when leaving
  sendViewingPresence('')
  // Execute pending send if navigating away
  if (pendingSend.value) {
    executePendingSend()
  }
})

const showMergeDialog = ref(false)


emitter.on('send-queued', (data) => {
  // Cancel any previous pending send
  if (pendingSend.value) {
    executePendingSend()
  }

  pendingSend.value = data
  undoProgress.value = 100

  // Animate progress bar
  const startTime = Date.now()
  undoProgressInterval = setInterval(() => {
    const elapsed = Date.now() - startTime
    undoProgress.value = Math.max(0, 100 - (elapsed / UNDO_DELAY_MS) * 100)
  }, 50)

  // Execute send after delay
  undoTimer = setTimeout(() => {
    executePendingSend()
  }, UNDO_DELAY_MS)
})

function undoSend() {
  clearTimeout(undoTimer)
  clearInterval(undoProgressInterval)
  const send = pendingSend.value
  pendingSend.value = null

  if (send) {
    // Re-expand editor and restore content
    replyExpanded.value = true
    nextTick(() => {
      emitter.emit('restore-send', send.restoreData)
      if (send.isPrivateNote) {
        emitter.emit('set-reply-type', 'private_note')
      } else {
        emitter.emit('set-reply-type', 'reply')
      }
    })
  }
}

const isFresh = computed(() => currentTheme.value === 'fresh')
const replyExpanded = ref(false)
const scrollContainer = ref(null)
provide('scrollContainer', scrollContainer)

// Listen for collapse-reply from ReplyBox (e.g. after discarding draft)
emitter.on('collapse-reply', () => {
  replyExpanded.value = false
})

// Escape key: collapse reply box (with discard confirmation if draft exists)
emitter.on('shortcut-escape', () => {
  if (!conversationStore.current?.uuid) return
  if (isFresh.value && replyExpanded.value) {
    emitter.emit('shortcut-discard-or-collapse')
  }
})

// Global keyboard shortcuts for reply/note
emitter.on('forward-message', async (messageData) => {
  if (!conversationStore.current?.uuid) return
  if (isFresh.value) {
    replyExpanded.value = true
    await nextTick()
    // Give ReplyBox time to mount and register its listener
    setTimeout(() => {
      emitter.emit('set-reply-type', 'forward')
      emitter.emit('populate-forward', messageData)
    }, 150)
  } else {
    emitter.emit('set-reply-type', 'forward')
    emitter.emit('populate-forward', messageData)
  }
})

emitter.on('shortcut-reply', () => {
  if (!conversationStore.current?.uuid) return
  if (isFresh.value) {
    expandReply('reply')
  } else {
    emitter.emit('set-reply-type', 'reply')
  }
})
emitter.on('shortcut-note', () => {
  if (!conversationStore.current?.uuid) return
  if (isFresh.value) {
    expandReply('private_note')
  } else {
    emitter.emit('set-reply-type', 'private_note')
  }
})

const expandReply = async (type) => {
  replyExpanded.value = true
  await nextTick()
  if (type) emitter.emit('set-reply-type', type)
  // Scroll to bottom so the full reply editor (including action icons) is visible
  if (scrollContainer.value) {
    // Use setTimeout to allow ReplyBox to fully render before scrolling
    setTimeout(() => {
      scrollContainer.value.scrollTop = scrollContainer.value.scrollHeight
    }, 100)
  }
}


const handleMoveToTrash = async () => {
  try {
    await api.moveToTrash(conversationStore.conversation.data.uuid)
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, { description: 'Moved to trash' })
    await conversationStore.fetchFirstPageConversations()
    router.push({ name: 'inbox', params: { type: 'assigned' } })
  } catch (error) {
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, { variant: 'destructive', description: handleHTTPError(error).message })
  }
}

const handleRestore = async () => {
  try {
    await api.restoreFromTrash(conversationStore.conversation.data.uuid)
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, { description: 'Conversation restored' })
    await conversationStore.fetchFirstPageConversations()
    router.push({ name: 'inbox', params: { type: 'assigned' } })
  } catch (error) {
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, { variant: 'destructive', description: handleHTTPError(error).message })
  }
}

const handleMarkAsSpam = async () => {
  try {
    await api.markAsSpam(conversationStore.conversation.data.uuid)
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, { description: 'Marked as spam' })
    await conversationStore.fetchFirstPageConversations()
    router.push({ name: 'inbox', params: { type: 'assigned' } })
  } catch (error) {
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, { variant: 'destructive', description: handleHTTPError(error).message })
  }
}

const handleMarkAsNotSpam = async () => {
  try {
    await api.markAsNotSpam(conversationStore.conversation.data.uuid)
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, { description: 'Moved to inbox' })
    await conversationStore.fetchFirstPageConversations()
    router.push({ name: 'inbox', params: { type: 'assigned' } })
  } catch (error) {
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, { variant: 'destructive', description: handleHTTPError(error).message })
  }
}

const handleUpdateStatus = (status) => {
  if (status === CONVERSATION_DEFAULT_STATUSES.SNOOZED) {
    emitter.emit(EMITTER_EVENTS.SET_NESTED_COMMAND, {
      command: 'snooze',
      open: true
    })
    return
  }
  conversationStore.updateStatus(status)
}
</script>

<style scoped>
@keyframes blink {
  0%, 90%, 100% { transform: scaleY(1); }
  95% { transform: scaleY(0.1); }
}
.animate-blink {
  animation: blink 3s ease-in-out infinite;
  transform-origin: center;
}
</style>
