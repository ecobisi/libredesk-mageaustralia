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
