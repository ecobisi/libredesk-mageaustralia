-- name: get-default-provider
SELECT id, name, provider, config, is_default FROM ai_providers where is_default is true;

-- name: get-prompt
SELECT id, created_at, updated_at, key, title, content FROM ai_prompts where key = $1;

-- name: get-prompts
SELECT id, created_at, updated_at, key, title FROM ai_prompts order by title;

-- name: set-openai-key
UPDATE ai_providers
SET config = jsonb_set(
    COALESCE(config, '{}'::jsonb),
    '{api_key}',
    to_jsonb($1::text)
)
WHERE provider = 'openai';

-- name: get-provider
-- T3a: Look up a provider row by provider name (e.g. 'openai'). Used by
-- the embedding pipeline which always reaches for OpenAI directly
-- regardless of the default-provider setting (Whisper/embeddings only
-- ship via OpenAI proper, not via OpenRouter or other relays).
SELECT id, name, provider, config, is_default FROM ai_providers WHERE provider = $1;

-- name: get-providers
-- T3b: Full list of provider rows for the AISettings admin UI. Returned
-- shape (sans config) is mapped to ProviderInfo so the frontend gets
-- has_api_key / model / is_default without ever seeing the API key
-- itself.
SELECT id, name, provider, config, is_default FROM ai_providers ORDER BY name;

-- name: set-default-provider
-- T3b: Atomically flip is_default to whichever provider matches $1.
-- Relies on the partial unique index (is_default = true) — the UPDATE
-- can't violate uniqueness because both sides of the toggle happen in
-- one statement.
UPDATE ai_providers SET is_default = (provider = $1), updated_at = NOW();

-- name: upsert-openrouter
-- T3b: Make sure the OpenRouter row exists before set-openrouter-config
-- runs an UPDATE on it. Fresh installs get the row from schema.sql, but
-- existing v1.0.x databases predate OpenRouter — this idempotently adds
-- the row without stomping on the OpenAI default flag.
INSERT INTO ai_providers (name, provider, config, is_default)
VALUES ('OpenRouter', 'openrouter', '{"api_key": "", "model": "anthropic/claude-3-haiku"}'::jsonb, false)
ON CONFLICT (name) DO NOTHING;

-- name: set-openrouter-config
-- T3b: Save the OpenRouter API key + model. Empty $1 (api_key) means
-- "preserve the existing key" so the admin can change just the model
-- without re-entering the key. As of T3j the api_key is stored
-- encrypted at rest (crypto.Encrypt in setOpenRouterConfig); the empty
-- string sentinel is checked BEFORE encryption, so this CASE still
-- works as designed.
UPDATE ai_providers
SET config = jsonb_build_object(
    'api_key', CASE WHEN $1::text = '' THEN config->>'api_key' ELSE $1::text END,
    'model', $2::text
),
    updated_at = NOW()
WHERE provider = 'openrouter';
