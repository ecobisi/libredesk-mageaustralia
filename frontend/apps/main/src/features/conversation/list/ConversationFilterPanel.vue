<template>
  <!--
    FS23: Freshdesk-style slide-out filter panel.

    Coexists with the FilterBar pill row above the list. Pill bar handles
    arbitrary saved-view-style filters (any field, any operator); the panel
    surfaces the named-field UX agents are used to from Freshdesk: Contact
    Email, Status, Agents (include/exclude), Groups (include/exclude),
    Priority, Tags, and a row of date pickers (Created, Last activity,
    Closed at, Resolved at, SLA deadline).

    State is derived directly from `conversations.adHocFilters` in the store,
    so the panel and pill bar stay in sync, and per-view persistence (FS3 +
    FS4) carries panel state across view switches and tab reloads for free.
    The panel never owns its own copy of the filter set.
  -->
  <Sheet :open="open" @update:open="$emit('update:open', $event)">
    <SheetContent
      side="right"
      class="w-80 sm:max-w-80 p-0 flex flex-col"
    >
      <!-- Required by reka-ui for screen readers; visually rendered as the
           sticky header below. The visible header carries the agent-facing
           Clear button so it can't double as the SheetTitle. -->
      <SheetTitle class="sr-only">{{ t('globals.terms.filter', 2) }}</SheetTitle>

      <!-- Header -->
      <div class="flex items-center justify-between px-4 py-3 border-b shrink-0">
        <h3 class="text-sm font-semibold">{{ t('globals.terms.filter', 2) }}</h3>
        <Button variant="ghost" size="sm" class="h-7 text-xs mr-6" @click="clearAll">
          {{ t('filter.clear') }}
        </Button>
      </div>

      <!-- Scrollable body -->
      <div class="flex-1 overflow-y-auto px-4 py-3 space-y-4">
        <!-- Contact Email -->
        <div>
          <label class="text-xs font-medium text-muted-foreground mb-1 block">
            {{ t('conversation.filterPanel.contactEmail') }}
          </label>
          <input
            v-model="contactEmail"
            type="text"
            :placeholder="t('conversation.filterPanel.searchByEmail')"
            class="w-full h-9 px-3 text-sm border rounded-md bg-transparent outline-none focus:ring-1 focus:ring-ring hover:border-ring transition-colors"
            @keydown.enter.prevent="applyAdHocFilters()"
            @blur="applyAdHocFilters()"
          />
        </div>

        <!-- Status (hidden on spam/trash because the route already pins those server-side) -->
        <FilterDropdown
          v-if="!isServerFilteredView"
          ref="statusRef"
          :label="t('globals.terms.status', 1)"
          :summary="statusSummary"
          :placeholder="t('conversation.filterPanel.anyStatus')"
          :open="statusDropOpen"
          @toggle="statusDropOpen = !statusDropOpen"
        >
          <label
            v-for="status in conversationStore.statusOptions"
            :key="status.value"
            class="flex items-center gap-2 px-3 py-2 text-sm cursor-pointer hover:bg-accent"
          >
            <Checkbox
              :checked="conversationStore.conversations.status.includes(status.label)"
              @update:checked="handleStatusToggle(status)"
            />
            {{ status.label }}
          </label>
        </FilterDropdown>

        <!-- Agent (toggle-label switches between include / exclude mode) -->
        <FilterDropdown
          ref="agentRef"
          :label="agentMode === 'include' ? t('conversation.filterPanel.agentsInclude') : t('conversation.filterPanel.agentsExclude')"
          :summary="agentSummary"
          :placeholder="t('conversation.filterPanel.anyAgent')"
          :toggleLabel="true"
          :open="agentDropOpen"
          @toggle="agentDropOpen = !agentDropOpen"
          @toggleMode="toggleAgentMode"
        >
          <div class="p-2 border-b">
            <input
              ref="agentSearchInput"
              v-model="agentSearch"
              type="text"
              :placeholder="t('filter.search')"
              class="w-full h-7 px-2 text-sm border rounded bg-transparent outline-none focus:ring-1 focus:ring-ring"
            />
          </div>
          <div class="max-h-48 overflow-y-auto">
            <button
              v-for="agent in filteredAgents"
              :key="agent.value"
              type="button"
              class="w-full text-left px-3 py-2 text-sm hover:bg-accent flex items-center justify-between cursor-pointer"
              :class="{ 'bg-accent/50': selectedAgents.includes(String(agent.value)) }"
              @click="toggleAgent(String(agent.value))"
            >
              <span class="truncate">{{ agent.label }}</span>
              <Check
                v-if="selectedAgents.includes(String(agent.value))"
                class="w-3.5 h-3.5 text-primary shrink-0"
              />
            </button>
            <div v-if="filteredAgents.length === 0" class="py-3 text-center text-xs text-muted-foreground">
              {{ t('conversation.filterPanel.noAgents') }}
            </div>
          </div>
        </FilterDropdown>

        <!-- Team -->
        <FilterDropdown
          ref="teamRef"
          :label="teamMode === 'include' ? t('conversation.filterPanel.teamsInclude') : t('conversation.filterPanel.teamsExclude')"
          :summary="teamSummary"
          :placeholder="t('conversation.filterPanel.anyTeam')"
          :toggleLabel="true"
          :open="teamDropOpen"
          @toggle="teamDropOpen = !teamDropOpen"
          @toggleMode="toggleTeamMode"
        >
          <div class="p-2 border-b">
            <input
              v-model="teamSearch"
              type="text"
              :placeholder="t('filter.search')"
              class="w-full h-7 px-2 text-sm border rounded bg-transparent outline-none focus:ring-1 focus:ring-ring"
            />
          </div>
          <div class="max-h-48 overflow-y-auto">
            <button
              v-for="team in filteredTeams"
              :key="team.value"
              type="button"
              class="w-full text-left px-3 py-2 text-sm hover:bg-accent flex items-center justify-between cursor-pointer"
              :class="{ 'bg-accent/50': selectedTeams.includes(String(team.value)) }"
              @click="toggleTeam(String(team.value))"
            >
              <span class="truncate">{{ team.label }}</span>
              <Check
                v-if="selectedTeams.includes(String(team.value))"
                class="w-3.5 h-3.5 text-primary shrink-0"
              />
            </button>
            <div v-if="filteredTeams.length === 0" class="py-3 text-center text-xs text-muted-foreground">
              {{ t('conversation.filterPanel.noTeams') }}
            </div>
          </div>
        </FilterDropdown>

        <!-- Priority -->
        <FilterDropdown
          ref="priorityRef"
          :label="t('globals.terms.priority', 1)"
          :summary="prioritySummary"
          :placeholder="t('conversation.filterPanel.anyPriority')"
          :open="priorityDropOpen"
          @toggle="priorityDropOpen = !priorityDropOpen"
        >
          <button
            v-for="priority in conversationStore.priorityOptions"
            :key="priority.value"
            type="button"
            class="w-full text-left px-3 py-2 text-sm hover:bg-accent flex items-center justify-between cursor-pointer"
            :class="{ 'bg-accent/50': selectedPriorities.includes(String(priority.value)) }"
            @click="togglePriority(String(priority.value))"
          >
            {{ priority.label }}
            <Check
              v-if="selectedPriorities.includes(String(priority.value))"
              class="w-3.5 h-3.5 text-primary shrink-0"
            />
          </button>
        </FilterDropdown>

        <!-- Tags -->
        <FilterDropdown
          ref="tagsRef"
          :label="t('globals.terms.tag', 2)"
          :summary="tagsSummary"
          :placeholder="t('conversation.filterPanel.anyTag')"
          :open="tagsDropOpen"
          @toggle="tagsDropOpen = !tagsDropOpen"
        >
          <div class="p-2 border-b">
            <input
              v-model="tagsSearch"
              type="text"
              :placeholder="t('filter.search')"
              class="w-full h-7 px-2 text-sm border rounded bg-transparent outline-none focus:ring-1 focus:ring-ring"
            />
          </div>
          <div class="max-h-48 overflow-y-auto">
            <button
              v-for="tag in filteredTags"
              :key="tag.value"
              type="button"
              class="w-full text-left px-3 py-2 text-sm hover:bg-accent flex items-center justify-between cursor-pointer"
              :class="{ 'bg-accent/50': selectedTags.includes(String(tag.value)) }"
              @click="toggleTag(String(tag.value))"
            >
              <span class="truncate">{{ tag.label }}</span>
              <Check
                v-if="selectedTags.includes(String(tag.value))"
                class="w-3.5 h-3.5 text-primary shrink-0"
              />
            </button>
            <div v-if="filteredTags.length === 0" class="py-3 text-center text-xs text-muted-foreground">
              {{ t('conversation.filterPanel.noTags') }}
            </div>
          </div>
        </FilterDropdown>

        <!-- Date filters: each maps to a relative_date conversation filter -->
        <FilterDateDropdown
          ref="createdRef"
          :label="t('filter.field.createdAt')"
          v-model="dateCreated"
          :open="createdDropOpen"
          @toggle="createdDropOpen = !createdDropOpen"
          @change="applyAdHocFilters()"
        />
        <FilterDateDropdown
          ref="lastActivityRef"
          :label="t('filter.field.lastActivity')"
          v-model="dateLastActivity"
          :open="lastActivityDropOpen"
          @toggle="lastActivityDropOpen = !lastActivityDropOpen"
          @change="applyAdHocFilters()"
        />
        <FilterDateDropdown
          ref="closedRef"
          :label="t('filter.field.closedAt')"
          v-model="dateClosedAt"
          :open="closedDropOpen"
          @toggle="closedDropOpen = !closedDropOpen"
          @change="applyAdHocFilters()"
        />
        <FilterDateDropdown
          ref="resolvedRef"
          :label="t('filter.field.resolvedAt')"
          v-model="dateResolvedAt"
          :open="resolvedDropOpen"
          @toggle="resolvedDropOpen = !resolvedDropOpen"
          @change="applyAdHocFilters()"
        />
        <FilterDateDropdown
          ref="slaRef"
          :label="t('filter.field.slaDeadline')"
          v-model="dateSLA"
          :open="slaDropOpen"
          @toggle="slaDropOpen = !slaDropOpen"
          @change="applyAdHocFilters()"
        />
      </div>

      <!-- Footer count -->
      <div v-if="activeFilterCount > 0" class="px-4 py-2 border-t shrink-0 bg-muted/30">
        <p class="text-xs text-muted-foreground">
          {{ t('conversation.filterPanel.activeCount', activeFilterCount, { count: activeFilterCount }) }}
        </p>
      </div>
    </SheetContent>
  </Sheet>
