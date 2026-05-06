<template>
  <div class="h-screen flex flex-col">
    <!-- Header -->
    <div class="flex items-center space-x-4 px-2 h-12 border-b shrink-0">
      <SidebarTrigger class="cursor-pointer" />
      <span class="text-xl font-semibold">{{ title }}</span>
    </div>

    <!-- Bulk Action Toolbar (when items selected) -->
    <div
      v-if="hasSelection"
      role="toolbar"
      :aria-label="t('conversation.bulkActions.toolbar')"
      class="p-2 flex items-center gap-1 border-b bg-muted/30"
    >
      <Checkbox
        :checked="conversationStore.allSelected"
        @update:checked="toggleSelectAll"
        :aria-label="t('conversation.bulkActions.selectAll')"
        class="ml-1 mr-1"
      />
      <span class="text-xs font-medium whitespace-nowrap mr-1" aria-live="polite">
        {{ t('conversation.bulkActions.selected', conversationStore.selectedCount, { count: conversationStore.selectedCount }) }}
      </span>

      <!-- Assign dropdown -->
      <DropdownMenu v-if="canAssignAgent || canAssignTeam">
        <DropdownMenuTrigger asChild>
          <Button variant="outline" size="sm" class="h-7 text-xs" :disabled="bulkLoading">
            {{ t('conversation.bulkActions.assign') }}
            <ChevronDown class="w-3 h-3 ml-1 opacity-50" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent class="max-h-60 overflow-y-auto">
          <template v-if="canAssignAgent">
            <DropdownMenuLabel class="text-xs text-muted-foreground">
              {{ t('globals.terms.agent', 2) }}
            </DropdownMenuLabel>
            <DropdownMenuItem
              v-for="agent in usersStore.options"
              :key="'agent-' + agent.value"
              @click="bulkAssignAgent(agent.value)"
            >
              {{ agent.label }}
            </DropdownMenuItem>
          </template>
          <DropdownMenuSeparator v-if="canAssignAgent && canAssignTeam" />
          <template v-if="canAssignTeam">
            <DropdownMenuLabel class="text-xs text-muted-foreground">
              {{ t('globals.terms.team', 2) }}
            </DropdownMenuLabel>
            <DropdownMenuItem
              v-for="team in teamsStore.options"
              :key="'team-' + team.value"
              @click="bulkAssignTeam(team.value)"
            >
              {{ team.label }}
            </DropdownMenuItem>
          </template>
        </DropdownMenuContent>
      </DropdownMenu>

      <!-- Status dropdown -->
      <DropdownMenu v-if="canUpdateStatus">
        <DropdownMenuTrigger asChild>
          <Button variant="outline" size="sm" class="h-7 text-xs" :disabled="bulkLoading">
            {{ t('globals.terms.status', 1) }}
            <ChevronDown class="w-3 h-3 ml-1 opacity-50" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent>
          <DropdownMenuItem
            v-for="status in conversationStore.statusOptionsNoSnooze"
            :key="status.value"
            @click="bulkUpdateStatus(status.label)"
          >
            {{ status.label }}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <!-- Priority dropdown -->
      <DropdownMenu v-if="canUpdatePriority">
        <DropdownMenuTrigger asChild>
          <Button variant="outline" size="sm" class="h-7 text-xs" :disabled="bulkLoading">
            {{ t('globals.terms.priority', 1) }}
            <ChevronDown class="w-3 h-3 ml-1 opacity-50" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent>
          <DropdownMenuItem
            v-for="priority in conversationStore.priorityOptions"
            :key="priority.value"
            @click="bulkUpdatePriority(priority.label)"
          >
            {{ priority.label }}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <Button
        v-if="route.params.type !== 'trash' && route.params.type !== 'spam'"
        variant="outline"
        size="sm"
        class="h-7 text-xs"
        :disabled="bulkLoading"
        @click="bulkMoveToTrash"
      >
        <Trash2 class="w-3 h-3 mr-1" />
        {{ t('conversation.trash') }}
      </Button>

      <!-- Permanent delete (FS13): only in the trash view. Opens an AlertDialog
           so the agent has to actively confirm the destructive action, which
           drops the rows out of the database with no recovery path. -->
      <Button
        v-if="route.params.type === 'trash'"
        variant="destructive"
        size="sm"
        class="h-7 text-xs"
        :disabled="bulkLoading"
        @click="deleteConfirmOpen = true"
      >
        <Trash2 class="w-3 h-3 mr-1" />
        {{ t('conversation.bulkActions.deletePermanently') }}
      </Button>

      <Loader2 v-if="bulkLoading" class="w-4 h-4 animate-spin text-muted-foreground ml-2" />

      <Button
        variant="ghost"
        size="sm"
        class="h-7 text-xs ml-auto"
        :aria-label="t('conversation.bulkActions.clearSelection')"
        @click="conversationStore.clearSelection()"
      >
        <X class="w-3 h-3" />
      </Button>
    </div>

    <!-- Filters (hidden when bulk selecting) -->
    <div v-else class="p-2 flex flex-wrap items-center gap-1.5">
      <!-- Status dropdown-menu, hidden when a view is selected as views are pre-filtered -->
      <DropdownMenu v-if="!route.params.viewID">
        <DropdownMenuTrigger asChild>
          <Button variant="ghost" size="sm" class="shrink-0">
            <span class="mr-1">{{ conversationStore.conversations.total }}</span>
            <span>{{ conversationStore.listStatusLabel }}</span>
            <ChevronDown class="w-4 h-4 ml-1 opacity-50" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent>
          <!-- "All" clears every status filter (empty array). Use @select with
               event.preventDefault() so picking an item doesn't close the
               menu — agents typically tick a couple of statuses at once. -->
          <DropdownMenuCheckboxItem
            :checked="conversationStore.conversations.status.length === 0"
            @select="(e) => { e.preventDefault(); conversationStore.setListStatus([]) }"
          >
            {{ $t('globals.messages.all') }}
          </DropdownMenuCheckboxItem>
          <DropdownMenuCheckboxItem
            v-for="status in conversationStore.statusOptions"
            :key="status.value"
            :checked="conversationStore.conversations.status.includes(status.label)"
            :disabled="conversationStore.conversations.status.length === 1
              && conversationStore.conversations.status[0] === status.label"
            @select="(e) => { e.preventDefault(); handleStatusToggle(status) }"
          >
            {{ status.label }}
          </DropdownMenuCheckboxItem>
        </DropdownMenuContent>
      </DropdownMenu>
      <div v-else>
        <Button variant="ghost" size="sm" class="shrink-0">
          <span>{{ conversationStore.conversations.total }}</span>
        </Button>
      </div>

      <!-- Filter pills + add-filter button (inline alongside the status counter) -->
      <FilterBar
        :fields="pillBarFields"
        :model-value="adHocFilters"
        @update:model-value="handleFiltersChange"
      />

      <!-- Right-side controls -->
      <div class="flex items-center gap-1 ml-auto shrink-0">
        <!-- View-mode switcher -->
        <div class="flex border rounded-md p-0.5">
          <Button
            variant="ghost"
            size="sm"
            class="h-7 w-7 p-0"
            :class="viewMode === 'card' ? 'bg-accent text-foreground' : 'text-muted-foreground'"
            :aria-label="t('conversation.list.viewMode.card')"
            :title="t('conversation.list.viewMode.card')"
            :aria-pressed="viewMode === 'card'"
            @click="setViewMode('card')"
          >
            <LayoutList class="w-4 h-4" />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            class="h-7 w-7 p-0"
            :class="viewMode === 'table' ? 'bg-accent text-foreground' : 'text-muted-foreground'"
            :aria-label="t('conversation.list.viewMode.table')"
            :title="t('conversation.list.viewMode.table')"
            :aria-pressed="viewMode === 'table'"
            @click="setViewMode('table')"
          >
            <Table2 class="w-4 h-4" />
          </Button>
        </div>

        <!-- Sort dropdown-menu -->
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" class="w-30">
              {{ conversationStore.getListSortField }}
              <ChevronDown class="w-4 h-4 ml-2 opacity-50" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent>
            <DropdownMenuItem @click="handleSortChange('oldest')">
              {{ $t('conversation.sort.oldestActivity') }}
            </DropdownMenuItem>
            <DropdownMenuItem @click="handleSortChange('newest')">
              {{ $t('conversation.sort.newestActivity') }}
            </DropdownMenuItem>
            <DropdownMenuItem @click="handleSortChange('started_first')">
              {{ $t('conversation.sort.startedFirst') }}
            </DropdownMenuItem>
            <DropdownMenuItem @click="handleSortChange('started_last')">
              {{ $t('conversation.sort.startedLast') }}
            </DropdownMenuItem>
            <DropdownMenuItem @click="handleSortChange('waiting_longest')">
              {{ $t('conversation.sort.waitingLongest') }}
            </DropdownMenuItem>
            <DropdownMenuItem @click="handleSortChange('next_sla_target')">
              {{ $t('conversation.sort.nextSLATarget') }}
            </DropdownMenuItem>
            <DropdownMenuItem @click="handleSortChange('priority_first')">
              {{ $t('conversation.sort.priorityFirst') }}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </div>

    <!-- Pending updates pill -->
    <div
      v-if="conversationStore.conversations.pendingUpdates > 0"
      class="flex justify-center py-1.5 border-b shrink-0"
    >
      <button
        type="button"
        class="inline-flex items-center gap-1.5 px-3 py-1 rounded-full border bg-background text-sm font-medium hover:bg-accent transition-colors shadow-sm"
        @click="conversationStore.applyPendingUpdates()"
      >
        <RefreshCw class="w-3.5 h-3.5 text-primary" />
        {{ t('conversation.list.pendingUpdates', conversationStore.conversations.pendingUpdates, { count: conversationStore.conversations.pendingUpdates }) }}
      </button>
    </div>

    <!-- Content -->
    <div class="flex-grow overflow-y-auto overflow-x-auto">
      <EmptyList
        v-if="!hasConversations && !hasErrored && !isLoading"
        key="empty"
        class="px-4 py-8"
        :title="t('conversation.noConversationsFound')"
        :message="t('conversation.tryAdjustingFilters')"
        :icon="MessageCircleQuestion"
      />

      <!-- Error State -->
      <EmptyList
        v-if="conversationStore.conversations.errorMessage"
        key="error"
        class="px-4 py-8"
        :title="t('conversation.couldNotFetch')"
        :message="conversationStore.conversations.errorMessage"
        :icon="MessageCircleWarning"
      />

      <!-- Conversation list (table view) -->
      <ConversationTableView
        v-if="!conversationStore.conversations.errorMessage && viewMode === 'table' && hasConversations"
        :conversations="conversationStore.conversationsList"
      />

      <!-- Conversation list (card view) -->
      <TransitionGroup
        v-else-if="viewMode === 'card'"
        enter-active-class="transition-all duration-300 ease-in-out"
        enter-from-class="opacity-0 transform translate-y-4"
        enter-to-class="opacity-100 transform translate-y-0"
        leave-active-class="transition-all duration-300 ease-in-out"
        leave-from-class="opacity-100 transform translate-y-0"
        leave-to-class="opacity-0 transform translate-y-4"
      >
        <div
          v-if="!conversationStore.conversations.errorMessage"
          key="list-card"
          class="divide-y divide-border"
          :class="{ 'border-b border-border': hasConversations }"
        >
          <ConversationListItem
            v-for="conversation in conversationStore.conversationsList"
            :key="conversation.uuid"
            :conversation="conversation"
            :currentConversation="conversationStore.current"
            :contactFullName="conversationStore.getContactFullName(conversation.uuid)"
            class="transition-colors duration-200"
          />
        </div>
      </TransitionGroup>

      <!-- Loading Skeleton -->
      <div v-if="isLoading" class="space-y-4">
        <ConversationListItemSkeleton v-for="index in 5" :key="index" />
      </div>

      <!-- Load More -->
      <div
        v-if="!hasErrored && (conversationStore.conversations.hasMore || hasConversations)"
        class="flex justify-center items-center p-5"
      >
        <Button
          v-if="conversationStore.conversations.hasMore"
          variant="outline"
          @click="loadNextPage"
          :disabled="isLoading"
          class="transition-all duration-200 ease-in-out transform hover:scale-105"
        >
          <Loader2 v-if="isLoading" class="mr-2 h-4 w-4 animate-spin" />
          {{ isLoading ? t('globals.terms.loading') : t('globals.terms.loadMore') }}
        </Button>
        <p
          class="text-sm text-gray-500"
          v-else-if="conversationStore.conversationsList.length > 10"
        >
          {{ $t('conversation.allLoaded') }}
        </p>
      </div>
    </div>

    <!-- FS13: confirmation for permanent delete from trash. Standard
         AlertDialog (not window.confirm) per repo convention. -->
    <AlertDialog :open="deleteConfirmOpen" @update:open="deleteConfirmOpen = $event">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{{ t('globals.messages.areYouAbsolutelySure') }}</AlertDialogTitle>
          <AlertDialogDescription>
            {{ t('conversation.bulkActions.deletePermanentlyConfirmation', conversationStore.selectedCount, { count: conversationStore.selectedCount }) }}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{{ t('globals.messages.cancel') }}</AlertDialogCancel>
          <AlertDialogAction @click="bulkDeletePermanently">
            {{ t('conversation.bulkActions.deletePermanently') }}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </div>
