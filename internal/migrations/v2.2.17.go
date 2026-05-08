package migrations

import (
	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

// V2_2_17 seeds the T3d external-search-API integration settings rows so
// the partial-save merge in handleUpdateAISettings has rows to UPDATE.
// Like the rest of the AI settings keys, all five default to "off" /
// empty so an upgrade preserves prior behaviour — RAG continues to use
// only pgvector context until an admin enables external search via
// Admin → AI.
//
// schema.sql carries matching baseline rows so fresh installs converge
// with upgraded ones.
func V2_2_17(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf) error {
	if _, err := db.Exec(`
		INSERT INTO settings (key, value) VALUES
			('ai.external_search_enabled', 'false'::jsonb),
			('ai.external_search_url', '""'::jsonb),
			('ai.external_search_max_results', '3'::jsonb),
			('ai.external_search_endpoints', '""'::jsonb),
			('ai.external_search_headers', '""'::jsonb)
		ON CONFLICT (key) DO NOTHING
	`); err != nil {
		return err
	}
	return nil
}
