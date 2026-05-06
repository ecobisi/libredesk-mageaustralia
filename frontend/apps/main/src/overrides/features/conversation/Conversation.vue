<template>
  <div class="flex flex-col h-full">
    <!-- Header -->
    <div class="h-12 flex-shrink-0 px-2 border-b flex items-center justify-between">
      <div class="flex items-center gap-2">
        <span v-if="!conversationStore.conversation.loading">
          {{ conversationStore.currentContactName }}
        </span>
        <Skeleton class="w-[130px] h-6" v-else />
        <!-- Presence: other agents currently viewing this conversation -->
        <div v-if="otherViewers.length > 0" class="flex items-center gap-1 ml-2">
          <Eye class="w-3.5 h-3.5 text-blue-500 animate-blink" />
          <TooltipProvider :delay-duration="200">
            <div class="flex -space-x-1.5">
              <Tooltip v-for="viewer in otherViewers.slice(0, 3)" :key="viewer.user_id">
                <TooltipTrigger asChild>
                  <span
                    class="inline-flex items-center justify-center w-5 h-5 rounded-full border border-background bg-muted text-[8px] font-medium cursor-default"
                    :title="viewer.first_name || 'Agent'"
                  >
                    {{ (viewer.first_name || '?').substring(0, 1) }}
                  </span>
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
      <div class="flex items-center">
        <DropdownMenu>
          <DropdownMenuTrigger>
            <div
              class="flex items-center space-x-1 cursor-pointer bg-primary px-2 py-1 rounded text-sm"
              v-if="!conversationStore.conversation.loading"
            >
              <span class="text-secondary font-medium inline-block">
                {{ conversationStore.current?.status }}
              </span>
              <!-- Hint that this pill is a dropdown, not a static label.
                   Without the chevron, agents repeatedly miss that they
                   can change status by clicking it. -->
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

        <DropdownMenu v-if="!conversationStore.conversation.loading">
          <DropdownMenuTrigger as-child>
            <button
              class="flex items-center justify-center cursor-pointer hover:bg-muted rounded-md h-7 w-7 ml-2"
              :title="t('globals.terms.actions', 2)"
            >
              <MoreHorizontal class="w-4 h-4" />
            </button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem
              v-if="conversationStore.current?.status !== 'Trashed'"
              @click="handleMoveToTrash"
            >
              <Trash2 class="w-4 h-4 mr-2" />
              {{ t('conversation.moveToTrash') }}
            </DropdownMenuItem>
            <DropdownMenuItem
              v-if="conversationStore.current?.status === 'Trashed'"
              @click="handleRestore"
            >
              <RotateCcw class="w-4 h-4 mr-2" />
              {{ t('conversation.restoreFromTrash') }}
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              v-if="conversationStore.current?.status !== 'Spam'"
              @click="handleMarkAsSpam"
            >
              <ShieldAlert class="w-4 h-4 mr-2" />
              {{ t('conversation.markAsSpam') }}
            </DropdownMenuItem>
            <DropdownMenuItem
              v-if="conversationStore.current?.status === 'Spam'"
              @click="handleMarkAsNotSpam"
            >
              <ShieldCheck class="w-4 h-4 mr-2" />
              {{ t('conversation.markAsNotSpam') }}
            </DropdownMenuItem>
            <DropdownMenuSeparator
              v-if="!conversationStore.current?.merged_into_id"
            />
            <DropdownMenuItem
              v-if="!conversationStore.current?.merged_into_id"
              @click="showMergeDialog = true"
            >
              <GitMerge class="w-4 h-4 mr-2" />
              {{ t('conversation.merge.action') }}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </div>

    <!-- Merge dialog (modal — outside scroll container) -->
    <MergeDialog
      v-model:open="showMergeDialog"
      :initial-conversation="conversationStore.conversation.data"
      @merged="handleMerged"
    />

    <!--
      OVERRIDE: Unified scroll container.
      Subject bar, merge banner, MessageList, undo banner, and ReplyBox all
      flow together through a single overflow-y-auto element. As the editor
      grows with content (e.g. pasting a large image) the page scrolls — the
      v1.0.3 behaviour — instead of confining the editor to a 600px window.
      MessageList injects `unifiedScrollEl` (via `provide` below) and routes
      its scroll math through it; ReplyBoxContent drops its max-h cap.
    -->
    <div
      ref="unifiedScrollEl"
      class="flex-1 overflow-y-auto"
    >
      <!--
        EC4 / EC5: Subject bar — sticky inside the unified scroll container so
        it stays visible while the agent scrolls through messages or the editor.
      -->
      <div
        v-if="!conversationStore.conversation.loading && (conversationStore.current?.subject || conversationStore.current?.uuid)"
        class="sticky top-0 z-10 border-b bg-background px-4 py-2"
      >
        <div class="group flex items-center gap-2 min-w-0">
        <span
          v-if="conversationStore.current?.reference_number"
          class="text-xs font-medium text-muted-foreground shrink-0"
        >#{{ conversationStore.current.reference_number }}</span>
        <h2
          v-if="!editingHeaderSubject"
          class="text-sm font-semibold truncate"
          :class="{ 'italic text-muted-foreground': !conversationStore.current?.subject }"
          :title="conversationStore.current?.subject || t('conversation.list.noSubject')"
        >{{ conversationStore.current?.subject || t('conversation.list.noSubject') }}</h2>
        <input
          v-else
          ref="headerSubjectInput"
          v-model="headerSubjectDraft"
          :maxlength="MAX_SUBJECT_LEN"
          :aria-label="t('conversation.subject.editAria')"
          :disabled="savingHeaderSubject"
          class="flex-1 min-w-0 text-sm font-semibold border rounded px-2 py-0.5 bg-transparent disabled:opacity-50"
          @keyup.enter="saveHeaderSubject"
          @keyup.escape="cancelEditHeaderSubject"
          @blur="saveHeaderSubject"
        />
        <button
          v-if="!editingHeaderSubject"
          type="button"
          :title="t('conversation.subject.editTitle')"
          :aria-label="t('conversation.subject.editTitle')"
          class="opacity-0 group-hover:opacity-100 focus:opacity-100 transition-opacity text-muted-foreground hover:text-foreground shrink-0"
          @click="startEditHeaderSubject"
        >
          <Pencil :size="13" />
        </button>
        <button
          v-else
          type="button"
          :title="t('conversation.subject.save')"
          :aria-label="t('conversation.subject.save')"
          :disabled="savingHeaderSubject"
          class="text-muted-foreground hover:text-foreground shrink-0 disabled:opacity-50"
          @mousedown.prevent="saveHeaderSubject"
        >
          <Check :size="14" />
        </button>
      </div>
    </div>

      <!-- Merge banner -->
      <div
        v-if="conversationStore.current?.merged_into_id"
        class="flex items-center gap-2 px-4 py-2 bg-blue-50 dark:bg-blue-950/40 text-blue-800 dark:text-blue-200 text-sm border-b border-blue-200 dark:border-blue-800"
      >
        <GitMerge class="w-4 h-4 shrink-0" />
        <span>
          {{ t('conversation.merge.mergedIntoBanner') }}
          <router-link
            v-if="conversationStore.current?.merged_into_uuid"
            :to="mergedIntoLink"
            class="font-semibold underline text-blue-600 dark:text-blue-300 hover:text-blue-900 dark:hover:text-blue-100"
          >#{{ conversationStore.current.merged_into_ref }}</router-link>
        </span>
      </div>

      <!-- Messages: no internal scroll; flows in unified container -->
      <MessageList />

      <!--
        EC3: Undo-send banner. Sits between MessageList and ReplyBox while
        a queued send is counting down. The pending message itself is
        already rendered in MessageList via addPendingMessage, so the
        banner is purely for the Undo affordance + countdown progress.
      -->
      <div
        v-if="pendingSend"
        role="status"
        aria-live="polite"
        class="border-t bg-amber-50 dark:bg-amber-950/40 border-amber-300 dark:border-amber-800"
      >
        <div class="flex items-center justify-between px-4 py-2">
          <div class="flex items-center gap-2 text-amber-900 dark:text-amber-200 text-sm">
            <CheckCircle2 class="w-4 h-4" />
            <span class="font-medium">
              {{ pendingSend.isPrivate ? t('replyBox.undo.noteAdded') : t('replyBox.undo.replySent') }}
            </span>
          </div>
          <Button
            size="sm"
            variant="ghost"
            class="font-semibold uppercase tracking-wide text-blue-600 dark:text-blue-300 hover:text-blue-800 dark:hover:text-blue-100"
            @click="undoSend"
          >
            {{ t('replyBox.undo.action') }}
          </Button>
        </div>
        <div class="h-1 bg-amber-200 dark:bg-amber-800">
          <div
            class="h-full bg-amber-500 transition-all ease-linear"
            :style="{ width: undoProgress + '%' }"
          />
        </div>
      </div>

      <ReplyBox />
    </div>
  </div>