</template>

<script setup>
import { computed, ref, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { MessageCircleQuestion, MessageCircleWarning, ChevronDown, Loader2, X, LayoutList, Table2, Trash2, RefreshCw } from 'lucide-vue-next'
import { Button } from '@shared-ui/components/ui/button'
import { Checkbox } from '@shared-ui/components/ui/checkbox'
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger
} from '@shared-ui/components/ui/dropdown-menu'
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
import { SidebarTrigger } from '@shared-ui/components/ui/sidebar'
import { useConversationStore } from '@/stores/conversation'
import { useUsersStore } from '@/stores/users'
import { useTeamStore } from '@/stores/team'
import { useUserStore } from '@/stores/user'
import { useToast } from '@/composables/useToast'
import { useConversationFilters } from '@/composables/useConversationFilters'
import { permissions as p } from '@/constants/permissions'
import api from '@/api'
import { useViewMode } from '@/composables/useViewMode'
import FilterBar from '@/components/filter/FilterBar.vue'
import EmptyList from '@/features/conversation/list/ConversationEmptyList.vue'
import ConversationListItem from '@/features/conversation/list/ConversationListItem.vue'
import ConversationListItemSkeleton from '@/features/conversation/list/ConversationListItemSkeleton.vue'
import ConversationTableView from '@/features/conversation/list/ConversationTableView.vue'

