import { defineStore } from 'pinia'
import { computed, reactive, ref, watchEffect } from 'vue'
import { useDebounceFn } from '@vueuse/core'
import { CONVERSATION_LIST_TYPE, CONVERSATION_DEFAULT_STATUSES } from '@/constants/conversation'
import { handleHTTPError } from '@/utils/http'
import { computeRecipientsFromMessage, findRealCustomerEmail } from '@/utils/email-recipients'
import { useEmitter } from '@/composables/useEmitter'
import { EMITTER_EVENTS } from '@/constants/emitterEvents'
import MessageCache from '@/utils/conversation-message-cache'
import api from '@/api'
import { useUsersStore } from '@/stores/users'
import { useTeamStore } from '@/stores/team'
import { useInboxStore } from '@/stores/inbox'

export const useConversationStore = defineStore('conversation', () => {
  const CONV_LIST_PAGE_SIZE = 50
  const MESSAGE_LIST_PAGE_SIZE = 30
  const priorities = ref([])
  const statuses = ref([])
  const currentTo = ref([])
  const currentBCC = ref([])
  const currentCC = ref([])
  const macros = ref({})
  const isNewConversationOpen = ref(false)
  const drafts = ref(new Map())

  // Bulk selection state
  const selectedUUIDs = ref(new Set())

  // Options for select fields
  const priorityOptions = computed(() => {
    return priorities.value.map(p => ({ label: p.name, value: p.id }))
  })
  const statusOptions = computed(() => {
    return statuses.value.map(s => ({ label: s.name, value: s.id }))
  })
  // Status options excluding 'Snoozed'
  const statusOptionsNoSnooze = computed(() =>
    statuses.value.filter(s => s.name !== 'Snoozed').map(s => ({
      label: s.name,
      value: s.id
    }))
  )

  // Bulk selection methods
  let lastClickedUUID = null

  const selectedCount = computed(() => selectedUUIDs.value.size)
  const allSelected = computed(() => {
    const list = conversationsList.value
    return list.length > 0 && selectedUUIDs.value.size === list.length
  })

  function toggleSelect (uuid, shiftKey = false) {
    const next = new Set(selectedUUIDs.value)

    if (shiftKey && lastClickedUUID && lastClickedUUID !== uuid) {
      // Shift-click: select range between lastClicked and current
      const list = conversationsList.value
      const lastIdx = list.findIndex(c => c.uuid === lastClickedUUID)
      const curIdx = list.findIndex(c => c.uuid === uuid)
      if (lastIdx !== -1 && curIdx !== -1) {
        const start = Math.min(lastIdx, curIdx)
        const end = Math.max(lastIdx, curIdx)
        for (let i = start; i <= end; i++) {
          next.add(list[i].uuid)
        }
      }
    } else {
      if (next.has(uuid)) next.delete(uuid)
      else next.add(uuid)
    }

    lastClickedUUID = uuid
    selectedUUIDs.value = next
  }

  function selectAll () {
    selectedUUIDs.value = new Set(conversationsList.value.map(c => c.uuid))
  }

  function clearSelection () {
    selectedUUIDs.value = new Set()
    lastClickedUUID = null
  }

  function isSelected (uuid) {
    return selectedUUIDs.value.has(uuid)
  }

  function updateConversationField (uuid, field, value) {
    const conv = conversations.data.find(c => c.uuid === uuid)
    if (conv) {
      conv[field] = value
    }
  }

  // TODO: Move to constants.
  const sortFieldMap = {
    oldest: {
      model: 'conversations',
      field: 'last_message_at',
      order: 'asc'
    },
    newest: {
      model: 'conversations',
      field: 'last_message_at',
      order: 'desc'
    },
    started_first: {
      model: 'conversations',
      field: 'created_at',
      order: 'asc'
    },
    started_last: {
      model: 'conversations',
      field: 'created_at',
      order: 'desc'
    },
    waiting_longest: {
      model: 'conversations',
      field: 'waiting_since',
      order: 'asc'
    },
    next_sla_target: {
      model: 'conversations',
      field: 'next_sla_deadline_at',
      order: 'asc'
    },
    priority_first: {
      model: 'conversations',
      field: 'priority_id',
      order: 'desc'
    }
  }

  const sortFieldLabels = {
    oldest: 'Oldest activity',
    newest: 'Newest activity',
    started_first: 'Started first',
    started_last: 'Started last',
    waiting_longest: 'Waiting longest',
    next_sla_target: 'Next SLA target',
    priority_first: 'Priority first'
  }

  // --- Per-view filter persistence (localStorage) ---
  const FILTER_STORAGE_KEY = 'ld_view_filters'

  function _viewKey (listType, teamID, viewID) {
    if (teamID) return 'team:' + teamID
    if (viewID) return 'view:' + viewID
    return listType || 'assigned'
  }

  function saveViewFilters () {
    const key = _viewKey(conversations.listType, conversations.teamID, conversations.viewID)
    if (!key) return
    try {
      const all = JSON.parse(localStorage.getItem(FILTER_STORAGE_KEY) || '{}')
      all[key] = {
        status: conversations.status,
        sortField: conversations.sortField,
        adHocFilters: conversations.adHocFilters
      }
      localStorage.setItem(FILTER_STORAGE_KEY, JSON.stringify(all))
    } catch (_) { /* ignore */ }
  }

  function restoreViewFilters (listType, teamID, viewID) {
    const key = _viewKey(listType, teamID, viewID)
    try {
      const all = JSON.parse(localStorage.getItem(FILTER_STORAGE_KEY) || '{}')
      const saved = all[key]
      if (saved) {
        conversations.status = saved.status || ['Open']
        conversations.sortField = saved.sortField || 'started_last'
        conversations.adHocFilters = saved.adHocFilters || []
        return true
      }
    } catch (_) { /* ignore */ }
    return false
  }

  const conversations = reactive({
    data: [],
    listType: null,
    status: ['Open'],
    sortField: 'started_last',
    listFilters: [],
    viewID: 0,
    teamID: 0,
    loading: false,
    page: 1,
    hasMore: false,
    total: 0,
    errorMessage: '',
    adHocFilters: [],
    pendingUpdates: 0
  })

  const conversation = reactive({
    data: null,
    participants: {},
    loading: false,
    errorMessage: ''
  })

  const messages = reactive({
    data: new MessageCache(),
    loading: false,
    page: 1,
    // To trigger reactivity on the messages cache, simpler than making MessageCache reactive.
    version: 0,
  })

  let seenConversationUUIDs = new Map()
  let _listFetchToken = 0
  const emitter = useEmitter()

  const incrementMessageVersion = () => setTimeout(() => messages.version++, 0)

  function setListStatus (status, fetch = true) {
    if (!status || (Array.isArray(status) && status.length === 0)) {
      conversations.status = []
    } else {
      conversations.status = Array.isArray(status) ? status : [status]
    }
    // Only persist on user-initiated changes. View-switch seeding (fetch=false)
    // is the InboxView wrapper's job — it saves the outgoing view's state and
    // restores the new one's; saving here would clobber the saved good state.
    if (fetch) {
      saveViewFilters()
      resetConversations()
      reFetchConversationsList()
    }
  }

  function toggleListStatus (status) {
    const idx = conversations.status.indexOf(status)
    if (idx >= 0) {
      // Don't allow deselecting the last status
      if (conversations.status.length > 1) {
        conversations.status.splice(idx, 1)
      }
    } else {
      conversations.status.push(status)
    }
    saveViewFilters()
    resetConversations()
    reFetchConversationsList()
  }

  const getListStatus = computed(() => {
    if (conversations.status.length === 0) return 'All'
    if (conversations.status.length === 1) return conversations.status[0]
    return conversations.status.length + ' statuses'
  })

  function setListSortField (field) {
    if (conversations.sortField === field) return
    conversations.sortField = field
    saveViewFilters()
    resetConversations()
    reFetchConversationsList()
  }


  let _adHocDebounce = null
  function setAdHocFilters (filters, fetch = true) {
    // Deduplicate by model+field, keeping last entry per field
    const seen = new Map()
    for (const f of filters) {
      seen.set(f.model + "." + f.field, f)
    }
    conversations.adHocFilters = Array.from(seen.values())
    // Same persistence rule as setListStatus: only write through on user-driven
    // changes. View-switch seeding sets fetch=false and the wrapper persists.
    if (!fetch) return
    saveViewFilters()
    if (_adHocDebounce) clearTimeout(_adHocDebounce)
    _adHocDebounce = setTimeout(() => {
      resetConversations()
      reFetchConversationsList()
    }, 500)
  }

  const getListSortField = computed(() => {
    return sortFieldLabels[conversations.sortField]
  })


  async function fetchStatuses () {
    if (statuses.value.length > 0) return
    try {
      const response = await api.getStatuses()
      statuses.value = response.data.data.map(status => ({
        ...status,
        id: status.id.toString()
      }))
    } catch (error) {
      emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
        variant: 'destructive',
        description: handleHTTPError(error).message
      })
    }
  }

  async function fetchPriorities () {
    if (priorities.value.length > 0) return
    try {
      const response = await api.getPriorities()
      priorities.value = response.data.data.map(priority => ({
        ...priority,
        id: priority.id.toString()
      }))
    } catch (error) {
      emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
        variant: 'destructive',
        description: handleHTTPError(error).message
      })
    }
  }

  const conversationsList = computed(() => {
    if (!conversations.data) return []
    let filteredConversations = conversations.data
    // Filter by status if set (skip for custom views — they have server-side filters).
    if (conversations.status.length > 0 && conversations.listType !== CONVERSATION_LIST_TYPE.VIEW) {
      filteredConversations = conversations.data
        .filter(conv => {
          return conversations.status.includes(conv.status)
        })
    }

    // Sort conversations based on the selected sort field
    return [...filteredConversations].sort((a, b) => {
      const field = sortFieldMap[conversations.sortField]?.field
      if (!a[field] && !b[field]) return 0
      if (!a[field]) return 1       // null goes last
      if (!b[field]) return -1
      const order = sortFieldMap[conversations.sortField]?.order
      return order === 'asc'
        ? new Date(a[field]) - new Date(b[field])
        : new Date(b[field]) - new Date(a[field])
    })
  })

  const currentConversationHasMoreMessages = computed(() => {
    return messages.data.hasMore(conversation.data?.uuid)
  })

  const conversationMessages = computed(() => {
    const _ = messages.version // eslint-disable-line no-unused-vars
    return messages.data.getAllPagesMessages(conversation.data?.uuid)
  })

  function markConversationAsRead (uuid) {
    const index = conversations.data.findIndex(conv => conv.uuid === uuid)
    if (index !== -1) {
      setTimeout(() => {
        if (conversations.data?.[index]) {
          conversations.data[index].unread_message_count = 0
        }
      }, 3000)
    }
  }

  async function markAsUnread (uuid) {
    try {
      await api.markConversationAsUnread(uuid)
      const index = conversations.data.findIndex(conv => conv.uuid === uuid)
      if (index !== -1) {
        conversations.data[index].unread_message_count = 1
      }
    } catch (err) {
      handleHTTPError(err)
    }
  }

  const currentContactName = computed(() => {
    if (!conversation.data?.contact) return ''
    return conversation.data?.contact.first_name + ' ' + conversation.data?.contact.last_name
  })

  function getContactFullName (uuid) {
    if (conversations?.data) {
      const conv = conversations.data.find(conv => conv.uuid === uuid)
      return conv ? `${conv.contact.first_name} ${conv.contact.last_name}` : ''
    }
  }

  const current = computed(() => {
    return conversation.data || {}
  })

  const hasConversationOpen = computed(() => {
    return Object.keys(conversation.data || {}).length > 0
  })

  // Watch for changes in the conversation and messages and update the to, cc, and bcc
  watchEffect(async () => {
    const _ = messages.version // eslint-disable-line no-unused-vars
    const conv = conversation.data
    const msgData = messages.data
    const inboxEmail = conv?.inbox_mail
    if (!conv || !msgData || !inboxEmail) return

    const latestMessage = msgData.getLatestMessage(conv.uuid, ['incoming', 'outgoing'], true)
    if (!latestMessage) {
      // Reset recipients if no latest message is found.
      currentTo.value = []
      currentCC.value = []
      currentBCC.value = []
      return
    }

    const inboxStore = useInboxStore()
    const allInboxEmails = [
      ...inboxStore.inboxes.map(i => i.from).filter(Boolean),
      ...inboxStore.inboxes.flatMap(i => i.config?.email_aliases || []).filter(Boolean)
    ]
    // If contact email is an inbox email, find the real customer from message history
    let effectiveContactEmail = conv.contact?.email || ''
    const allInboxLower = new Set(allInboxEmails.map(e => e.toLowerCase()))
    if (effectiveContactEmail && allInboxLower.has(effectiveContactEmail.toLowerCase())) {
      const allMessages = msgData.getAllPagesMessages(conv.uuid) || []
      const realCustomer = findRealCustomerEmail(allMessages, allInboxEmails)
      if (realCustomer) effectiveContactEmail = realCustomer
    }
    const { to, cc, bcc } = computeRecipientsFromMessage(
      latestMessage,
      effectiveContactEmail,
      inboxEmail,
      allInboxEmails
    )
    currentTo.value = to
    currentCC.value = cc
    currentBCC.value = bcc
  })

  async function fetchParticipants (uuid) {
    try {
      const resp = await api.getConversationParticipants(uuid)
      const participants = resp.data.data.reduce((acc, p) => {
        acc[p.id] = p
        return acc
      }, {})
      updateParticipants(participants)
    } catch (error) {
      emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
        variant: 'destructive',
        description: handleHTTPError(error).message
      })
    }
  }

  async function fetchConversation (uuid) {
    conversation.loading = true
    try {
      const resp = await api.getConversation(uuid)
      conversation.data = resp.data.data
    } catch (error) {
      conversation.errorMessage = handleHTTPError(error).message
      emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
        variant: 'destructive',
        description: conversation.errorMessage
      })
    } finally {
      conversation.loading = false
    }
  }

  /**
   * Fetches messages for a conversation if not already present in the cache.
   * 
   * @param {string} uuid
   * @returns 
   */
  async function fetchMessages (uuid, fetchNextPage = false) {
    // Messages are already cached?
    let hasMessages = messages.data.getAllPagesMessages(uuid)
    if (hasMessages.length > 0 && !fetchNextPage) {
      markConversationAsRead(uuid)
      return
    }

    // Fetch messages from server.
    messages.loading = true
    // Increment page number
    let page = messages.data.getLastFetchedPage(uuid) + 1
    try {
      const response = await api.getConversationMessages(uuid, { page: page, page_size: MESSAGE_LIST_PAGE_SIZE })
      const result = response.data?.data || {}
      const newMessages = result.results || []
      markConversationAsRead(uuid)
      // Cache messages
      messages.data.addMessages(uuid, newMessages, result.page, result.total_pages)
      incrementMessageVersion()
    } catch (error) {
      emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
        variant: 'destructive',
        description: handleHTTPError(error).message
      })
    } finally {
      messages.loading = false
    }
  }

  async function fetchNextMessages () {
    fetchMessages(conversation.data.uuid, true)
  }

  /**
   * Fetches a single message from the server and adds it to the message cache.
   * 
   * @param {string} conversationUUID
   * @param {string} messageUUID
   * @returns {object}
   */
  async function fetchMessage (conversationUUID, messageUUID) {
    try {
      const response = await api.getConversationMessage(conversationUUID, messageUUID)
      if (response?.data?.data) {
        const newMsg = response.data.data
        // Add message to cache.
        messages.data.addMessage(conversationUUID, newMsg)
        incrementMessageVersion()
        return newMsg
      }
    } catch (error) {
      emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
        variant: 'destructive',
        description: handleHTTPError(error).message
      })
    }
  }

  function fetchNextConversations () {
    conversations.page++
    fetchConversationsList(true, conversations.listType, conversations.teamID, conversations.listFilters, conversations.viewID, conversations.page)
  }

  function reFetchConversationsList (showLoader = true) {
    fetchConversationsList(showLoader, conversations.listType, conversations.teamID, conversations.listFilters, conversations.viewID, conversations.page)
  }

  function resetAndRefetch () {
    resetConversations()
    reFetchConversationsList()
  }

  async function fetchFirstPageConversations () {
    await fetchConversationsList(false, conversations.listType, conversations.teamID, conversations.listFilters, conversations.viewID, 1)
  }

  async function fetchConversationsList (showLoader = true, listType = null, teamID = 0, filters = [], viewID = 0, page = 0) {
    if (!listType) return
    // Bump request token so any in-flight earlier fetch's response is discarded.
    const myToken = ++_listFetchToken
    if (conversations.listType !== listType || conversations.teamID !== teamID || conversations.viewID !== viewID) {
      resetConversations()
    }
    if (listType) conversations.listType = listType
    if (teamID) conversations.teamID = teamID
    if (viewID) conversations.viewID = viewID
    // Save base filters BEFORE merging ad-hoc (prevents accumulation)
    if (filters) conversations.listFilters = [...filters]
    // Merge ad-hoc filters
    if (conversations.adHocFilters && conversations.adHocFilters.length > 0) {
      const validAdHoc = conversations.adHocFilters.filter(f => (f.value && f.value !== "[]" && f.value !== "") || f.operator === 'not set' || f.operator === 'set')
      filters = [...filters, ...validAdHoc]
    }
    if (conversations.status && conversations.status.length > 0 && conversations.listType !== CONVERSATION_LIST_TYPE.VIEW && conversations.listType !== CONVERSATION_LIST_TYPE.SPAM && conversations.listType !== CONVERSATION_LIST_TYPE.TRASH) {
      filters = filters.filter(f => f.model !== 'conversation_statuses')
      filters.push({
        model: 'conversation_statuses',
        field: 'name',
        operator: 'in',
        value: JSON.stringify(conversations.status)
      })
    }
    if (showLoader) conversations.loading = true
    try {
      conversations.errorMessage = ''
      if (page === 0)
        page = conversations.page
      const response = await makeConversationListRequest(listType, teamID, viewID, filters, page)
      // Discard if a newer fetch has been initiated while this one was in flight.
      if (myToken !== _listFetchToken) return
      processConversationListResponse(response)
    } catch (error) {
      if (myToken !== _listFetchToken) return
      conversations.errorMessage = handleHTTPError(error).message
      conversations.total = 0
    } finally {
      if (myToken === _listFetchToken) conversations.loading = false
    }
  }

  async function makeConversationListRequest (listType, teamID, viewID, filters, page) {
    filters = filters.length > 0 ? JSON.stringify(filters) : []
    switch (listType) {
      case CONVERSATION_LIST_TYPE.ASSIGNED:
        return await api.getAssignedConversations({
          page: page,
          page_size: CONV_LIST_PAGE_SIZE,
          order_by: sortFieldMap[conversations.sortField].model + "." + sortFieldMap[conversations.sortField].field,
          order: sortFieldMap[conversations.sortField].order,
          filters
        })
      case CONVERSATION_LIST_TYPE.UNASSIGNED:
        return await api.getUnassignedConversations({
          page: page,
          page_size: CONV_LIST_PAGE_SIZE,
          order_by: sortFieldMap[conversations.sortField].model + "." + sortFieldMap[conversations.sortField].field,
          order: sortFieldMap[conversations.sortField].order,
          filters
        })
      case CONVERSATION_LIST_TYPE.ALL:
        return await api.getAllConversations({
          page: page,
          page_size: CONV_LIST_PAGE_SIZE,
          order_by: sortFieldMap[conversations.sortField].model + "." + sortFieldMap[conversations.sortField].field,
          order: sortFieldMap[conversations.sortField].order,
          filters
        })
      case CONVERSATION_LIST_TYPE.TEAM_UNASSIGNED:
        return await api.getTeamUnassignedConversations(teamID, {
          page: page,
          page_size: CONV_LIST_PAGE_SIZE,
          order_by: sortFieldMap[conversations.sortField].model + "." + sortFieldMap[conversations.sortField].field,
          order: sortFieldMap[conversations.sortField].order,
          filters
        })
      case CONVERSATION_LIST_TYPE.VIEW:
        return await api.getViewConversations(viewID, {
          page: page,
          page_size: CONV_LIST_PAGE_SIZE,
          order_by: sortFieldMap[conversations.sortField].model + "." + sortFieldMap[conversations.sortField].field,
          order: sortFieldMap[conversations.sortField].order
        })
      case CONVERSATION_LIST_TYPE.MENTIONED:
        return await api.getMentionedConversations({
          page: page,
          page_size: CONV_LIST_PAGE_SIZE,
          order_by: sortFieldMap[conversations.sortField].model + "." + sortFieldMap[conversations.sortField].field,
          order: sortFieldMap[conversations.sortField].order,
          filters
        })
      case CONVERSATION_LIST_TYPE.SPAM:
        return await api.getSpamConversations({
          page: page,
          page_size: CONV_LIST_PAGE_SIZE,
          order_by: sortFieldMap[conversations.sortField].model + "." + sortFieldMap[conversations.sortField].field,
          order: sortFieldMap[conversations.sortField].order,
          filters
        })
      case CONVERSATION_LIST_TYPE.TRASH:
        return await api.getTrashConversations({
          page: page,
          page_size: CONV_LIST_PAGE_SIZE,
          order_by: sortFieldMap[conversations.sortField].model + "." + sortFieldMap[conversations.sortField].field,
          order: sortFieldMap[conversations.sortField].order,
          filters
        })
      default:
        throw new Error('Invalid conversation list type: ' + listType)
    }
  }

  function processConversationListResponse (response) {
    const apiResponse = response.data.data
    const newConversations = []
    for (const conv of apiResponse.results) {
      if (seenConversationUUIDs.has(conv.uuid)) {
        // Update existing conversation with fresh data.
        const idx = conversations.data.findIndex(c => c.uuid === conv.uuid)
        if (idx !== -1) {
          Object.assign(conversations.data[idx], conv)
        }
      } else {
        // Add to seen and new conversations list.
        seenConversationUUIDs.set(conv.uuid, true)
        newConversations.push(conv)
      }
    }
    if (apiResponse.total_pages <= conversations.page) conversations.hasMore = false
    else conversations.hasMore = true
    if (!conversations.data) conversations.data = []
    conversations.data.push(...newConversations)
    conversations.total = apiResponse.total
  }

  async function updatePriority (v) {
    try {
      await api.updateConversationPriority(conversation.data.uuid, { priority: v })
    } catch (error) {
      emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
        variant: 'destructive',
        description: handleHTTPError(error).message
      })
    }
  }

  async function updateStatus (v) {
    try {
      await api.updateConversationStatus(conversation.data.uuid, { status: v })
    } catch (error) {
      emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
        variant: 'destructive',
        description: handleHTTPError(error).message
      })
    }
  }

  async function snoozeConversation (snoozeDuration) {
    try {
      await api.updateConversationStatus(conversation.data.uuid, { status: CONVERSATION_DEFAULT_STATUSES.SNOOZED, snoozed_until: snoozeDuration })
    } catch (error) {
      emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
        variant: 'destructive',
        description: handleHTTPError(error).message
      })
    }
  }

  async function upsertTags (v) {
    try {
      await api.upsertTags(conversation.data.uuid, v)
    } catch (error) {
      emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
        variant: 'destructive',
        description: handleHTTPError(error).message
      })
    }
  }

  async function updateAssignee (type, v) {
    try {
      await api.updateAssignee(conversation.data.uuid, type, v)
    } catch (error) {
      emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
        variant: 'destructive',
        description: handleHTTPError(error).message
      })
    }
  }

  async function removeAssignee (type) {
    try {
      await api.removeAssignee(conversation.data.uuid, type)
      conversation.data[`assigned_${type}_id`] = null
    } catch (error) {
      emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
        variant: 'destructive',
        description: handleHTTPError(error).message
      })
    }
  }

  async function updateAssigneeLastSeen (uuid) {
    try {
      await api.updateAssigneeLastSeen(uuid)
    } catch (error) {
      // pass
    }
  }

  function updateParticipants (newParticipants) {
    conversation.participants = {
      ...conversation.participants,
      ...newParticipants
    }
  }

  function conversationUUIDExists (uuid) {
    return conversations.data?.find(c => c.uuid === uuid) ? true : false
  }

