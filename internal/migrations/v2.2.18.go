package migrations

import (
	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

// V2_2_18 stands up the inbox_ai_settings table for T3h per-inbox AI
// settings overrides. Each row holds a subset of the global
// AISettings (system prompt, RAG tuning, external-search config) plus
// a JSON array of knowledge_source_ids that scopes RAG search to a
// curated set of sources for the inbox. Absence of a row means the
// global settings apply (existing pre-T3h behaviour).
//
// Idempotent — uses CREATE TABLE IF NOT EXISTS and CREATE INDEX IF NOT
// EXISTS so re-runs on already-migrated DBs are no-ops. The unique
// constraint on inbox_id matches the upsert path's ON CONFLICT
// target. ON DELETE CASCADE on the FK ensures the override row is
// removed if the inbox itself is deleted, so dangling per-inbox rows
// can never accumulate.
//
// schema.sql carries matching baseline DDL so fresh installs converge
// with upgraded ones.
func V2_2_18(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf) error {
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS inbox_ai_settings (
			id SERIAL PRIMARY KEY,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW(),
			inbox_id INT NOT NULL REFERENCES inboxes(id) ON DELETE CASCADE,
			system_prompt TEXT NOT NULL DEFAULT '',
			max_context_chunks INT NOT NULL DEFAULT 5,
			similarity_threshold DOUBLE PRECISION NOT NULL DEFAULT 0.25,
			external_search_enabled BOOLEAN NOT NULL DEFAULT FALSE,
			external_search_url TEXT NOT NULL DEFAULT '',
			external_search_max_results INT NOT NULL DEFAULT 3,
			external_search_endpoints TEXT NOT NULL DEFAULT '',
			external_search_headers TEXT NOT NULL DEFAULT '',
			knowledge_source_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
			CONSTRAINT unique_inbox_ai_settings UNIQUE (inbox_id)
		)
	`); err != nil {
		return err
	}

	if _, err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_inbox_ai_settings_inbox_id
		ON inbox_ai_settings(inbox_id)
	`); err != nil {
		return err
	}

	return nil
}