const conversationStore = useConversationStore()
const usersStore = useUsersStore()
const teamsStore = useTeamStore()
const userStore = useUserStore()
const route = useRoute()
const { t } = useI18n()
const toast = useToast()
const { viewMode, setViewMode } = useViewMode()
const bulkLoading = ref(false)
const deleteConfirmOpen = ref(false)
const { conversationsPillBarFields: pillBarFields } = useConversationFilters()
// Local mirror of store-side adHocFilters: FilterBar drives this directly,
// the store debounces the actual fetch. Keeping a local ref means the UI
// stays snappy while the user toggles checkboxes inside a pill popover.
// Seed from the store so persisted/restored filters (per-view persistence)
// show up as pills on initial mount and on view-switch — without this watch
// the pill bar would render empty even when the store has restored values.
const adHocFilters = ref([...(conversationStore.conversations.adHocFilters || [])])
watch(
  () => conversationStore.conversations.adHocFilters,
  (next) => {
    // Skip updates that originate from this component (handleFiltersChange
    // already set adHocFilters.value); only mirror external changes (route
    // switch via InboxView restore). Cheap shallow equality is enough — the
    // store builds a fresh array on every mutation.
    if (next === adHocFilters.value) return
    adHocFilters.value = [...(next || [])]
  },
  { deep: false }
)

