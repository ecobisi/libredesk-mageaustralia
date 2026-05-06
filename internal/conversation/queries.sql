-- name: unsnooze-all
UPDATE conversations
SET snoozed_until = NULL, status_id = (SELECT id FROM conversation_statuses WHERE name = 'Open')
WHERE snoozed_until <= NOW()
  AND status_id = (SELECT id FROM conversation_statuses WHERE name = 'Snoozed');

-- name: insert-conversation
-- $11 = rate limit window start (timestamptz), $12 = max conversations (0 = unlimited)
-- $13 = subject reference marker template (placeholder: {ref})
WITH
status_id AS (
    SELECT id FROM conversation_statuses WHERE name = $2
),
reference_number AS (
    SELECT generate_reference_number($7) AS reference_number
)
INSERT INTO conversations
(contact_id, status_id, inbox_id, last_message, last_message_at, subject, reference_number, meta, custom_attributes)
SELECT
   $1,
   (SELECT id FROM status_id),
   $3,
   $4,
   $5,
   CASE
      WHEN $8 = TRUE THEN CONCAT($6::text, ' - ', REPLACE($13::text, '{ref}', (SELECT reference_number FROM reference_number)))
      ELSE $6::text
   END,
   (SELECT reference_number FROM reference_number),
   $9,
   $10
WHERE $12::int = 0 OR (SELECT COUNT(*) FROM conversations WHERE contact_id = $1 AND created_at >= $11) < $12::int
RETURNING id, uuid;

-- name: get-conversations
-- $1 = viewing user ID for per-agent unread count
-- $2 = include mentioned message UUID (true for mentioned inbox, false otherwise)
SELECT
    COUNT(*) OVER() as total,
    conversations.id,
    conversations.created_at,
    conversations.updated_at,
    conversations.uuid,
    conversations.reference_number,
    conversations.waiting_since,
    users.created_at as "contact.created_at",
    users.updated_at as "contact.updated_at",
    users.first_name as "contact.first_name",
    users.last_name as "contact.last_name",
    users.email as "contact.email",
    users.avatar_url as "contact.avatar_url",
    inboxes.channel as inbox_channel,
    inboxes.name as inbox_name,
    conversations.sla_policy_id,
    conversations.first_reply_at,
    conversations.last_reply_at,
    conversations.resolved_at,
    conversations.subject,
    conversations.last_message,
    conversations.last_message_at,
    conversations.last_message_sender,
    (SELECT text_content FROM conversation_messages
     WHERE conversation_id = conversations.id AND type != 'activity'
     ORDER BY id ASC LIMIT 1) AS first_message,
    conversations.last_interaction,
    conversations.last_interaction_at,
    conversations.last_interaction_sender,
    conversations.next_sla_deadline_at,
    conversations.priority_id,
    (
    SELECT CASE WHEN COUNT(*) > 9 THEN 10 ELSE COUNT(*) END
    FROM (
        SELECT 1 FROM conversation_messages
        WHERE conversation_id = conversations.id
        AND created_at > COALESCE(
            (SELECT last_seen_at FROM conversation_last_seen
             WHERE conversation_id = conversations.id AND user_id = $1),
            '1970-01-01'::TIMESTAMPTZ
        )
        AND (meta IS NULL OR NOT COALESCE((meta->>'continuity_email')::boolean, false))
        LIMIT 10
    ) t
    ) as unread_message_count,
    conversation_statuses.name as status,
    conversation_priorities.name as priority,
    as_latest.first_response_deadline_at,
    as_latest.resolution_deadline_at,
    as_latest.id as applied_sla_id,
    nxt_resp_event.deadline_at AS next_response_deadline_at,
    nxt_resp_event.met_at as next_response_met_at,
    CASE WHEN $2 = true THEN (
        SELECT msg.uuid
        FROM conversation_mentions cm2
        JOIN conversation_messages msg ON msg.id = cm2.message_id
        WHERE cm2.conversation_id = conversations.id
          AND (cm2.mentioned_user_id = $1 OR EXISTS(
              SELECT 1 FROM team_members tm2
              WHERE tm2.team_id = cm2.mentioned_team_id AND tm2.user_id = $1
          ))
        ORDER BY cm2.created_at DESC
        LIMIT 1
    ) ELSE NULL END as mentioned_message_uuid,
    CASE WHEN conversations.assigned_user_id IS NOT NULL
         THEN assigned_users.first_name || ' ' || assigned_users.last_name
         ELSE NULL END as assigned_user_name,
    assigned_teams.name as assigned_team_name
    FROM conversations
    JOIN users ON contact_id = users.id
    JOIN inboxes ON inbox_id = inboxes.id
    LEFT JOIN conversation_statuses ON status_id = conversation_statuses.id
    LEFT JOIN conversation_priorities ON priority_id = conversation_priorities.id
    LEFT JOIN teams assigned_teams ON conversations.assigned_team_id = assigned_teams.id
    LEFT JOIN users assigned_users ON conversations.assigned_user_id = assigned_users.id
    LEFT JOIN LATERAL (
        SELECT id, first_response_deadline_at, resolution_deadline_at
        FROM applied_slas 
        WHERE conversation_id = conversations.id 
        ORDER BY created_at DESC LIMIT 1
    ) as_latest ON true
    LEFT JOIN LATERAL (
        SELECT se.deadline_at, se.met_at
        FROM sla_events se
        WHERE se.applied_sla_id = as_latest.id
        AND se.type = 'next_response'
        ORDER BY se.created_at DESC
        LIMIT 1
    ) nxt_resp_event ON true
WHERE 1=1 %s