</template>

<script setup>
import { computed, provide, ref, watch, onMounted, onBeforeUnmount, nextTick } from 'vue'
// OVERRIDE: imports use absolute @/ paths so they resolve against
// apps/main/src (and flow through the override resolver where applicable),
// not against this file's own overrides/ subtree.
import { useConversationStore } from '@/stores/conversation'
import { useMacroStore } from '@/stores/macro'
import { usePresenceStore } from '@/stores/presence'
import { useUserStore } from '@/stores/user'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger
} from '@shared-ui/components/ui/dropdown-menu'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@shared-ui/components/ui/tooltip'
import { Button } from '@shared-ui/components/ui/button'
import MessageList from '@/features/conversation/message/MessageList.vue'
import ReplyBox from '@/features/conversation/ReplyBox.vue'
import MergeDialog from '@/features/conversation/MergeDialog.vue'
import { EMITTER_EVENTS } from '@/constants/emitterEvents.js'
import { CONVERSATION_DEFAULT_STATUSES } from '@/constants/conversation'
import { useEmitter } from '@/composables/useEmitter'
import { useToast } from '@/composables/useToast'
import { Skeleton } from '@shared-ui/components/ui/skeleton'
import { MoreHorizontal, Trash2, RotateCcw, ShieldAlert, ShieldCheck, Eye, GitMerge, ChevronDown, CheckCircle2, Pencil, Check } from 'lucide-vue-next'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import api from '@/api'
import { handleHTTPError } from '@shared-ui/utils/http.js'
import { sendViewConversation } from '@/websocket'

