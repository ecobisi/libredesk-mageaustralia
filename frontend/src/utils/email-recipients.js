// Strip +conv-{uuid-v4} from email if present.
// Only matches strict UUID v4 format (36 chars)
// e.g., support+conv-13216cf7-6626-4b0d-a938-46ce65a20701@domain.com -> support@domain.com
export function stripConvUUID (email) {
    if (!email) return email
    return email.replace(/\+conv-[a-f0-9]{8}-[a-f0-9]{4}-4[a-f0-9]{3}-[a-f0-9]{4}-[a-f0-9]{12}@/i, '@')
}

// Find the real customer email from message history when contact email is an inbox email.
// Scans from newest to oldest for a non-inbox from (incoming) or to (outgoing) address.
export function findRealCustomerEmail (messages, allInboxEmails) {
    const inboxSet = new Set((allInboxEmails || []).map(e => e.toLowerCase()))
    // Search newest first
    for (let i = messages.length - 1; i >= 0; i--) {
        const m = messages[i]
        const meta = m?.meta || {}
        if (m.type === 'incoming' && meta.from?.length) {
            for (const email of meta.from) {
                if (email && !inboxSet.has(stripConvUUID(email.toLowerCase()))) return email
            }
        }
        if (m.type === 'outgoing' && meta.to?.length) {
            for (const email of meta.to) {
                if (email && !inboxSet.has(stripConvUUID(email.toLowerCase()))) return email
            }
        }
    }
    return null
}

export function computeRecipientsFromMessage (message, contactEmail, inboxEmail, allInboxEmails = []) {
    const meta = message?.meta || {}
    const isIncoming = message.type === 'incoming'
    const contactLower = (contactEmail || '').toLowerCase()
    // Build set of all inbox emails to exclude from CC (handles forwarding between inboxes)
    const inboxEmailsLower = new Set(
        (allInboxEmails.length ? allInboxEmails : (inboxEmail ? [inboxEmail] : []))
            .map(e => e.toLowerCase())
    )

    // Build TO field. Use the latest message's actual addresses verbatim — that's
    // the email we're replying to, regardless of who the conversation's "contact"
    // currently points at. The previous "contact was changed" override that swapped
    // in `contactEmail` whenever it wasn't found in from/to was too broad: it
    // caught every legitimate 3rd-party CC/forward as a "contact change" and sent
    // replies to the wrong address. If the contact has actually been reassigned,
    // the caller should pass the updated value through `contactEmail` only when
    // it knows the change happened (e.g. via an `wasContactChanged` flag at the
    // call site) — not infer it from list membership.
    let toList
    if (isIncoming) {
        toList = (meta.from && meta.from.length) ? meta.from : (contactEmail ? [contactEmail] : [])
    } else {
        toList = (meta.to && meta.to.length) ? meta.to : (contactEmail ? [contactEmail] : [])
    }

    // Build CC field
    let ccList = [...(meta.cc || [])]

    if (isIncoming) {
        // Include original 'to' recipients in CC to preserve full thread context.
        if (Array.isArray(meta.to))
            ccList = ccList.concat(meta.to)
    }

    // Dedup + remove all inbox emails (including +conv-uuid variants) + remove contact email from CC
    const clean = (list, excludeExtra) => {
        const excludeLower = (excludeExtra || []).map(e => e.toLowerCase())
        return Array.from(new Set(list.filter(email => {
            if (!email) return false
            const lower = email.toLowerCase()
            const stripped = stripConvUUID(lower)
            if (inboxEmailsLower.has(stripped)) return false
            if (excludeLower.includes(lower)) return false
            return true
        })))
    }

    return {
        to: clean(toList),
        cc: clean(ccList, toList),
        // BCC stays empty — user is supposed to add it manually.
        bcc: [],
    }
}