-- name: get-conversation
SELECT
   c.id,
   c.created_at,
   c.updated_at,
   c.closed_at,
   c.resolved_at,
   c.inbox_id,
   inb.name as inbox_name,
   COALESCE(inb.from, '') as inbox_mail,
   COALESCE(inb.config->>'reply_to', '') as inbox_reply_to,
   COALESCE(inb.channel::TEXT, '') as inbox_channel,
   -- UX10: comma-joined `To:` header(s) from the first incoming message.
   -- Powers the `message_to_email` automation field. NULL when the
   -- conversation was outgoing-first or the meta->'to' shape isn't an array.
   COALESCE((SELECT string_agg(elem, ',') FROM jsonb_array_elements_text((SELECT meta->'to' FROM conversation_messages WHERE conversation_id = c.id AND type = 'incoming' ORDER BY id LIMIT 1)) elem), '') as message_to_email,
   c.status_id,
   c.priority_id,
   p.name as priority,
   s.name as status,
   c.uuid,
   c.reference_number,
   c.first_reply_at,
   c.last_reply_at,
   c.waiting_since,
   c.assigned_user_id,
   c.assigned_team_id,
   c.subject,
   c.contact_id,
   c.sla_policy_id,
   c.meta,
   sla.name as sla_policy_name,
   c.last_message_at,
   c.last_message_sender,
   c.last_message,
   c.last_interaction,
   c.last_interaction_at,
   c.last_interaction_sender,
   c.custom_attributes,
   (SELECT COALESCE(
       (SELECT json_agg(t.name)
       FROM tags t
       INNER JOIN conversation_tags ct ON ct.tag_id = t.id
       WHERE ct.conversation_id = c.id),
       '[]'::json
   )) AS tags,
   ct.id as "contact.id",
   ct.created_at as "contact.created_at",
   ct.updated_at as "contact.updated_at",
   ct.first_name as "contact.first_name",
   ct.last_name as "contact.last_name", 
   ct.email as "contact.email",
   ct.type as "contact.type",
   ct.availability_status as "contact.availability_status",
   ct.avatar_url as "contact.avatar_url",
   ct.phone_number as "contact.phone_number",
   ct.phone_number_country_code as "contact.phone_number_country_code",
   ct.country as "contact.country",
   ct.custom_attributes as "contact.custom_attributes",
   ct.enabled as "contact.enabled",
   ct.last_active_at as "contact.last_active_at",
   ct.last_login_at as "contact.last_login_at",
   ct.external_user_id as "contact.external_user_id",
   as_latest.first_response_deadline_at,
   as_latest.resolution_deadline_at,
   as_latest.id as applied_sla_id,
   nxt_resp_event.deadline_at AS next_response_deadline_at,
   nxt_resp_event.met_at as next_response_met_at,
   c.last_continuity_email_sent_at,
   c.merged_into_conversation_id,
   c.merged_at,
   (SELECT uuid FROM conversations mc WHERE mc.id = c.merged_into_conversation_id) AS merged_into_uuid,
   (SELECT reference_number FROM conversations mc WHERE mc.id = c.merged_into_conversation_id) AS merged_into_ref
FROM conversations c
JOIN users ct ON c.contact_id = ct.id
JOIN inboxes inb ON c.inbox_id = inb.id
LEFT JOIN sla_policies sla ON c.sla_policy_id = sla.id
LEFT JOIN teams at ON at.id = c.assigned_team_id
LEFT JOIN conversation_statuses s ON c.status_id = s.id
LEFT JOIN conversation_priorities p ON c.priority_id = p.id
LEFT JOIN LATERAL (
    SELECT id, first_response_deadline_at, resolution_deadline_at
    FROM applied_slas
    WHERE conversation_id = c.id 
    ORDER BY created_at DESC LIMIT 1
) as_latest ON true
LEFT JOIN LATERAL (
  SELECT se.deadline_at, se.met_at
  FROM sla_events se
  WHERE se.applied_sla_id = as_latest.id
  AND se.type = 'next_response'
  ORDER BY se.created_at DESC
  LIMIT 1
) nxt_resp_event ON true
WHERE 
  ($1 > 0 AND c.id = $1)
  OR 
  ($2::uuid IS NOT NULL AND c.uuid = $2::uuid)
  OR
  ($3::TEXT != '' AND c.reference_number = $3::TEXT)


-- name: get-conversations-created-after
SELECT
    c.id,
    c.uuid
FROM conversations c
WHERE c.created_at > $1;

-- name: get-contact-previous-conversations
SELECT
    c.id,
    c.created_at,
    c.updated_at,
    c.uuid,
    c.subject,
    u.first_name AS "contact.first_name",
    u.last_name AS "contact.last_name",
    u.avatar_url AS "contact.avatar_url",
    c.last_message as last_message,
    c.last_message_at as last_message_at
FROM users u
JOIN conversations c ON c.contact_id = u.id
WHERE c.contact_id = $1
ORDER BY c.created_at DESC
LIMIT $2;

-- name: get-chat-conversation
SELECT
    c.created_at,
    c.uuid,
    cs.name as status,
    COALESCE(c.last_interaction, '') as "last_message.content",
    c.last_interaction_at as "last_message.created_at",
    COALESCE(lis.id, 0) AS "last_message.author.id",
    COALESCE(lis.first_name, '') AS "last_message.author.first_name",
    COALESCE(lis.last_name, '') AS "last_message.author.last_name",
    COALESCE(lis.avatar_url, '') AS "last_message.author.avatar_url",
    COALESCE(c.last_interaction_sender::TEXT, '') AS "last_message.author.type",
    (SELECT CASE WHEN COUNT(*) > 9 THEN 10 ELSE COUNT(*) END
     FROM (
         SELECT 1 FROM conversation_messages unread
         WHERE unread.conversation_id = c.id
           AND unread.created_at > c.contact_last_seen_at
           AND unread.type = 'outgoing'
           AND unread.private = false
         LIMIT 10
     ) t) AS unread_message_count,
    COALESCE(au.availability_status::TEXT, '') as "assignee.availability_status",
    au.avatar_url as "assignee.avatar_url",
    COALESCE(au.first_name, '') as "assignee.first_name",
    COALESCE(au.id, 0) as "assignee.id",
    COALESCE(au.last_name, '') as "assignee.last_name",
    COALESCE(au.type::TEXT, '') as "assignee.type"
