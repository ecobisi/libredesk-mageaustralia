// Strip +conv-{uuid-v4} from email if present.
// Only matches strict UUID v4 format (36 chars)
// e.g., support+conv-13216cf7-6626-4b0d-a938-46ce65a20701@domain.com -> support@domain.com
export function stripConvUUID (email) {
    if (!email) return email
    return email.replace(/\+conv-[a-f0-9]{8}-[a-f0-9]{4}-4[a-f0-9]{3}-[a-f0-9]{4}-[a-f0-9]{12}@/i, '@')
}

export function computeRecipientsFromMessage (message, contactEmail, inboxEmail, inboxReplyTo = '') {
    const meta = message?.meta || {}
    const isIncoming = message.type === 'incoming'
    const contactLower = (contactEmail || '').toLowerCase()

    // Build TO field. The conversation's current contact is always the primary
    // recipient — if the agent has swapped the contact mid-thread (UX18), the
    // meta on the source message still references the old sender's address;
    // detect that mismatch and substitute the current contact instead.
    let toList
    if (isIncoming) {
        if (meta.from && meta.from.length) {
            const fromLower = meta.from.map(e => e.toLowerCase())
            if (contactLower && !fromLower.includes(contactLower)) {
                toList = [contactEmail]
            } else {
                toList = meta.from
            }
        } else {
            toList = contactEmail ? [contactEmail] : []
        }
    } else {
        if (meta.to && meta.to.length) {
            const toLower = meta.to.map(e => e.toLowerCase())
            if (contactLower && !toLower.includes(contactLower)) {
                toList = [contactEmail]
            } else {
                toList = meta.to
            }
        } else {
            toList = contactEmail ? [contactEmail] : []
        }
    }

    // Build CC field. Slice meta.cc so we don't mutate the message object when
    // we concat() additional addresses below.
    let ccList = [...(meta.cc || [])]

    if (isIncoming) {
        // Include original 'to' recipients in CC to preserve full thread context (e.g. other participants)
        if (Array.isArray(meta.to))
            ccList = ccList.concat(meta.to)
    }

    const inboxAddresses = [inboxEmail, inboxReplyTo]
        .filter(Boolean)
        .map(e => e.toLowerCase())
    const clean = (list, excludeExtra) => {
        const excludeLower = (excludeExtra || []).map(e => (e || '').toLowerCase())
        return Array.from(new Set(list.filter(email => {
            if (!email) return false
            const lower = email.toLowerCase()
            if (inboxAddresses.includes(stripConvUUID(lower))) return false
            if (excludeLower.includes(lower)) return false
            return true
        })))
    }

    return {
        to: clean(toList),
        // Strip anything from CC that's already in TO so the contact doesn't
        // get a duplicate copy — addresses (especially after a contact swap)
        // can otherwise show up in both columns.
        cc: clean(ccList, toList),
        // BCC stays empty user is supposed to add it manually.
        bcc: [],
    }
}