</template>

<!--
  Inline sub-components live here so the panel reads top-to-bottom in one
  file. Two purpose-built dropdowns: FilterDropdown is a generic combobox-
  style trigger + slot, FilterDateDropdown bakes the relative-date preset
  list in directly.
-->
<script>
import { h, defineComponent } from 'vue'
import { useI18n } from 'vue-i18n'
import { ChevronDown } from 'lucide-vue-next'

const FilterDropdown = defineComponent({
  name: 'FilterDropdown',
  props: {
    label: String,
    summary: String,
    placeholder: String,
    open: Boolean,
    toggleLabel: Boolean
  },
  emits: ['toggle', 'toggleMode'],
  setup (props, { slots, emit }) {
    return () => h('div', {}, [
      props.toggleLabel
        ? h('button', {
            type: 'button',
            class: 'text-xs font-medium text-muted-foreground hover:text-foreground transition-colors cursor-pointer flex items-center gap-0.5 mb-1',
            onClick: () => emit('toggleMode')
          }, [props.label, h(ChevronDown, { class: 'w-3 h-3 opacity-50' })])
        : h('label', { class: 'text-xs font-medium text-muted-foreground mb-1 block' }, props.label),
      h('div', { class: 'relative' }, [
        h('button', {
          type: 'button',
          class: 'w-full h-9 px-3 text-sm border rounded-md bg-transparent flex items-center justify-between cursor-pointer hover:border-ring transition-colors',
          onClick: () => emit('toggle')
        }, [
          h('span', { class: props.summary === props.placeholder ? 'text-muted-foreground' : '' }, props.summary),
          h(ChevronDown, { class: ['w-3.5 h-3.5 opacity-50 shrink-0 transition-transform', props.open ? 'rotate-180' : ''] })
        ]),
        props.open
          ? h('div', { class: 'absolute z-50 w-full mt-1 border rounded-md bg-popover shadow-md max-h-64 overflow-y-auto' },
              slots.default?.())
          : null
      ])
    ])
  }
})