FROM conversations c
INNER JOIN inboxes inb on c.inbox_id = inb.id
LEFT JOIN conversation_statuses cs ON c.status_id = cs.id
LEFT JOIN users au ON c.assigned_user_id = au.id
LEFT JOIN users lis ON c.last_interaction_sender_id = lis.id
WHERE c.uuid = $1
  AND inb.deleted_at IS NULL;

-- name: get-contact-chat-conversations
SELECT
    c.created_at,
    c.uuid,
    cs.name as status,
    COALESCE(c.last_interaction, '') as "last_message.content",
    c.last_interaction_at as "last_message.created_at",
    COALESCE(lis.id, 0) AS "last_message.author.id",
    COALESCE(lis.first_name, '') AS "last_message.author.first_name",
    COALESCE(lis.last_name, '') AS "last_message.author.last_name",
    COALESCE(lis.avatar_url, '') AS "last_message.author.avatar_url",
    COALESCE(c.last_interaction_sender::TEXT, '') AS "last_message.author.type",
    (SELECT CASE WHEN COUNT(*) > 9 THEN 10 ELSE COUNT(*) END
     FROM (
         SELECT 1 FROM conversation_messages unread
         WHERE unread.conversation_id = c.id
           AND unread.created_at > c.contact_last_seen_at
           AND unread.type = 'outgoing'
           AND unread.private = false
         LIMIT 10
     ) t) AS unread_message_count,
    COALESCE(au.availability_status::TEXT, '') as "assignee.availability_status",
    au.avatar_url as "assignee.avatar_url",
    COALESCE(au.first_name, '') as "assignee.first_name",
    COALESCE(au.id, 0) as "assignee.id",
    COALESCE(au.last_name, '') as "assignee.last_name",
    COALESCE(au.type::TEXT, '') as "assignee.type"
FROM conversations c
INNER JOIN inboxes inb ON c.inbox_id = inb.id
INNER JOIN users con ON c.contact_id = con.id
LEFT JOIN conversation_statuses cs ON c.status_id = cs.id
LEFT JOIN users au ON c.assigned_user_id = au.id
LEFT JOIN users lis ON c.last_interaction_sender_id = lis.id
WHERE c.contact_id = $1 AND c.inbox_id = $2
  AND inb.deleted_at IS NULL
  AND con.deleted_at IS NULL
ORDER BY c.created_at DESC
LIMIT 200;

-- name: get-conversation-uuid
SELECT uuid from conversations where id = $1;

-- name: update-conversation-assigned-user
UPDATE conversations
SET assigned_user_id = $2,
updated_at = NOW()
WHERE uuid = $1;

-- name: update-conversation-contact-last-seen
UPDATE conversations
SET contact_last_seen_at = NOW(),
updated_at = NOW()
WHERE uuid = $1;

-- name: update-conversation-assigned-team
UPDATE conversations
SET assigned_team_id = $2,
updated_at = NOW()
WHERE uuid = $1;


-- name: update-conversation-status
WITH new_status AS (
    SELECT id, category FROM conversation_statuses WHERE name = $2
)
UPDATE conversations
SET status_id     = (SELECT id FROM new_status),
    resolved_at   = COALESCE(resolved_at, CASE WHEN (SELECT category FROM new_status) = 'resolved' THEN NOW() END),
    closed_at     = COALESCE(closed_at,   CASE WHEN $2 = 'Closed'                                  THEN NOW() END),
    -- Trashed sets the timestamp on the way in; transitioning out of Trashed clears it.
    trashed_at    = CASE WHEN $2 = 'Trashed' THEN COALESCE(trashed_at, NOW()) ELSE NULL END,
    snoozed_until = CASE WHEN $2 = 'Snoozed' THEN $3::timestamptz ELSE NULL END,
    updated_at    = NOW()
WHERE uuid = $1;

-- name: get-user-active-conversations-count
SELECT COUNT(*) FROM conversations WHERE status_id IN (SELECT id FROM conversation_statuses WHERE category = 'open') AND assigned_user_id = $1;

-- name: update-conversation-priority
UPDATE conversations
SET priority_id = (SELECT id FROM conversation_priorities WHERE name = $2),
    updated_at = NOW()
WHERE uuid = $1;

-- name: update-conversation-subject
UPDATE conversations
SET subject = $2,
    updated_at = NOW()
WHERE uuid = $1;

-- name: upsert-user-last-seen
INSERT INTO conversation_last_seen (user_id, conversation_id, last_seen_at)
VALUES ($1, (SELECT id FROM conversations WHERE uuid = $2), NOW())
ON CONFLICT (conversation_id, user_id)
DO UPDATE SET last_seen_at = NOW(), updated_at = NOW();

-- name: update-conversation-last-message
-- $1=id, $2=uuid, $3=content, $4=sender_type, $5=timestamp, $6=message_type, $7=private, $8=sender_id
UPDATE conversations SET
    last_message = $3,
    last_message_sender = $4,
    last_message_sender_id = $8,
    last_message_at = $5,
    last_interaction = CASE WHEN $6 != 'activity' AND $7 = false THEN $3 ELSE last_interaction END,
    last_interaction_sender = CASE WHEN $6 != 'activity' AND $7 = false THEN $4 ELSE last_interaction_sender END,
    last_interaction_sender_id = CASE WHEN $6 != 'activity' AND $7 = false THEN $8 ELSE last_interaction_sender_id END,
    last_interaction_at = CASE WHEN $6 != 'activity' AND $7 = false THEN $5 ELSE last_interaction_at END,
    updated_at = NOW()