const conversationStore = useConversationStore()
const macroStore = useMacroStore()
const presenceStore = usePresenceStore()
const userStore = useUserStore()
const emitter = useEmitter()
const toast = useToast()
const route = useRoute()
const router = useRouter()
const { t } = useI18n()

const showMergeDialog = ref(false)

// Unified scroll container — provided to MessageList so it can route its
// scroll math (isAtBottom, scrollToBottom, scrollToMessage) through the
// outer container instead of its own internal overflow-y-auto.
const unifiedScrollEl = ref(null)
provide('unifiedScrollEl', unifiedScrollEl)

// ---------------------------------------------------------------------------
// EC4 / EC5: Sticky subject header + inline edit
// ---------------------------------------------------------------------------
// Backend caps subject at 500 chars (see UpdateConversationSubject in
// internal/conversation/conversation.go). Mirror the cap client-side via
// maxlength so the input physically can't exceed it — saves the round-trip
// on egregious paste.
const MAX_SUBJECT_LEN = 500
const editingHeaderSubject = ref(false)
const headerSubjectDraft = ref('')
const headerSubjectInput = ref(null)
// In-flight guard: blur fires after Enter, which would double-submit. Also
// guards against the user clicking the check button while the PUT is still
// pending and producing two parallel requests.
const savingHeaderSubject = ref(false)

const startEditHeaderSubject = () => {
  headerSubjectDraft.value = conversationStore.current?.subject || ''
  editingHeaderSubject.value = true
  // nextTick so the input exists in the DOM before we focus + select.
  nextTick(() => {
    headerSubjectInput.value?.focus()
    headerSubjectInput.value?.select?.()
  })
}

const cancelEditHeaderSubject = () => {
  editingHeaderSubject.value = false
  headerSubjectDraft.value = ''
}

// If the agent switches conversations mid-edit, drop the draft. Otherwise
// `saveHeaderSubject` would PUT against the new conversation's UUID using
// the old conversation's draft — silently rewriting the wrong subject.
watch(
  () => conversationStore.current?.uuid,
  () => {
    if (editingHeaderSubject.value) {
      cancelEditHeaderSubject()
    }
  }
)

const saveHeaderSubject = async () => {
  // Guard against the blur+enter double-fire and rapid double-clicks.
  if (!editingHeaderSubject.value || savingHeaderSubject.value) return
  const trimmed = headerSubjectDraft.value.trim()
  const current = conversationStore.current?.subject || ''
  // No-op cases: empty input or unchanged value. Treat empty as a cancel
  // rather than a delete — the backend rejects empty anyway, and prompting
  // the agent for "did you mean to clear?" is more friction than we want
  // for the click-outside-saves UX.
  if (!trimmed || trimmed === current) {
    cancelEditHeaderSubject()
    return
  }
  const uuid = conversationStore.current?.uuid
  if (!uuid) {
    cancelEditHeaderSubject()
    return
  }
  savingHeaderSubject.value = true
  try {
    await api.updateConversationSubject(uuid, trimmed)
    // Optimistically update local state — the backend also broadcasts via
    // BroadcastConversationUpdate, but the WS round-trip is async and we
    // want the field to reflect the new value before exiting edit mode so
    // the agent sees their change immediately.
    if (conversationStore.conversation.data?.uuid === uuid) {
      conversationStore.conversation.data.subject = trimmed
    }
    editingHeaderSubject.value = false
    headerSubjectDraft.value = ''
    toast.success(t('conversation.subject.updated'))
  } catch (error) {
    toast.error(error)
  } finally {
    savingHeaderSubject.value = false
  }
}