function handleFiltersChange (filters) {
  // Dedupe by field key (UI shouldn't allow duplicates but defend in depth)
  // and forward to the store. Store strips empty arrays before they hit the
  // wire, but we also drop them locally so the pill UI doesn't flash an
  // active-but-empty pill while the user is mid-edit.
  const seen = new Map()
  for (const f of filters) {
    seen.set(f.field, f)
  }
  const deduped = Array.from(seen.values())
  adHocFilters.value = deduped
  // `set` / `not set` operators are intentionally valueless (they translate
  // to IS NOT NULL / IS NULL on the backend) so don't strip them out.
  const meaningful = deduped.filter(f =>
    (f.value && f.value !== '[]' && f.value !== '') ||
    f.operator === 'not set' || f.operator === 'set'
  )
  conversationStore.setAdHocFilters(meaningful)
}

const canAssignAgent = computed(() => userStore.can(p.CONVERSATIONS_UPDATE_USER_ASSIGNEE))
const canAssignTeam = computed(() => userStore.can(p.CONVERSATIONS_UPDATE_TEAM_ASSIGNEE))
const canUpdateStatus = computed(() => userStore.can(p.CONVERSATIONS_UPDATE_STATUS))
const canUpdatePriority = computed(() => userStore.can(p.CONVERSATIONS_UPDATE_PRIORITY))

