package migrations

import (
	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

// V2_2_16 stands up T3b's OpenRouter provider support.
//
// Two pieces:
//
//  1. Extend the `ai_provider` Postgres enum to include 'openrouter'.
//     The enum was created in the v0.x line with only ('openai') as a
//     value; the upsert in step 2 fails INSERT validation without this.
//     ALTER TYPE … ADD VALUE IF NOT EXISTS is idempotent and survives
//     re-runs of the migration.
//
//  2. Seed an OpenRouter provider row with an empty api_key + the
//     default Haiku 3 model. ON CONFLICT (name) DO NOTHING means
//     existing installs that have already saved an OpenRouter config
//     (from a manual upsert via the AISettings UI) keep their values.
//
// Encryption-at-rest for the OpenRouter api_key was added in T3j
// (mirroring the existing OpenAI encryption path). The empty seed value
// here is encryption-agnostic — setOpenRouterConfig encrypts non-empty
// keys when the admin first saves a value. No migration is required to
// re-encrypt rows because seeded rows ship with `api_key: ""`.
//
// schema.sql carries matching baseline (enum value + seed row) so fresh
// installs converge with upgraded ones.
func V2_2_16(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf) error {
	// 1. Extend the ai_provider enum. Postgres requires ALTER TYPE …
	//    ADD VALUE to run outside a transaction block — sqlx.DB.Exec
	//    runs each statement standalone, so the implicit autocommit
	//    here is fine.
	if _, err := db.Exec(`ALTER TYPE ai_provider ADD VALUE IF NOT EXISTS 'openrouter'`); err != nil {
		return err
	}

	// 2. Seed the OpenRouter row. The AISettings UI's upsert path will
	//    also do this, but seeding here means a freshly-upgraded
	//    install shows OpenRouter as a "Not configured" tile in the
	//    admin UI right away — no need to save once just to populate
	//    the row.
	if _, err := db.Exec(`
		INSERT INTO ai_providers (name, provider, config, is_default)
		VALUES ('OpenRouter', 'openrouter', '{"api_key": "", "model": "anthropic/claude-3-haiku"}'::jsonb, false)
		ON CONFLICT (name) DO NOTHING
	`); err != nil {
		return err
	}

	return nil
}