WHERE CASE
    WHEN $1 > 0 THEN id = $1
    ELSE uuid = $2
END

-- name: get-conversation-participants
-- Only agents can be conversation followers (UX5 — followers are agents
-- watching a ticket they're not assigned to). The contacts auto-added by
-- InsertMessage stay in the table for the activity-log "added as
-- participant" trail and for the email-recipients computation, but they
-- do NOT show up in the followers picker or notifyParticipants fan-out.
SELECT users.id as id, first_name, last_name, avatar_url
FROM conversation_participants
INNER JOIN users ON users.id = conversation_participants.user_id
WHERE users.type = 'agent' AND conversation_id =
(
    SELECT id FROM conversations WHERE uuid = $1
);

-- name: insert-conversation-participant
INSERT INTO conversation_participants
(user_id, conversation_id)
VALUES($1, (SELECT id FROM conversations WHERE uuid = $2));

-- name: delete-conversation-participant
DELETE FROM conversation_participants
WHERE user_id = $1
  AND conversation_id = (SELECT id FROM conversations WHERE uuid = $2);

-- name: get-unassigned-conversations
SELECT
    c.created_at,
    c.updated_at,
    c.uuid,
    c.assigned_team_id,
    inb.channel as inbox_channel,
    inb.name as inbox_name
FROM conversations c
    JOIN inboxes inb ON c.inbox_id = inb.id 
WHERE assigned_user_id IS NULL AND assigned_team_id IS NOT NULL
ORDER BY c.created_at ASC;

-- name: add-conversation-tags
-- Insert new tags
INSERT INTO conversation_tags (conversation_id, tag_id)
  SELECT c.id, t.id
  FROM conversations c, tags t
  WHERE t.name = ANY($2::text[]) AND c.uuid = $1
  ON CONFLICT (conversation_id, tag_id) DO UPDATE SET tag_id = EXCLUDED.tag_id;

-- name: set-conversation-tags
WITH conversation_id AS (
    SELECT id FROM conversations WHERE uuid = $1
),
-- Insert new tags
inserted AS (
    INSERT INTO conversation_tags (conversation_id, tag_id)
    SELECT conversation_id.id, t.id
    FROM conversation_id, tags t
    WHERE t.name = ANY($2::text[])
    ON CONFLICT (conversation_id, tag_id) DO UPDATE SET tag_id = EXCLUDED.tag_id
)
-- Delete tags that are not in the new list
DELETE FROM conversation_tags
WHERE conversation_id = (SELECT id FROM conversation_id) 
AND tag_id NOT IN (
    SELECT id FROM tags WHERE name = ANY($2::text[])
);

-- name: remove-conversation-tags
-- Delete tags that are not in the new list
DELETE FROM conversation_tags
WHERE conversation_id = (SELECT id FROM conversations WHERE uuid = $1)
AND tag_id IN (
    SELECT id FROM tags WHERE name = ANY($2::text[])
);

-- name: get-conversation-tags
SELECT t.name
FROM conversation_tags ct
JOIN tags t ON ct.tag_id = t.id
WHERE ct.conversation_id = (SELECT id FROM conversations WHERE uuid = $1);

-- name: get-conversation-uuid-from-message-uuid
SELECT c.uuid AS conversation_uuid
FROM conversation_messages m
JOIN conversations c ON m.conversation_id = c.id
WHERE m.uuid = $1;

-- name: unassign-open-conversations
UPDATE conversations
SET assigned_user_id = NULL,
    updated_at = NOW()
WHERE assigned_user_id = $1 AND status_id IN (SELECT id FROM conversation_statuses WHERE category != 'resolved');

-- name: update-conversation-custom-attributes
UPDATE conversations
SET custom_attributes = $2,
    updated_at = NOW()
WHERE uuid = $1;

-- name: update-conversation-waiting-since
UPDATE conversations
SET waiting_since = $2,
    updated_at = NOW()
WHERE uuid = $1;

-- name: update-conversation-reply-timestamps
WITH old AS (
    SELECT first_reply_at IS NULL AS is_first FROM conversations WHERE id = $1
)
UPDATE conversations SET
    first_reply_at = COALESCE(conversations.first_reply_at, $2),
    last_reply_at = $2,
    waiting_since = NULL,
    updated_at = NOW()
FROM old WHERE conversations.id = $1
RETURNING old.is_first AS is_first_reply;

-- name: remove-conversation-assignee
UPDATE conversations
SET
    assigned_user_id = CASE WHEN $2 = 'user' THEN NULL ELSE assigned_user_id END,
    assigned_team_id = CASE WHEN $2 = 'team' THEN NULL ELSE assigned_team_id END,
    updated_at = NOW()
WHERE uuid = $1;

-- name: re-open-conversation
-- Open conversation if it is not already open and unset the assigned user if they are away and reassigning.
UPDATE conversations
SET 
  status_id = (SELECT id FROM conversation_statuses WHERE name = 'Open'),
  snoozed_until = NULL,
  updated_at = NOW(),
  assigned_user_id = CASE
    WHEN EXISTS (
      SELECT 1 FROM users 
      WHERE users.id = conversations.assigned_user_id 
        AND users.availability_status = 'away_and_reassigning'
    ) THEN NULL
    ELSE assigned_user_id
  END
WHERE 
  uuid = $1
  AND status_id IN (
    -- Trashed conversations stay trashed even if a new reply arrives — they're
    -- considered deleted. Spam IS allowed to reopen so a genuine customer reply
    -- can self-correct a false-positive spam classification.
    SELECT id FROM conversation_statuses WHERE name NOT IN ('Open', 'Trashed')
  )

-- name: get-conversation-by-message-id
SELECT
    c.id,
    c.uuid,
    c.assigned_team_id,
    c.assigned_user_id
FROM conversation_messages m
JOIN conversations c ON m.conversation_id = c.id
WHERE m.id = $1;

-- name: delete-conversation
DELETE FROM conversations WHERE uuid = $1;

-- MESSAGE queries.
-- name: delete-message
DELETE FROM conversation_messages WHERE CASE 
    WHEN $1 > 0 THEN id = $1 
    ELSE uuid = $2 
END;

-- name: get-message-source-ids
SELECT 
    source_id
FROM conversation_messages
WHERE conversation_id = $1
AND type in ('incoming', 'outgoing') and private = false
and source_id > ''
ORDER BY id DESC
LIMIT $2;

-- name: get-outgoing-pending-messages
SELECT
    m.id,
    m.created_at,
    m.updated_at,
    m.status,
    m.type,
    m.content,
    m.text_content,
    m.sender_type,
    m.content_type,
    m.conversation_id,
    m.uuid,
    m.private,
    m.sender_type,
    m.sender_id,
    m.meta,
    c.uuid as conversation_uuid,
    m.content_type,
    m.source_id,
    m.meta,
    ARRAY(SELECT jsonb_array_elements_text(m.meta->'cc')) AS cc,
    ARRAY(SELECT jsonb_array_elements_text(m.meta->'bcc')) AS bcc,
    ARRAY(SELECT jsonb_array_elements_text(m.meta->'to')) AS to,
    c.inbox_id,
    c.uuid as conversation_uuid,
    c.subject,
    c.contact_id as message_receiver_id,
    c.subject
FROM conversation_messages m
INNER JOIN conversations c ON c.id = m.conversation_id
WHERE m.status = 'pending' AND m.type = 'outgoing' AND m.private = false
AND NOT(m.id = ANY($1::INT[]))

-- name: get-message
SELECT
    m.id,
    m.created_at,
    m.updated_at,
    m.status,
    m.type,
    m.content,
    m.text_content,
    m.content_type,
    m.conversation_id,
    m.uuid,
    m.private,
    m.sender_type,
    m.sender_id,
    m.meta,
    c.uuid as conversation_uuid,
    u.id AS "author.id",
    u.first_name AS "author.first_name",
    u.last_name AS "author.last_name",
    u.email AS "author.email",
    u.avatar_url AS "author.avatar_url",
    u.availability_status AS "author.availability_status",
    u.type AS "author.type",
    u.last_active_at AS "author.last_active_at",
    COALESCE(
        json_agg(
            json_build_object(
                'name', media.filename,
                'content_type', media.content_type,
                'uuid', media.uuid,
                'size', media.size,
                'content_id', media.content_id,
                'disposition', media.disposition
            ) ORDER BY media.filename
        ) FILTER (WHERE media.id IS NOT NULL),
        '[]'::json
    ) AS attachments
FROM conversation_messages m
INNER JOIN conversations c ON c.id = m.conversation_id
JOIN users u ON m.sender_id = u.id
LEFT JOIN media ON media.model_type = 'messages' AND media.model_id = m.id
WHERE m.uuid = $1
GROUP BY
    m.id, m.created_at, m.updated_at, m.status, m.type, m.content, m.uuid, m.private, m.sender_type, c.uuid,
    u.id, u.first_name, u.last_name, u.email, u.avatar_url, u.availability_status, u.type, u.last_active_at
ORDER BY m.created_at;

-- name: get-messages
SELECT
   COUNT(*) OVER() AS total,
   m.id,
   m.created_at,
   m.updated_at,
   m.status,
   m.type,
   m.content,
   m.text_content,
   m.content_type,
   m.conversation_id,
   m.uuid,
   m.private,
   m.sender_id,
   m.sender_type,
   m.meta,
   $1::uuid AS conversation_uuid,
   u.id AS "author.id",
   u.first_name AS "author.first_name",
   u.last_name AS "author.last_name",
   u.email AS "author.email",
   u.avatar_url AS "author.avatar_url",
   u.availability_status AS "author.availability_status",
   u.type AS "author.type",
   u.last_active_at AS "author.last_active_at",
   COALESCE(
     (SELECT json_agg(
       json_build_object(
         'name', filename,
         'content_type', content_type,
         'uuid', uuid,
         'size', size,
         'content_id', content_id,
         'disposition', disposition
       ) ORDER BY filename
     ) FROM media
     WHERE model_type = 'messages' AND model_id = m.id),
   '[]'::json) AS attachments
FROM conversation_messages m
JOIN users u ON m.sender_id = u.id
WHERE m.conversation_id = (
   SELECT id FROM conversations WHERE uuid = $1 LIMIT 1
)
AND ($2::boolean IS NULL OR m.private = $2)
AND ($3::text[] IS NULL OR m.type::text = ANY($3))
AND (m.meta IS NULL OR NOT COALESCE((m.meta->>'continuity_email')::boolean, false))
ORDER BY m.created_at DESC %s

-- name: insert-message
WITH conversation_id AS (
   SELECT id
   FROM conversations
   WHERE CASE
       WHEN $3 > 0 THEN id = $3
       ELSE uuid = $4
   END
),
inserted_msg AS (
   INSERT INTO conversation_messages (
       "type", status, conversation_id, "content",
       text_content, sender_id, sender_type, private,
       content_type, source_id, meta
   )
   VALUES (
       $1, $2, (SELECT id FROM conversation_id),
       $5, $6, $7, $8, $9, $10, $11, $12
   )
   RETURNING *
)
SELECT * FROM inserted_msg;

-- name: message-exists-by-source-id
SELECT conversation_id
FROM conversation_messages
WHERE source_id = ANY($1::text []);

-- name: update-message-status
update conversation_messages set status = $1, updated_at = NOW() where uuid = $2;

-- name: get-latest-message
SELECT
    m.created_at,
    m.updated_at,
    m.status,
    m.type,
    m.content,
    m.uuid,
    m.private,
    m.sender_id,
    m.sender_type,
    m.meta
FROM conversation_messages m
WHERE m.conversation_id = $1
AND m.type = ANY($2)
AND m.status = ANY($3)
AND m.private = NOT $4
ORDER BY m.created_at DESC
LIMIT 1;

-- name: get-previous-email-messages
-- Last N non-private incoming/outgoing messages in a conversation strictly
-- older than the given message ID. Used by sendOutgoingMessage to build a
-- Gmail-style quoted thread on outbound replies. Activity rows and private
-- notes are excluded so neither leaks to the recipient.
SELECT cm.content, cm.created_at, cm.sender_type, cm.type,
       COALESCE(u.first_name, '') AS sender_first_name,
       COALESCE(u.last_name, '') AS sender_last_name,
       COALESCE(u.email, '')      AS sender_email
FROM conversation_messages cm
LEFT JOIN users u ON cm.sender_id = u.id
WHERE cm.conversation_id = $1
  AND cm.type IN ('incoming', 'outgoing')
  AND cm.private = false
  AND cm.id < $2
ORDER BY cm.created_at DESC
LIMIT $3;

-- name: update-message-source-id
UPDATE conversation_messages SET source_id = $1 WHERE id = $2;

-- name: get-offline-livechat-conversations
SELECT
    c.id,
    c.uuid,
    c.contact_id,
    c.inbox_id,
    c.contact_last_seen_at,
    c.last_continuity_email_sent_at,
    i.linked_email_inbox_id,
    u.email as contact_email,
    u.first_name as contact_first_name,
    u.last_name as contact_last_name,
    c.reference_number,
    c.meta->>'continuity_email_subject' as continuity_email_subject
FROM conversations c
JOIN users u ON u.id = c.contact_id
JOIN inboxes i ON i.id = c.inbox_id
WHERE i.channel = 'livechat'
  AND i.enabled = TRUE
  AND i.linked_email_inbox_id IS NOT NULL
  AND c.contact_last_seen_at IS NOT NULL
  AND c.contact_last_seen_at < NOW() - MAKE_INTERVAL(mins => $1)
  AND EXISTS (
    SELECT 1 FROM conversation_messages cm
    WHERE cm.conversation_id = c.id
      AND cm.created_at > c.contact_last_seen_at
      AND cm.type = 'outgoing'
      AND cm.private = false
      AND (cm.meta IS NULL OR NOT COALESCE((cm.meta->>'continuity_email')::boolean, false))
      AND (cm.meta IS NULL OR NOT COALESCE((cm.meta->>'continuity_emailed')::boolean, false))
  )
  AND u.email > ''
  AND (c.last_continuity_email_sent_at IS NULL
       OR c.last_continuity_email_sent_at < NOW() - MAKE_INTERVAL(mins => $2))
  AND c.inbox_id = $3;

-- name: get-unread-messages
SELECT
    m.id,
    m.created_at,
    m.updated_at,
    m.status,
    m.type,
    m.content,
    m.text_content,
    m.uuid,
    m.private,
    m.sender_id,
    m.sender_type,
    m.meta,
    u.first_name as "sender.first_name",
    u.last_name as "sender.last_name",
    u.type as "sender.type",
    COALESCE((SELECT string_agg(md.filename, ',') FROM media md WHERE md.model_id = m.id AND md.model_type = 'messages'), '') AS attachment_names
FROM conversation_messages m
LEFT JOIN users u ON u.id = m.sender_id
WHERE m.conversation_id = $1
  AND m.created_at > $2
  AND m.type = 'outgoing'
  AND m.private = false
  AND (m.meta IS NULL OR NOT COALESCE((m.meta->>'continuity_email')::boolean, false))
  AND (m.meta IS NULL OR NOT COALESCE((m.meta->>'continuity_emailed')::boolean, false))
ORDER BY m.created_at ASC
LIMIT $3;

-- name: mark-messages-continuity-emailed
UPDATE conversation_messages
SET meta = COALESCE(meta, '{}'::jsonb) || '{"continuity_emailed": true}'::jsonb
WHERE id = ANY($1::int[]);

-- name: update-continuity-email-tracking
UPDATE conversations
SET last_continuity_email_sent_at = NOW(),
    meta = jsonb_set(COALESCE(meta, '{}'::jsonb), '{continuity_email_subject}', to_jsonb($2::text))
WHERE id = $1;

-- name: upsert-conversation-draft
INSERT INTO conversation_drafts (conversation_id, user_id, content, meta, updated_at)
VALUES ($1, $2, $3, $4, NOW())
ON CONFLICT (conversation_id, user_id)
DO UPDATE SET content = EXCLUDED.content, meta = EXCLUDED.meta, updated_at = NOW()
RETURNING *;

-- name: get-all-user-drafts
SELECT cd.id, cd.conversation_id, cd.user_id, cd.content, cd.meta, cd.created_at, cd.updated_at, c.uuid as conversation_uuid
FROM conversation_drafts cd
INNER JOIN conversations c ON cd.conversation_id = c.id
WHERE cd.user_id = $1
ORDER BY cd.updated_at DESC;

-- name: delete-conversation-draft
DELETE FROM conversation_drafts
WHERE conversation_id IN (
  SELECT id FROM conversations
  WHERE ($1 > 0 AND id = $1) OR ($2::uuid IS NOT NULL AND uuid = $2::uuid)
) AND user_id = $3;

-- name: delete-stale-drafts
DELETE FROM conversation_drafts
WHERE created_at < $1;

-- name: insert-mention
INSERT INTO conversation_mentions (conversation_id, message_id, mentioned_user_id, mentioned_team_id, mentioned_by_user_id)
VALUES ($1, $2, $3, $4, $5);

-- name: mark-conversation-unread
INSERT INTO conversation_last_seen (user_id, conversation_id, last_seen_at)
VALUES (
    $1,
    (SELECT id FROM conversations WHERE uuid = $2),
    (SELECT created_at - INTERVAL '1 second' FROM conversation_messages
     WHERE conversation_id = (SELECT id FROM conversations WHERE uuid = $2)
     ORDER BY created_at DESC LIMIT 1)
)
ON CONFLICT (conversation_id, user_id)
DO UPDATE SET
    last_seen_at = (SELECT created_at - INTERVAL '1 second' FROM conversation_messages
                    WHERE conversation_id = (SELECT id FROM conversations WHERE uuid = $2)
                    ORDER BY created_at DESC LIMIT 1),
    updated_at = NOW();

-- name: get-active-livechat-conversations-by-agent
SELECT c.uuid, c.contact_id, c.inbox_id
FROM conversations c
JOIN inboxes i ON i.id = c.inbox_id
JOIN users u ON u.id = c.contact_id
WHERE c.assigned_user_id = $1
  AND i.channel = 'livechat'
  AND i.enabled = TRUE
  AND i.deleted_at IS NULL
  AND u.availability_status = 'online'
ORDER BY c.last_interaction_at DESC
LIMIT 50;

-- name: update-private-note-content
UPDATE conversation_messages
SET content = $2, text_content = $3, updated_at = NOW()
WHERE uuid = $1 AND private = true AND type = 'outgoing';

-- name: soft-delete-private-note
UPDATE conversation_messages
SET content = $2, text_content = $3,
    meta = jsonb_set(COALESCE(meta, '{}')::jsonb, '{deleted}', 'true')::jsonb,
    updated_at = NOW()
WHERE uuid = $1 AND private = true AND type = 'outgoing';

-- name: is-source-id-from-forward
-- Returns true if any of the given non-empty message source IDs (from
-- In-Reply-To / References headers) belongs to a message that was either
-- sent as a forward (meta.forwarded_to set) or is itself flagged as a
-- reply-to-forward (meta.from_forward = true). Used to propagate the
-- from_forward tag through chained replies so internal threads stay
-- internal.
--
-- Empty strings in the input array are filtered out so an empty
-- In-Reply-To value can't false-match a row whose source_id is also empty.
-- The from_forward check uses jsonb containment so it never throws on
-- unexpected meta shapes (a stray string/null wouldn't survive a ::boolean
-- cast).
SELECT EXISTS(
    SELECT 1 FROM conversation_messages
    WHERE source_id IS NOT NULL AND source_id <> ''
      AND source_id = ANY(ARRAY(SELECT s FROM unnest($1::text[]) AS s WHERE s <> ''))
      AND (meta ? 'forwarded_to' OR meta @> '{"from_forward": true}'::jsonb)
) AS is_from_forward;

-- name: contact-has-prior-agent-reply
-- Used by the spam-folder rescue path: if any prior outgoing agent message
-- exists for this contact, the sender is trusted enough to skip the
-- isSpamMailbox auto-spam classification (and instead trigger an IMAP
-- MOVE-back so providers like Gmail learn to deliver to INBOX next time).
SELECT EXISTS (
    SELECT 1
    FROM conversation_messages m
    JOIN conversations c ON c.id = m.conversation_id
    WHERE c.contact_id = $1
      AND m.sender_type = 'agent'
      AND m.type = 'outgoing'
);

-- name: get-latest-incoming-source-id
-- Returns the IMAP Message-ID (source_id) and inbox id of the most recent
-- incoming message for a conversation. Used by the manual "not spam" handler
-- to drive an IMAP MOVE from the spam folder back to INBOX.
SELECT m.source_id, c.inbox_id
FROM conversation_messages m
JOIN conversations c ON c.id = m.conversation_id
WHERE c.uuid = $1
  AND m.type = 'incoming'
  AND m.source_id IS NOT NULL
  AND m.source_id != ''
ORDER BY m.created_at DESC
LIMIT 1;

-- name: auto-trash-old-resolved
-- Aged from updated_at so a reopen-and-re-resolve resets the clock.
UPDATE conversations SET
    status_id = (SELECT id FROM conversation_statuses WHERE name = 'Trashed'),
    trashed_at = NOW(),
    updated_at = NOW()
WHERE status_id IN (SELECT id FROM conversation_statuses WHERE name IN ('Resolved', 'Closed'))
AND updated_at < NOW() - INTERVAL '1 day' * $1
AND trashed_at IS NULL;

-- name: auto-trash-old-spam
-- Aged from created_at: spam doesn't earn a "fresh activity" clock reset.
UPDATE conversations SET
    status_id = (SELECT id FROM conversation_statuses WHERE name = 'Trashed'),
    trashed_at = NOW(),
    updated_at = NOW()
WHERE status_id = (SELECT id FROM conversation_statuses WHERE name = 'Spam')
AND created_at < NOW() - INTERVAL '1 day' * $1;

-- name: purge-old-trash-media
-- Unlink media rows so the periodic media cleaner removes both the DB row AND
-- the underlying storage blob (and any thumbnails). Hard-deleting media here
-- would orphan the files in S3/disk because cleanup would never see them.
UPDATE media SET model_id = 0 WHERE model_type = 'messages' AND model_id IN (
    SELECT id FROM conversation_messages WHERE conversation_id IN (
        SELECT id FROM conversations
        WHERE status_id = (SELECT id FROM conversation_statuses WHERE name = 'Trashed')
        AND trashed_at < NOW() - INTERVAL '1 day' * $1
    )
);

-- name: purge-old-trash
DELETE FROM conversations
WHERE status_id = (SELECT id FROM conversation_statuses WHERE name = 'Trashed')
AND trashed_at < NOW() - INTERVAL '1 day' * $1;

-- name: get-conversations-by-uuids
-- Lightweight fetch for merge: returns only the columns MergeConversations needs
-- (no joins). $1 is a Go pq.Array of UUID strings.
SELECT id, uuid, reference_number, COALESCE(subject, '') AS subject, contact_id
FROM conversations
WHERE uuid = ANY($1::uuid[]);

-- name: shift-secondary-message-timestamps
-- Before moving messages from a secondary into a primary, shift any secondary
-- messages whose created_at is older than the primary's earliest non-activity
-- message so they land *after* the primary's first message but preserve their
-- relative order. Without this, a freshly-merged thread can read time-jumbled
-- (older customer reply appearing before the original ticket).
-- $1 = primary conversation_id, $2 = secondary conversation_id.
WITH primary_first AS (
    SELECT COALESCE(MIN(created_at), NOW()) AS ts
    FROM conversation_messages
    WHERE conversation_id = $1 AND type != 'activity'
),
secondary_msgs AS (
    SELECT id, created_at,
        ROW_NUMBER() OVER (ORDER BY created_at ASC) AS rn
    FROM conversation_messages
    WHERE conversation_id = $2 AND type != 'activity'
        AND created_at < (SELECT ts FROM primary_first)
)
UPDATE conversation_messages cm
SET created_at = (SELECT ts FROM primary_first) - INTERVAL '1 second' * (
    (SELECT COUNT(*) FROM secondary_msgs) - sm.rn
)
FROM secondary_msgs sm
WHERE cm.id = sm.id;

-- name: move-conversation-messages
-- $1 = destination conversation_id, $2 = source conversation_id.
UPDATE conversation_messages SET conversation_id = $1 WHERE conversation_id = $2;

-- name: move-conversation-mentions
-- $1 = destination conversation_id, $2 = source conversation_id.
UPDATE conversation_mentions SET conversation_id = $1 WHERE conversation_id = $2;

-- name: move-conversation-notifications
-- $1 = destination conversation_id, $2 = source conversation_id.
UPDATE user_notifications SET conversation_id = $1 WHERE conversation_id = $2;

-- name: copy-conversation-tags
-- $1 = destination conversation_id, $2 = source conversation_id. Skips duplicate tags.
INSERT INTO conversation_tags (conversation_id, tag_id)
SELECT $1, tag_id FROM conversation_tags WHERE conversation_id = $2
ON CONFLICT (conversation_id, tag_id) DO NOTHING;

-- name: mark-conversation-merged-into
-- Mark a secondary as merged into the given primary. Status close + closed_at
-- bookkeeping is handled separately via UpdateConversationStatus so audit log,
-- webhooks and websockets all fire.
-- $1 = secondary conversation_id, $2 = primary conversation_id.
UPDATE conversations SET
    merged_into_conversation_id = $2,
    merged_at = NOW(),
    updated_at = NOW()
WHERE id = $1;

-- name: any-conversation-open
-- Returns true if any conversation in $1 (UUID array) currently has a status
-- other than Closed/Trashed. Run BEFORE the merge transaction so the result
-- isn't tainted by secondaries we're about to close.
SELECT EXISTS(
    SELECT 1 FROM conversations c
    JOIN conversation_statuses cs ON cs.id = c.status_id
    WHERE c.uuid = ANY($1::uuid[]) AND cs.name NOT IN ('Closed', 'Trashed')
);

-- name: find-conversation-by-subject-contact
-- ER3 fallback: when an incoming reply's In-Reply-To/References don't match
-- any known conversation (typically because Google or another mail relay
-- rewrote the Message-ID), match against the most-recent open conversation
-- with the same contact + inbox + (case-insensitive, Re:/Fw:-stripped)
-- subject. Resolved/Closed/Spam/Trashed conversations are excluded so a stale
-- but technically-matching old ticket doesn't get re-opened.
SELECT c.id
FROM conversations c
WHERE c.contact_id = $1
  AND c.inbox_id = $2
  AND LOWER(REGEXP_REPLACE(c.subject, '^(re|fw|fwd):\s*', '', 'gi')) = LOWER(REGEXP_REPLACE($3, '^(re|fw|fwd):\s*', '', 'gi'))
  AND c.status_id NOT IN (3, 4, 5, 6)
ORDER BY c.created_at DESC
LIMIT 1;

-- name: get-recent-activities
-- Global activity feed (Reports > Recent Activities). Returns activity
-- messages (status changes, assignments) and outgoing agent messages joined
-- to the parent conversation and acting user so the timeline UI can render
-- without N+1 lookups. COUNT(*) OVER() in the same row gives the total for
-- pagination without a second round trip.
SELECT COUNT(*) OVER() as total,
    cm.id, cm.created_at, cm.type, cm.content, cm.sender_type,
    c.uuid as conversation_uuid, c.reference_number, c.subject,
    u.first_name as actor_first_name, u.last_name as actor_last_name,
    u.avatar_url as actor_avatar_url
FROM conversation_messages cm
JOIN conversations c ON c.id = cm.conversation_id
JOIN users u ON u.id = cm.sender_id
WHERE cm.type IN ('activity', 'outgoing')
ORDER BY cm.created_at DESC
LIMIT $1 OFFSET $2;

-- name: purge-old-activities
-- Hourly cleanup pass driven by trash.activity_purge_days. Deletes only
-- activity-type messages (status changes, assignments) older than $1 days,
-- never agent replies or incoming customer messages. Caller skips the call
-- when retention is 0 to disable the pass entirely.
DELETE FROM conversation_messages
WHERE type = 'activity'
AND created_at < NOW() - INTERVAL '1 day' * $1;