// Build a link to the primary the current secondary was merged into. Reuse the
// current route's `type` (assigned/unassigned/etc) so the navigation lands the
// agent in the same inbox view.
const mergedIntoLink = computed(() => ({
  name: 'inbox-conversation',
  params: {
    uuid: conversationStore.current?.merged_into_uuid,
    type: route.params.type || 'assigned'
  }
}))

function handleMerged ({ primary_uuid }) {
  // Navigate to the primary so the agent immediately sees the unified thread.
  if (!primary_uuid) return
  router.push({
    name: 'inbox-conversation',
    params: { uuid: primary_uuid, type: route.params.type || 'assigned' }
  })
}

// Presence tracking
const otherViewers = computed(() => {
  const uuid = conversationStore.current?.uuid
  if (!uuid) return []
  return presenceStore.getViewers(uuid, userStore.userID)
})

// Send presence when conversation changes
watch(
  () => conversationStore.current?.uuid,
  (newUUID, oldUUID) => {
    // EC18: If a queued send is still counting down when the agent switches
    // conversations, fire it now so the message lands in the conversation
    // it was composed for. Without this guard, the timer keeps running and
    // the POST eventually fires while we're already viewing a different
    // conversation — opens the door to confusing UI states (banner from a
    // conv we no longer have open) and, if the agent then sends another
    // reply on the new conv, two queued sends racing.
    if (oldUUID && pendingSend.value && pendingSend.value.uuid === oldUUID) {
      executePendingSend()
    }
    if (newUUID) {
      sendViewConversation(newUUID)
    } else if (oldUUID) {
      sendViewConversation('')
    }
  },
  { immediate: true }
)

onBeforeUnmount(() => {
  sendViewConversation('')
  // EC18 (sibling case): if the user closes the tab / navigates away while a
  // send is queued, flush it to the server. Otherwise the optimistic message
  // sits in cache and the customer never gets the reply.
  // Fire-and-forget by design: we can't await executePendingSend (Vue tears
  // down the component immediately after this hook returns) and we don't
  // need to — the api.sendMessage call has already been issued, and the
  // emitter.off below unregisters handleSendQueued. If executePendingSend
  // catches and tries to emit RESTORE_SEND after teardown, there's no
  // listener so it's a safe no-op.
  if (pendingSend.value) {
    executePendingSend()
  }
  emitter.off(EMITTER_EVENTS.SEND_QUEUED, handleSendQueued)
})

// ---------------------------------------------------------------------------
// EC3: Undo-send queue
// ---------------------------------------------------------------------------
// ReplyBox emits SEND_QUEUED with the prepared payload + restore data; we
// hold a 5s timer here. If the agent clicks Undo before it fires, we cancel,
// remove the optimistic pending message, and emit RESTORE_SEND back into
// ReplyBox to repopulate the editor. If they switch conversations we flush
// (EC18) so a stale send can't fire to the wrong customer.
const UNDO_DELAY_MS = 5000
const PROGRESS_TICK_MS = 50
const pendingSend = ref(null)
const undoProgress = ref(100)
let undoTimer = null
let undoProgressInterval = null

function handleSendQueued (data) {
  // If a previous send is still queued (rapid-fire Send clicks), flush it
  // first so we don't lose it. Synchronously firing it with no delay is
  // safe — the agent already chose to send it 5s ago.
  if (pendingSend.value) {
    executePendingSend()
  }

  pendingSend.value = data
  undoProgress.value = 100

  const startTime = Date.now()
  undoProgressInterval = setInterval(() => {
    const elapsed = Date.now() - startTime
    undoProgress.value = Math.max(0, 100 - (elapsed / UNDO_DELAY_MS) * 100)
  }, PROGRESS_TICK_MS)

  undoTimer = setTimeout(() => {
    executePendingSend()
  }, UNDO_DELAY_MS)
}