// Debounced to prevent apis calls during many WS events in a short time.
  const debouncedFetchFirstPage = useDebounceFn(fetchFirstPageConversations, 1000)
  const debouncedFetchParticipants = useDebounceFn(fetchParticipants, 400)

  function refreshConversationList () {
    debouncedFetchFirstPage()
  }

  function updateConversationLastMessage (uuid, message) {
    const conv = conversations.data?.find(c => c.uuid === uuid)
    if (!conv) return
    conv.last_message = message.text_content || message.content
    conv.last_message_at = message.created_at
    conv.last_message_sender = message.sender_type
    // Update last interaction only for non-private, non-activity messages
    if (message.type !== 'activity' && !message.private) {
      conv.last_interaction = message.text_content || message.content
      conv.last_interaction_at = message.created_at
      conv.last_interaction_sender = message.sender_type
    }
    if (conv.uuid !== conversation?.data?.uuid) {
      conv.unread_message_count = (conv.unread_message_count || 0) + 1
      // Invalidate any stale cached messages for this conversation so the next open re-fetches.
      // The fetch-by-id path can silently drop the message if there's no cache entry yet (race
      // with the initial fetchMessages, or WS reconnect after a disconnect), leaving stale pages.
      messages.data.invalidate(conv.uuid)
      incrementMessageVersion()
    }
  }

  function updateConversationList (message) {
    const listConversation = conversations.data.find(c => c.uuid === message.conversation_uuid)
    if (listConversation) {
      updateConversationLastMessage(message.conversation_uuid, message)
    } else {
      // Conversation is not in the list — buffer as a pending update instead of auto-refreshing.
      conversations.pendingUpdates++
    }
  }

  /**
   * Update conversation message in the cache by fetching it from the server.
   *
   * @param {object} message - Message object with conversation_uuid field
   */
  async function updateConversationMessage (message) {
    if (conversation.data?.uuid !== message.conversation_uuid) {
      // Not the open conversation. If we have cached messages for it,
      // fetch the new message to keep the cache fresh.
      if (messages.data.getLastFetchedPage(message.conversation_uuid) > 0) {
        const fetchedMessage = await fetchMessage(message.conversation_uuid, message.uuid)
        if (fetchedMessage) {
          // Update last message in conversation list (preview)
          updateConversationLastMessage(message.conversation_uuid, fetchedMessage)
        }
      }
      return
    }

    // Open conversation and message not in cache? Fetch from server.
    if (!messages.data.hasMessage(message.conversation_uuid, message.uuid)) {
      debouncedFetchParticipants(message.conversation_uuid)
      const fetchedMessage = await fetchMessage(message.conversation_uuid, message.uuid)
      if (fetchedMessage) {
        updateConversationLastMessage(message.conversation_uuid, fetchedMessage)
        setTimeout(() => {
          emitter.emit(EMITTER_EVENTS.NEW_MESSAGE, {
            conversation_uuid: message.conversation_uuid,
            message: fetchedMessage
          })
        }, 100)
      }

      updateAssigneeLastSeen(message.conversation_uuid)
    }
  }

  function addNewConversation (conv) {
    if (!conversationUUIDExists(conv.uuid)) {
      // Buffer as a pending update instead of auto-refreshing.
      conversations.pendingUpdates++
    }
  }

  function applyPendingUpdates () {
    conversations.pendingUpdates = 0
    resetConversations()
    reFetchConversationsList()
  }

  /**
   * Update a single message property in the cache.
   * 
   * @param {Object} message - Message
   */
  function updateMessageProp (message) {
    const exists = messages.data.hasMessage(message.conversation_uuid, message.uuid)
    if (exists) {
      messages.data.updateMessageField(message.conversation_uuid, message.uuid, message.prop, message.value)
      incrementMessageVersion()
    }
  }

  /**
   * Update a conversation property, supports nested paths via dot notation
   * @param {Object} update - { uuid, prop, value }
   */
  function updateConversationProp (update) {
    const updateNested = (obj, prop, value) => {
      if (!prop.includes('.')) {
        obj[prop] = value
        return
      }

      const keys = prop.split('.')
      const lastKey = keys.pop()
      const target = keys.reduce((o, key) => o[key] = o[key] || {}, obj)
      target[lastKey] = value
    }

    const { uuid, prop, value } = update

    // Resolve names for ID-only WebSocket updates so the UI shows names, not IDs
    const resolveAssignmentName = (obj) => {
      if (prop === 'assigned_user_id') {
        if (value) {
          const usersStore = useUsersStore()
          const user = usersStore.options.find(u => String(u.value) === String(value))
          obj.assigned_user_name = user ? user.label : obj.assigned_user_name
        } else {
          obj.assigned_user_name = null
        }
      }
      if (prop === 'assigned_team_id') {
        if (value) {
          const teamStore = useTeamStore()
          const team = teamStore.options.find(t => String(t.value) === String(value))
          obj.assigned_team_name = team ? team.label : obj.assigned_team_name
        } else {
          obj.assigned_team_name = null
        }
      }
    }

    // Conversation is currently open? Update it.
    if (conversation.data?.uuid === uuid) {
      updateNested(conversation.data, prop, value)
      resolveAssignmentName(conversation.data)
    }

    // Update conversation if it exists in the list.
    const existingConversation = conversations?.data?.find(c => c.uuid === uuid)
    if (existingConversation) {
      const oldStatus = existingConversation.status
      updateNested(existingConversation, prop, value)
      resolveAssignmentName(existingConversation)

      // If status changed, remove from list if it no longer matches the view/filter.
      if (prop === 'status' && oldStatus !== value) {
        // For views: status filtering is server-side, so remove if status changed
        if (conversations.viewID > 0) {
          conversations.data = conversations.data.filter(c => c.uuid !== uuid)
          conversations.total = Math.max(0, conversations.total - 1)
        }
        // For SPAM/TRASH: server-side filtered by exact status — remove if no longer matches.
        if (conversations.listType === CONVERSATION_LIST_TYPE.SPAM && value !== 'Spam') {
          conversations.data = conversations.data.filter(c => c.uuid !== uuid)
          conversations.total = Math.max(0, conversations.total - 1)
        }
        if (conversations.listType === CONVERSATION_LIST_TYPE.TRASH && value !== 'Trashed') {
          conversations.data = conversations.data.filter(c => c.uuid !== uuid)
          conversations.total = Math.max(0, conversations.total - 1)
        }
        // For normal inbox: the computed conversationsList filter handles it
      }

      // Also remove if assignment changed and we're on assigned/unassigned view
      if (prop === 'assigned_user_id') {
        if (conversations.listType === 'assigned' && !value) {
          conversations.data = conversations.data.filter(c => c.uuid !== uuid)
          conversations.total = Math.max(0, conversations.total - 1)
        }
        if (conversations.listType === 'unassigned' && value) {
          conversations.data = conversations.data.filter(c => c.uuid !== uuid)
          conversations.total = Math.max(0, conversations.total - 1)
        }
      }
    }
  }

  function resetCurrentConversation () {
    Object.assign(conversation, {
      data: null,
      participants: {},
      macro: {},
      loading: false,
      errorMessage: ''
    })
  }

  function resetConversations () {
    conversations.data = []
    conversations.page = 1
    conversations.pendingUpdates = 0
    seenConversationUUIDs = new Map()
    clearSelection()
  }


  /** Macros set for new conversation or an open conversation **/
  function setMacro (macro, context) {
    macros.value[context] = macro
  }

  function setMacroActions (actions, context) {
    if (!macros.value[context]) {
      macros.value[context] = {}
    }
    macros.value[context].actions = actions
  }

  function getMacro (context) {
    return macros.value[context] || {}
  }

  function removeMacroAction (action, context) {
    if (!macros.value[context]) return
    macros.value[context].actions = macros.value[context].actions.filter(a => a.type !== action.type)
  }

  function resetMacro (context) {
    macros.value = { ...macros.value, [context]: {} }
  }

  // Fetch all drafts for the current user
  async function fetchAllDrafts () {
    try {
      const resp = await api.getAllDrafts()
      const newDrafts = new Map()
      if (resp.data?.data) {
        for (const draft of resp.data.data) {
          newDrafts.set(draft.conversation_uuid, draft)
        }
      }
      drafts.value = newDrafts
    } catch (e) {
      emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
        variant: 'destructive',
        description: handleHTTPError(e).message
      })
    }
  }

  // Get draft for a specific conversation
  function getDraft (uuid) {
    return drafts.value.get(uuid)
  }

  // Set draft for a specific conversation
  function setDraft (uuid, draft) {
    drafts.value.set(uuid, draft)
    // Trigger reactivity
    drafts.value = new Map(drafts.value)
  }

  // Remove draft for a specific conversation
  function removeDraft (uuid) {
    drafts.value.delete(uuid)
    // Trigger reactivity
    drafts.value = new Map(drafts.value)
  }

  // Check if a conversation has a draft
  function hasDraft (uuid) {
    return drafts.value.has(uuid)
  }

  return {
    macros,
    conversations,
    conversation,
    messages,
    conversationsList,
    conversationMessages,
    currentConversationHasMoreMessages,
    hasConversationOpen,
    current,
    currentContactName,
    currentTo,
    currentBCC,
    currentCC,
    conversationUUIDExists,
    updateConversationProp,
    addNewConversation,
    applyPendingUpdates,
    getContactFullName,
    fetchParticipants,
    fetchNextMessages,
    fetchNextConversations,
    updateMessageProp,
    updateAssigneeLastSeen,
    markAsUnread,
    updateConversationMessage,
    updateConversationList,
    snoozeConversation,
    fetchConversation,
    fetchConversationsList,
    fetchMessages,
    upsertTags,
    updateAssignee,
    updatePriority,
    updateStatus,
    refreshConversationList,
    updateConversationLastMessage,
    resetCurrentConversation,
    fetchFirstPageConversations,
    resetAndRefetch,
    fetchStatuses,
    fetchPriorities,
    setListSortField,
    setListStatus,
    toggleListStatus,
    setAdHocFilters,
    isNewConversationOpen,
    removeMacroAction,
    getMacro,
    setMacro,
    resetMacro,
    setMacroActions,
    removeAssignee,
    getListSortField,
    getListStatus,
    saveViewFilters,
    restoreViewFilters,
    statuses,
    priorities,
    priorityOptions,
    statusOptionsNoSnooze,
    statusOptions,
    drafts,
    fetchAllDrafts,
    getDraft,
    setDraft,
    removeDraft,
    hasDraft,
    selectedUUIDs,
    selectedCount,
    allSelected,
    toggleSelect,
    selectAll,
    clearSelection,
    isSelected
  }
})
