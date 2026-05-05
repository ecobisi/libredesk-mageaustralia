<template>
  <ConversationPlaceholder v-if="['inbox', 'team-inbox', 'view-inbox'].includes(route.name)" />
  <router-view />
</template>

<script setup>
import { computed, watch, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { useConversationStore } from '../../stores/conversation'
import { CONVERSATION_LIST_TYPE, CONVERSATION_DEFAULT_STATUSES } from '../../constants/conversation'
import ConversationPlaceholder from '@/features/conversation/ConversationPlaceholder.vue'

const route = useRoute()
const type = computed(() => route.params.type)
const teamID = computed(() => route.params.teamID)
const viewID = computed(() => route.params.viewID)

const conversationStore = useConversationStore()

// Spam and trash views ignore status filtering — they show every conversation
// in their bucket regardless of Open/Snoozed/Resolved/Closed.
const isStatusUnfilteredView = (t) => t === CONVERSATION_LIST_TYPE.SPAM || t === CONVERSATION_LIST_TYPE.TRASH

/**
 * Apply filter state for the view we're switching INTO. The flow is:
 *
 *   1. Persist whatever's currently in the store under the OUTGOING view key.
 *   2. Either restore the new view's saved state from localStorage, or fall
 *      back to sane defaults (Open status for normal views, no status for
 *      spam/trash/saved-views since those buckets filter server-side).
 *
 * Setters are called with fetch=false so they don't trigger an extra API
 * call mid-transition (fetchConversationsList runs on the very next line in
 * the caller). They also won't write through to localStorage with fetch=false
 * — persistence during view-switch is this function's responsibility, so a
 * stale listType in the store can't corrupt the wrong key.
 */
function applyFiltersForView (listType, tID, vID) {
  // Save outgoing view state before mutating store fields.
  conversationStore.saveViewFilters()

  // Spam/Trash: no status filter, no carried-over ad-hoc filters.
  if (isStatusUnfilteredView(listType)) {
    conversationStore.setListStatus([], false)
    conversationStore.setAdHocFilters([], false)
    return
  }

  // Saved views (vID > 0) have their filters baked into the view definition
  // server-side, so we don't apply a status filter on top.
  if (vID) {
    conversationStore.setListStatus([], false)
    conversationStore.setAdHocFilters([], false)
    return
  }

  // Normal views (assigned/unassigned/all/mentioned/team-unassigned): try
  // to restore last-used filters; otherwise apply Open default.
  const restored = conversationStore.restoreViewFilters(listType, tID, vID)
  if (!restored) {
    conversationStore.setListStatus(CONVERSATION_DEFAULT_STATUSES.OPEN, false)
    conversationStore.setAdHocFilters([], false)
  }
}

// Init conversations list based on route params
onMounted(() => {
  if (type.value) {
    applyFiltersForView(type.value, 0, 0)
    conversationStore.fetchConversationsList(true, type.value)
  }
  if (teamID.value) {
    applyFiltersForView(CONVERSATION_LIST_TYPE.TEAM_UNASSIGNED, teamID.value, 0)
    conversationStore.fetchConversationsList(
      true,
      CONVERSATION_LIST_TYPE.TEAM_UNASSIGNED,
      teamID.value
    )
  }
  if (viewID.value) {
    applyFiltersForView(CONVERSATION_LIST_TYPE.VIEW, 0, viewID.value)
    conversationStore.fetchConversationsList(true, CONVERSATION_LIST_TYPE.VIEW, 0, [], viewID.value)
  }
})

// Refetch when route params change
watch(
  [type, teamID, viewID],
  ([newType, newTeamID, newViewID], [oldType, oldTeamID, oldViewID]) => {
    if (newType !== oldType && newType) {
      applyFiltersForView(newType, 0, 0)
      conversationStore.fetchConversationsList(true, newType)
    }
    if (newTeamID !== oldTeamID && newTeamID) {
      applyFiltersForView(CONVERSATION_LIST_TYPE.TEAM_UNASSIGNED, newTeamID, 0)
      conversationStore.fetchConversationsList(
        true,
        CONVERSATION_LIST_TYPE.TEAM_UNASSIGNED,
        newTeamID
      )
    }
    if (newViewID !== oldViewID && newViewID) {
      applyFiltersForView(CONVERSATION_LIST_TYPE.VIEW, 0, newViewID)
      conversationStore.fetchConversationsList(true, CONVERSATION_LIST_TYPE.VIEW, 0, [], newViewID)
    }
  }
)
</script>