onMounted(() => {
  if (canAssignAgent.value) usersStore.fetchUsers()
  if (canAssignTeam.value) teamsStore.fetchTeams()
})

const hasSelection = computed(() => conversationStore.selectedCount > 0)

const toggleSelectAll = () => {
  if (conversationStore.allSelected) {
    conversationStore.clearSelection()
  } else {
    conversationStore.selectAll()
  }
}

const title = computed(() => {
  const typeKey = route.meta?.typeKey?.(route)
  if (typeKey) {
    return t(typeKey)
  }
  const key = route.meta?.titleKey
  if (!key) return ''
  return t(key, route.meta?.titleCount || 1)
})

const handleStatusToggle = (status) => {
  conversationStore.toggleListStatus(status.label)
}

const handleSortChange = (order) => {
  conversationStore.setListSortField(order)
}

const loadNextPage = () => {
  conversationStore.fetchNextConversations()
}

// Bulk action helpers
const runBulkAction = async (actionFn) => {
  const uuids = [...conversationStore.selectedUUIDs]
  bulkLoading.value = true
  const results = await Promise.allSettled(uuids.map((uuid) => actionFn(uuid)))
  bulkLoading.value = false

  const successCount = results.filter((r) => r.status === 'fulfilled').length
  const errorCount = results.length - successCount

  if (errorCount > 0) {
    const failures = results
      .map((r, i) => ({ uuid: uuids[i], reason: r.reason }))
      .filter((f) => f.reason)
    if (failures.length) {
      console.warn('Bulk action failures:', failures)
    }
  }

  conversationStore.clearSelection()
  conversationStore.fetchFirstPageConversations()

  if (errorCount > 0) {
    toast.error(t('conversation.bulkActions.failedToast', {
      success: successCount,
      failed: errorCount,
      total: uuids.length
    }))
  } else {
    toast.success(t('conversation.bulkActions.successToast', successCount, { count: successCount }))
  }
}

const bulkAssignAgent = (agentId) => {
  runBulkAction((uuid) => api.updateAssignee(uuid, 'user', { assignee_id: parseInt(agentId, 10) }))
}

const bulkAssignTeam = (teamId) => {
  runBulkAction((uuid) => api.updateAssignee(uuid, 'team', { assignee_id: parseInt(teamId, 10) }))
}

const bulkUpdateStatus = (status) => {
  runBulkAction((uuid) => api.updateConversationStatus(uuid, { status }))
}

const bulkUpdatePriority = (priority) => {
  runBulkAction((uuid) => api.updateConversationPriority(uuid, { priority }))
}

const bulkMoveToTrash = () => {
  runBulkAction((uuid) => api.moveToTrash(uuid))
}

// FS13: permanent delete from trash. runBulkAction's default refresh appends
// page-1 results to the existing data array, so deleted UUIDs would linger in
// the in-memory list (processConversationListResponse merges, it doesn't
// prune). Reset the store first so the follow-up fetch repopulates from a
// clean slate. Inlined per FS4 convention; v2 prefers call-site composition
// over a store wrapper helper.
const bulkDeletePermanently = async () => {
  deleteConfirmOpen.value = false
  await runBulkAction((uuid) => api.deleteConversationPermanently(uuid))
  conversationStore.resetConversations()
  await conversationStore.fetchFirstPageConversations()
}

const hasConversations = computed(() => conversationStore.conversationsList.length !== 0)
const hasErrored = computed(() => !!conversationStore.conversations.errorMessage)
const isLoading = computed(() => conversationStore.conversations.loading)
</script>
