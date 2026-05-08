-- name: search-conversations-by-reference-number
SELECT
    conversations.created_at,
    conversations.uuid,
    conversations.reference_number,
    conversations.subject,
    cs.name AS status
FROM conversations
LEFT JOIN conversation_statuses cs ON conversations.status_id = cs.id
WHERE reference_number::text = $1;

-- name: search-conversations-by-contact-email
SELECT
    conversations.created_at,
    conversations.uuid,
    conversations.reference_number,
    conversations.subject,
    cs.name AS status
FROM conversations
JOIN users ON conversations.contact_id = users.id
LEFT JOIN conversation_statuses cs ON conversations.status_id = cs.id
WHERE users.email = $1
ORDER BY conversations.created_at DESC
LIMIT 1000;

-- name: search-messages
SELECT
    c.created_at as "conversation_created_at",
    c.reference_number as "conversation_reference_number",
    c.uuid as "conversation_uuid",
    m.text_content,
    cs.name as "conversation_status"
FROM conversation_messages m
    JOIN conversations c ON m.conversation_id = c.id
    LEFT JOIN conversation_statuses cs ON c.status_id = cs.id
WHERE m.type != 'activity' and m.text_content ILIKE '%' || $1 || '%'
LIMIT 30;

-- name: search-contacts
SELECT
    id,
    created_at,
    first_name,
    last_name,
    email,
    external_user_id
FROM users
WHERE type = 'contact'
AND deleted_at IS NULL
AND (
    email ILIKE '%' || $1 || '%'
    OR first_name ILIKE '%' || $1 || '%'
    OR last_name ILIKE '%' || $1 || '%'
)
LIMIT 15;

-- name: search-unified
-- FS8: Single-pass search across reference number, subject, contact email,
-- and message text content. Returns one row per conversation with the most
-- relevant snippet. Uses DISTINCT ON to dedupe (a conversation can match
-- multiple predicates) and a window count for total pagination metadata.
SELECT *, COUNT(*) OVER() AS total FROM (
    SELECT DISTINCT ON (c.id)
        c.created_at,
        c.uuid,
        c.reference_number,
        c.subject,
        cs.name AS status,
        COALESCE(
            -- Prefer the latest message containing the search term (highlights
            -- the actual hit when the user searched message body).
            (SELECT m.text_content FROM conversation_messages m
             WHERE m.conversation_id = c.id AND m.type != 'activity'
             AND m.text_content ILIKE '%' || $1 || '%'
             ORDER BY m.created_at DESC LIMIT 1),
            -- Fall back to the first incoming message (gives subject/refnum
            -- matches a body snippet to display).
            (SELECT m.text_content FROM conversation_messages m
             WHERE m.conversation_id = c.id AND m.type = 'incoming' AND m.sender_type = 'contact'
             ORDER BY m.id ASC LIMIT 1),
            ''
        ) AS snippet
    FROM conversations c
    LEFT JOIN users u ON c.contact_id = u.id
    LEFT JOIN conversation_statuses cs ON c.status_id = cs.id
    WHERE c.reference_number::text = $1
       OR c.subject ILIKE '%' || $1 || '%'
       OR u.email = $1
       OR c.id IN (
           SELECT m.conversation_id FROM conversation_messages m
           WHERE m.type != 'activity' AND m.text_content ILIKE '%' || $1 || '%'
       )
    ORDER BY c.id
) sub
ORDER BY
    CASE WHEN reference_number::text = $1 THEN 0 ELSE 1 END,
    created_at DESC
LIMIT $2 OFFSET $3;
