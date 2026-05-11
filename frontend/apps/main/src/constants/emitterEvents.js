export const EMITTER_EVENTS = {
    EDIT_MODEL: 'edit-model',
    REFRESH_LIST: 'refresh-list',
    SHOW_TOAST: 'show-toast',
    SHOW_SOONER: 'show-sooner',
    NEW_MESSAGE: 'new-message',
    SET_NESTED_COMMAND: 'set-nested-command',
    CONVERSATION_SIDEBAR_TOGGLE: 'conversation-sidebar-toggle',
    SCROLL_TO_MESSAGE: 'scroll-to-message',
    FORWARD_MESSAGE: 'forward-message',
    // EC3: Undo-send pipeline. ReplyBox emits SEND_QUEUED with the prepared
    // payload + restoreData; Conversation.vue holds the 5s timer and emits
    // RESTORE_SEND back into ReplyBox if the agent clicks Undo.
    SEND_QUEUED: 'send-queued',
    RESTORE_SEND: 'restore-send',
    // UX10: global keyboard shortcuts. App.vue listens for R / N keys and
    // emits these; the active conversation's ReplyBox switches messageType
    // and focuses the editor.
    SHORTCUT_REPLY: 'shortcut-reply',
    SHORTCUT_NOTE: 'shortcut-note',
    // T3a / T3r workaround: Vue's parent-emit chain from ReplyBoxMenuBar → ReplyBoxContent → ReplyBox
    // silently drops these specific events in v2.1.1 + radix-vue Button wrappers (other events on the
    // same MenuBar like emojiSelect work fine, but generateResponse/generateWithOrders don't reach
    // the parent listener). Bypass via the global emitter bus — MenuBar emits these and ReplyBox
    // subscribes in onMounted. Root cause not isolated; keep the bypass until reproduced upstream.
    RAG_GENERATE: 'rag-generate',
    RAG_GENERATE_WITH_ORDERS: 'rag-generate-with-orders'
}