// Preset values match the strings the SQL builder accepts
// (internal/dbutil/builder.go relativeDateRange) so the value can flow
// straight through as the relative_date filter value.
const FilterDateDropdown = defineComponent({
  name: 'FilterDateDropdown',
  props: {
    label: String,
    modelValue: String,
    open: Boolean
  },
  emits: ['update:modelValue', 'toggle', 'change'],
  setup (props, { emit }) {
    const { t } = useI18n()
    const presets = () => [
      { label: t('conversation.filterPanel.anyTime'), value: '' },
      { label: t('filter.preset.today'), value: 'today' },
      { label: t('filter.preset.yesterday'), value: 'yesterday' },
      { label: t('filter.preset.last7Days'), value: 'last_7_days' },
      { label: t('filter.preset.last30Days'), value: 'last_30_days' },
      { label: t('filter.preset.thisMonth'), value: 'this_month' }
    ]
    const summary = () => {
      const list = presets()
      if (!props.modelValue) return list[0].label
      const preset = list.find(p => p.value === props.modelValue)
      return preset ? preset.label : props.modelValue
    }
    return () => h('div', {}, [
      h('label', { class: 'text-xs font-medium text-muted-foreground mb-1 block' }, props.label),
      h('div', { class: 'relative' }, [
        h('button', {
          type: 'button',
          class: 'w-full h-9 px-3 text-sm border rounded-md bg-transparent flex items-center justify-between cursor-pointer hover:border-ring transition-colors',
          onClick: () => emit('toggle')
        }, [
          h('span', { class: !props.modelValue ? 'text-muted-foreground' : '' }, summary()),
          h(ChevronDown, { class: ['w-3.5 h-3.5 opacity-50 shrink-0 transition-transform', props.open ? 'rotate-180' : ''] })
        ]),
        props.open
          ? h('div', { class: 'absolute z-50 w-full mt-1 border rounded-md bg-popover shadow-md overflow-y-auto' },
              presets().map(preset =>
                h('button', {
                  key: preset.value,
                  type: 'button',
                  class: ['w-full text-left px-3 py-2 text-sm hover:bg-accent cursor-pointer',
                    props.modelValue === preset.value ? 'bg-accent/50 font-medium' : ''].join(' '),
                  onClick: () => {
                    emit('update:modelValue', preset.value)
                    emit('toggle')
                    emit('change')
                  }
                }, preset.label)
              )
            )
          : null
      ])
    ])
  }
})

