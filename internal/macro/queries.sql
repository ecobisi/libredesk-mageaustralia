-- name: get
SELECT
    id,
    created_at,
    updated_at,
    name,
    actions,
    visibility,
    visible_when,
    message_content,
    user_id,
    team_id,
    usage_count
FROM
    macros
WHERE
    id = $1;

-- name: get-all
SELECT
    id,
    created_at,
    updated_at,
    name,
    actions,
    visibility,
    visible_when,
    message_content,
    user_id,
    team_id,
    usage_count
FROM
    macros
ORDER BY
    updated_at DESC;

-- name: get-all-for-user
-- Returns every macro joined with the calling agent's per-user usage row,
-- so the picker can sort by THIS user's last_used_at (most-recently-used
-- first, NULLs last for macros they've never applied), then alphabetical
-- by name as a stable tiebreaker.
SELECT
    m.id,
    m.created_at,
    m.updated_at,
    m.name,
    m.actions,
    m.visibility,
    m.visible_when,
    m.message_content,
    m.user_id,
    m.team_id,
    m.usage_count,
    u.last_used_at
FROM
    macros m
LEFT JOIN
    macro_user_usage u ON u.macro_id = m.id AND u.user_id = $1
ORDER BY
    u.last_used_at DESC NULLS LAST,
    m.name ASC;

-- name: mark-used
-- Upsert that bumps the per-user last_used_at + use_count when an agent
-- applies a macro. Drives the MRU sort in get-all-for-user.
INSERT INTO macro_user_usage (user_id, macro_id, last_used_at, use_count)
VALUES ($1, $2, NOW(), 1)
ON CONFLICT (user_id, macro_id)
DO UPDATE SET
    last_used_at = NOW(),
    use_count = macro_user_usage.use_count + 1;

-- name: create
INSERT INTO
    macros (name, message_content, user_id, team_id, visibility, visible_when, actions)
VALUES
    ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: update
UPDATE
    macros
SET
    name = $2,
    message_content = $3,
    user_id = $4,
    team_id = $5,
    visibility = $6,
    visible_when = $7,
    actions = $8,
    updated_at = NOW()
WHERE
    id = $1
RETURNING *;

-- name: delete
DELETE FROM
    macros
WHERE
    id = $1;

-- name: increment-usage-count
UPDATE
    macros
SET
    usage_count = usage_count + 1
WHERE
    id = $1;