onMounted(() => {
  emitter.on(EMITTER_EVENTS.SEND_QUEUED, handleSendQueued)
})

async function executePendingSend () {
  clearTimeout(undoTimer)
  clearInterval(undoProgressInterval)
  undoTimer = null
  undoProgressInterval = null
  const send = pendingSend.value
  if (!send) return
  pendingSend.value = null

  try {
    const response = await api.sendMessage(send.uuid, send.payload)

    // Private notes don't echo over WS, so swap the optimistic pending
    // message for the real one immediately. Public replies wait for the WS
    // echo (matched by echo_id == tempUUID) to do the swap.
    if (send.isPrivate && response?.data?.data) {
      conversationStore.replacePendingMessage(send.uuid, send.tempUUID, response.data.data)
    }

    // EC1: surface non-fatal status-transition failure. The reply itself
    // landed; the post-send status change (Send & Resolve / Send & Close)
    // didn't apply. Tell the agent so they don't think they resolved a
    // conversation that's still open.
    const setStatusError = response?.data?.data?.set_status_error
    if (setStatusError && send.setStatus) {
      toast.error(t('replyBox.sentButSetStatusFailed', { status: send.setStatus }))
    }

    // Apply macro actions if any. Non-fatal — surface failure but don't
    // re-throw, the reply itself succeeded.
    if (send.macroID > 0 && send.macroActions?.length > 0) {
      try {
        await api.applyMacro(send.uuid, send.macroID, send.macroActions)
        // Optimistic MRU reorder so the picker reflects this apply
        // immediately, ahead of the next /api/v1/macros refetch (MP6).
        macroStore.markUsedLocal(send.macroID)
      } catch (error) {
        toast.error(error)
      }
    }
  } catch (error) {
    // EC2: Restore the editor content + recipients on failure so the agent
    // doesn't lose their work. Only restore if (a) we're still on the same
    // conversation — otherwise we'd hijack the agent's current draft on a
    // different conversation — AND (b) no NEWER send has been queued since
    // this one started. The latter guards against rapid-fire sends: while
    // we were awaiting api.sendMessage above, handleSendQueued may have
    // assigned a new pendingSend.value for a fresh compose, and a stomp of
    // RESTORE_SEND would clobber the agent's in-flight new draft. The
    // pending message is removed in either case so the failed send doesn't
    // visually persist in the thread.
    conversationStore.removePendingMessage(send.uuid, send.tempUUID)
    if (
      conversationStore.current?.uuid === send.uuid &&
      pendingSend.value === null
    ) {
      emitter.emit(EMITTER_EVENTS.RESTORE_SEND, send.restoreData)
    }
    toast.error(t('replyBox.messageFailedToSend', {
      error: handleHTTPError(error).message
    }))
  }
}

function undoSend () {
  clearTimeout(undoTimer)
  clearInterval(undoProgressInterval)
  undoTimer = null
  undoProgressInterval = null
  const send = pendingSend.value
  pendingSend.value = null
  if (!send) return

  // Pull the optimistic message back out of the thread.
  conversationStore.removePendingMessage(send.uuid, send.tempUUID)

  // Restore editor content only if we're still on the conversation the send
  // was queued from. Edge case: agent switches conversations during the 5s
  // window — EC18's flush should fire the send first, but defend against
  // any future change to that policy.
  if (conversationStore.current?.uuid === send.uuid) {
    nextTick(() => {
      emitter.emit(EMITTER_EVENTS.RESTORE_SEND, send.restoreData)
    })
  }
}

const runConversationAction = async (action, successMsg) => {
  const uuid = conversationStore.conversation.data?.uuid
  if (!uuid) return
  try {
    await action(uuid)
    toast.success(successMsg)
    // Step back so the user lands on whatever list they came from (e.g. Trash
    // when restoring from trash, custom view when marking as spam) instead of
    // forcing them back to the assigned inbox.
    if (window.history.length > 1) router.back()
    else router.push({ name: 'inbox', params: { type: 'assigned' } })
  } catch (error) {
    toast.error(error)
  }
}

const handleMoveToTrash = () => runConversationAction(api.moveToTrash, t('conversation.moveToTrash'))
const handleRestore = () => runConversationAction(api.restoreFromTrash, t('conversation.restoreFromTrash'))
const handleMarkAsSpam = () => runConversationAction(api.markAsSpam, t('conversation.markAsSpam'))
const handleMarkAsNotSpam = () => runConversationAction(api.markAsNotSpam, t('conversation.markAsNotSpam'))

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