export { FilterDropdown, FilterDateDropdown }
</script>

<script setup>
import { ref, computed, watch, nextTick, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import { useConversationStore } from '@/stores/conversation'
import { useUsersStore } from '@/stores/users'
import { useTeamStore } from '@/stores/team'
import { useTagStore } from '@/stores/tag'
import { Sheet, SheetContent, SheetTitle } from '@shared-ui/components/ui/sheet'
import { Checkbox } from '@shared-ui/components/ui/checkbox'
import { Button } from '@shared-ui/components/ui/button'
import { Check } from 'lucide-vue-next'

const props = defineProps({
  open: Boolean,
  // Comes from `route.params.type` in ConversationList. Used to suppress the
  // status filter on views where the route already pins it server-side
  // (spam, trash). Toggling status there would silently do nothing.
  viewType: { type: String, default: '' }
})

defineEmits(['update:open'])

const { t } = useI18n()
const conversationStore = useConversationStore()
const usersStore = useUsersStore()
const teamsStore = useTeamStore()
const tagStore = useTagStore()

const NO_STATUS_VIEWS = ['spam', 'trash']
const isServerFilteredView = computed(() => NO_STATUS_VIEWS.includes(props.viewType))

// Lazily fetch reference data the panel renders. The conversation list view
// already triggers users/teams; tags are panel-specific so kick that off.
onMounted(() => {
  tagStore.fetchTags()
  if (usersStore.users?.length === 0 && usersStore.fetchUsers) usersStore.fetchUsers()
  if (teamsStore.teams?.length === 0 && teamsStore.fetchTeams) teamsStore.fetchTeams()
})

// Per-section dropdown open state. One state per section so opening one
// doesn't auto-collapse another the user is mid-comparing against.
const statusDropOpen = ref(false)
const agentDropOpen = ref(false)
const teamDropOpen = ref(false)
const priorityDropOpen = ref(false)
const tagsDropOpen = ref(false)
const createdDropOpen = ref(false)
const lastActivityDropOpen = ref(false)
const closedDropOpen = ref(false)
const resolvedDropOpen = ref(false)
const slaDropOpen = ref(false)

const statusRef = ref(null)
const agentRef = ref(null)
const teamRef = ref(null)
const priorityRef = ref(null)
const tagsRef = ref(null)
const createdRef = ref(null)
const lastActivityRef = ref(null)
const closedRef = ref(null)
const resolvedRef = ref(null)
const slaRef = ref(null)
const agentSearchInput = ref(null)

// Click-outside handling for the in-section dropdowns. Runs on every
// pointerdown anywhere in the document while the panel is mounted; cheap
// (10 element refs) and stops at the first match.
function handleClickOutside (e) {
  const refs = [
    [statusRef, statusDropOpen], [agentRef, agentDropOpen], [teamRef, teamDropOpen],
    [priorityRef, priorityDropOpen], [tagsRef, tagsDropOpen],
    [createdRef, createdDropOpen], [lastActivityRef, lastActivityDropOpen],
    [closedRef, closedDropOpen], [resolvedRef, resolvedDropOpen], [slaRef, slaDropOpen]
  ]
  for (const [refEl, openState] of refs) {
    const el = refEl.value?.$el || refEl.value
    if (el && !el.contains(e.target)) openState.value = false
  }
}

onMounted(() => document.addEventListener('pointerdown', handleClickOutside))
onBeforeUnmount(() => document.removeEventListener('pointerdown', handleClickOutside))

watch(agentDropOpen, (open) => {
  if (open) nextTick(() => agentSearchInput.value?.focus())
})

// --- Status ---
const statusSummary = computed(() => {
  const s = conversationStore.conversations.status
  if (s.length === 0) return t('conversation.filterPanel.anyStatus')
  if (s.length === 1) return s[0]
  return t('conversation.nStatuses', { count: s.length })
})

function handleStatusToggle (status) {
  conversationStore.toggleListStatus(status.label)
}

// --- Agent ---
const agentSearch = ref('')
const agentMode = ref('include')
const selectedAgents = ref([])

function toggleAgentMode () {
  agentMode.value = agentMode.value === 'include' ? 'exclude' : 'include'
  if (selectedAgents.value.length > 0) applyAdHocFilters()
}

const agentSummary = computed(() => {
  if (selectedAgents.value.length === 0) return t('conversation.filterPanel.anyAgent')
  if (selectedAgents.value.length === 1) {
    const opt = (usersStore.options || []).find(o => String(o.value) === selectedAgents.value[0])
    return opt ? opt.label : selectedAgents.value[0]
  }
  return t('conversation.filterPanel.nAgents', { count: selectedAgents.value.length })
})

const filteredAgents = computed(() => {
  const options = usersStore.options || []
  if (!agentSearch.value) return options
  const q = agentSearch.value.toLowerCase()
  return options.filter(a => a.label.toLowerCase().includes(q))
})

function toggleAgent (id) {
  const idx = selectedAgents.value.indexOf(id)
  if (idx >= 0) selectedAgents.value.splice(idx, 1)
  else selectedAgents.value.push(id)
  applyAdHocFilters()
}

// --- Team ---
const teamSearch = ref('')
const teamMode = ref('include')
const selectedTeams = ref([])

function toggleTeamMode () {
  teamMode.value = teamMode.value === 'include' ? 'exclude' : 'include'
  if (selectedTeams.value.length > 0) applyAdHocFilters()
}

const teamSummary = computed(() => {
  if (selectedTeams.value.length === 0) return t('conversation.filterPanel.anyTeam')
  if (selectedTeams.value.length === 1) {
    const opt = (teamsStore.options || []).find(o => String(o.value) === selectedTeams.value[0])
    return opt ? opt.label : selectedTeams.value[0]
  }
  return t('conversation.filterPanel.nTeams', { count: selectedTeams.value.length })
})

const filteredTeams = computed(() => {
  const options = teamsStore.options || []
  if (!teamSearch.value) return options
  const q = teamSearch.value.toLowerCase()
  return options.filter(t => t.label.toLowerCase().includes(q))
})

function toggleTeam (id) {
  const idx = selectedTeams.value.indexOf(id)
  if (idx >= 0) selectedTeams.value.splice(idx, 1)
  else selectedTeams.value.push(id)
  applyAdHocFilters()
}

// --- Priority ---
const selectedPriorities = ref([])

const prioritySummary = computed(() => {
  if (selectedPriorities.value.length === 0) return t('conversation.filterPanel.anyPriority')
  if (selectedPriorities.value.length === 1) {
    const opt = conversationStore.priorityOptions.find(o => String(o.value) === selectedPriorities.value[0])
    return opt ? opt.label : selectedPriorities.value[0]
  }
  return t('conversation.filterPanel.nPriorities', { count: selectedPriorities.value.length })
})

function togglePriority (id) {
  const idx = selectedPriorities.value.indexOf(id)
  if (idx >= 0) selectedPriorities.value.splice(idx, 1)
  else selectedPriorities.value.push(id)
  applyAdHocFilters()
}

// --- Tags ---
const tagsSearch = ref('')
const selectedTags = ref([])

const tagsSummary = computed(() => {
  if (selectedTags.value.length === 0) return t('conversation.filterPanel.anyTag')
  if (selectedTags.value.length === 1) {
    const opt = (tagStore.tagOptions || []).find(o => String(o.value) === selectedTags.value[0])
    return opt ? opt.label : selectedTags.value[0]
  }
  return t('conversation.filterPanel.nTags', { count: selectedTags.value.length })
})

const filteredTags = computed(() => {
  const options = tagStore.tagOptions || []
  if (!tagsSearch.value) return options
  const q = tagsSearch.value.toLowerCase()
  return options.filter(t => t.label.toLowerCase().includes(q))
})

function toggleTag (id) {
  const idx = selectedTags.value.indexOf(id)
  if (idx >= 0) selectedTags.value.splice(idx, 1)
  else selectedTags.value.push(id)
  applyAdHocFilters()
}

// --- Contact email ---
const contactEmail = ref('')

// --- Date filters ---
const dateCreated = ref('')
const dateLastActivity = ref('')
const dateClosedAt = ref('')
const dateResolvedAt = ref('')
const dateSLA = ref('')

// Build the ad-hoc filter list and hand it to the store. Store dedupes by
// `${model}.${field}` and debounces the network call, so the panel can call
// this freely on every keystroke / toggle without burning API requests.
//
// Operators match v2 conventions:
//   tags  -> `contains` / `not contains` (space form, see FilterBar.vue)
//   email -> `ilike` (substring, case-insensitive — handled by builder)
//   else  -> `in` / `not_in`
//   date  -> `relative_date` with the preset string as the value
function applyAdHocFilters () {
  const filters = []

  if (selectedAgents.value.length > 0) {
    filters.push({
      field: 'assigned_user_id',
      operator: agentMode.value === 'include' ? 'in' : 'not_in',
      value: JSON.stringify(selectedAgents.value),
      model: 'conversations'
    })
  }
  if (selectedTeams.value.length > 0) {
    filters.push({
      field: 'assigned_team_id',
      operator: teamMode.value === 'include' ? 'in' : 'not_in',
      value: JSON.stringify(selectedTeams.value),
      model: 'conversations'
    })
  }
  if (selectedPriorities.value.length > 0) {
    filters.push({
      field: 'priority_id',
      operator: 'in',
      value: JSON.stringify(selectedPriorities.value),
      model: 'conversations'
    })
  }
  if (selectedTags.value.length > 0) {
    filters.push({
      field: 'tags',
      operator: 'contains',
      value: JSON.stringify(selectedTags.value),
      model: 'conversations'
    })
  }
  if (contactEmail.value.trim()) {
    filters.push({
      field: 'email',
      operator: 'ilike',
      value: contactEmail.value.trim(),
      model: 'users'
    })
  }

  const dateFields = [
    { ref: dateCreated, field: 'created_at' },
    { ref: dateLastActivity, field: 'last_message_at' },
    { ref: dateClosedAt, field: 'closed_at' },
    { ref: dateResolvedAt, field: 'resolved_at' },
    { ref: dateSLA, field: 'next_sla_deadline_at' }
  ]
  for (const d of dateFields) {
    if (d.ref.value) {
      filters.push({
        field: d.field,
        operator: 'relative_date',
        value: d.ref.value,
        model: 'conversations'
      })
    }
  }

  conversationStore.setAdHocFilters(filters)
}

// Footer count and the badge on the trigger button. Counts each section
// once (multiple selections in one section = one filter) plus one per
// active date row plus the status row when it's user-narrowed.
const activeFilterCount = computed(() => {
  let count = 0
  if (selectedAgents.value.length > 0) count++
  if (selectedTeams.value.length > 0) count++
  if (selectedPriorities.value.length > 0) count++
  if (selectedTags.value.length > 0) count++
  if (contactEmail.value.trim()) count++
  if (dateCreated.value) count++
  if (dateLastActivity.value) count++
  if (dateClosedAt.value) count++
  if (dateResolvedAt.value) count++
  if (dateSLA.value) count++
  if (!isServerFilteredView.value) {
    const s = conversationStore.conversations.status
    // "Open only" is the implicit default; only count status as user-set
    // when it has been narrowed beyond that single value.
    if (s.length !== 1 || s[0] !== 'Open') count++
  }
  return count
})

// Repopulate panel state from the store's current adHocFilters. Called on
// every panel-open so per-view persistence (FS3) and pill-bar edits show
// up correctly the moment the agent reopens the slide-out.
function syncFromStore () {
  const adHoc = conversationStore.conversations.adHocFilters || []

  const agentFilter = adHoc.find(f => f.field === 'assigned_user_id')
  if (agentFilter) {
    agentMode.value = agentFilter.operator === 'not_in' ? 'exclude' : 'include'
    try { selectedAgents.value = JSON.parse(agentFilter.value) } catch { selectedAgents.value = [] }
  } else {
    agentMode.value = 'include'
    selectedAgents.value = []
  }

  const teamFilter = adHoc.find(f => f.field === 'assigned_team_id')
  if (teamFilter) {
    teamMode.value = teamFilter.operator === 'not_in' ? 'exclude' : 'include'
    try { selectedTeams.value = JSON.parse(teamFilter.value) } catch { selectedTeams.value = [] }
  } else {
    teamMode.value = 'include'
    selectedTeams.value = []
  }

  const priorityFilter = adHoc.find(f => f.field === 'priority_id')
  if (priorityFilter) {
    try { selectedPriorities.value = JSON.parse(priorityFilter.value) } catch { selectedPriorities.value = [] }
  } else {
    selectedPriorities.value = []
  }

  const tagsFilter = adHoc.find(f => f.field === 'tags')
  if (tagsFilter) {
    try { selectedTags.value = JSON.parse(tagsFilter.value) } catch { selectedTags.value = [] }
  } else {
    selectedTags.value = []
  }

  const emailFilter = adHoc.find(f => f.field === 'email')
  contactEmail.value = emailFilter ? emailFilter.value : ''

  dateCreated.value = adHoc.find(f => f.field === 'created_at')?.value || ''
  dateLastActivity.value = adHoc.find(f => f.field === 'last_message_at')?.value || ''
  dateClosedAt.value = adHoc.find(f => f.field === 'closed_at')?.value || ''
  dateResolvedAt.value = adHoc.find(f => f.field === 'resolved_at')?.value || ''
  dateSLA.value = adHoc.find(f => f.field === 'next_sla_deadline_at')?.value || ''

  statusDropOpen.value = false
  agentDropOpen.value = false
  teamDropOpen.value = false
  priorityDropOpen.value = false
  tagsDropOpen.value = false
  createdDropOpen.value = false
  lastActivityDropOpen.value = false
  closedDropOpen.value = false
  resolvedDropOpen.value = false
  slaDropOpen.value = false
  agentSearch.value = ''
  teamSearch.value = ''
  tagsSearch.value = ''
}

watch(() => props.open, (isOpen) => {
  if (isOpen) syncFromStore()
})

function clearAll () {
  selectedAgents.value = []
  selectedTeams.value = []
  selectedPriorities.value = []
  selectedTags.value = []
  contactEmail.value = ''
  dateCreated.value = ''
  dateLastActivity.value = ''
  dateClosedAt.value = ''
  dateResolvedAt.value = ''
  dateSLA.value = ''
  agentMode.value = 'include'
  teamMode.value = 'include'
  agentSearch.value = ''
  teamSearch.value = ''
  tagsSearch.value = ''
  // Default status: Open only. setListStatus will persist + refetch.
  conversationStore.setListStatus(['Open'])
  conversationStore.setAdHocFilters([])
}
</script>

