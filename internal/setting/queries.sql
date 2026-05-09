-- name: get-all
SELECT JSON_OBJECT_AGG(key, value) AS settings FROM (SELECT * FROM settings ORDER BY key) t;

-- name: update
UPDATE settings AS s
SET value = c.value,
    updated_at = now()
FROM (SELECT * FROM jsonb_each($1)) AS c(key, value)
WHERE s.key = c.key;

-- name: get-by-prefix
SELECT JSON_OBJECT_AGG(key, value) AS settings 
FROM settings 
WHERE key LIKE $1 || '%';

-- name: get
SELECT value FROM settings WHERE key = $1;

-- name: get-inbox-ai-settings
SELECT id, created_at, updated_at, inbox_id, system_prompt, max_context_chunks,
    similarity_threshold, external_search_enabled, external_search_url,
    external_search_max_results, external_search_endpoints, external_search_headers,
    knowledge_source_ids
FROM inbox_ai_settings WHERE inbox_id = $1;

-- name: upsert-inbox-ai-settings
INSERT INTO inbox_ai_settings (inbox_id, system_prompt, max_context_chunks, similarity_threshold,
    external_search_enabled, external_search_url, external_search_max_results,
    external_search_endpoints, external_search_headers, knowledge_source_ids, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW())
ON CONFLICT (inbox_id) DO UPDATE SET
    system_prompt = EXCLUDED.system_prompt,
    max_context_chunks = EXCLUDED.max_context_chunks,
    similarity_threshold = EXCLUDED.similarity_threshold,
    external_search_enabled = EXCLUDED.external_search_enabled,
    external_search_url = EXCLUDED.external_search_url,
    external_search_max_results = EXCLUDED.external_search_max_results,
    external_search_endpoints = EXCLUDED.external_search_endpoints,
    external_search_headers = EXCLUDED.external_search_headers,
    knowledge_source_ids = EXCLUDED.knowledge_source_ids,
    updated_at = NOW()
RETURNING id, created_at, updated_at, inbox_id, system_prompt, max_context_chunks,
    similarity_threshold, external_search_enabled, external_search_url,
    external_search_max_results, external_search_endpoints, external_search_headers,
    knowledge_source_ids;

-- name: delete-inbox-ai-settings
DELETE FROM inbox_ai_settings WHERE inbox_id = $1;