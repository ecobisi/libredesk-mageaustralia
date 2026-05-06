package migrations

import (
	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

// V2_2_7 adds per-user macro usage tracking (MP6) so each agent's macro
// picker can sort by their own most-recently-used order. The existing
// macros.usage_count column tracks GLOBAL usage (used by the RAG sync /
// reporting paths), which is fine for "what's the most popular macro
// across the team" but useless for "what did THIS agent use last".
//
// Idempotent: CREATE TABLE / INDEX use IF NOT EXISTS so re-runs are safe.
func V2_2_7(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf) error {
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS macro_user_usage (
			user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			macro_id INT NOT NULL REFERENCES macros(id) ON DELETE CASCADE,
			last_used_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			use_count INT NOT NULL DEFAULT 0,
			PRIMARY KEY (user_id, macro_id)
		)
	`); err != nil {
		return err
	}

	if _, err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_macro_user_usage_user_recent
			ON macro_user_usage(user_id, last_used_at DESC)
	`); err != nil {
		return err
	}

	return nil
}
