package migrations

import (
	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

// V2_2_13 creates the user_push_tokens table for T3ab — storage for per-agent
// FCM device tokens used by the mobile app's push notification flow. The FCM
// dispatcher itself is deferred to T3ad, but the table + register/unregister
// endpoints land here so the mobile client has somewhere to deposit tokens
// before the sender goes in.
//
// Schema mirrors what v1.0.3 was running in production (the upstream commit
// d4f953b1 added the handlers but never wrote the table into schema.sql —
// it was created ad hoc via psql). Captured here:
//   - (user_id, token) unique so the same device re-registering is an UPSERT
//   - platform CHECK constrained to 'android' / 'ios' (mirrors handler validation)
//   - ON DELETE CASCADE on user_id so deleting an agent reaps their tokens
//   - btree index on user_id for the dispatcher's per-user lookup hot path
//
// Idempotent: CREATE TABLE / INDEX IF NOT EXISTS guards re-runs.
func V2_2_13(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf) error {
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS user_push_tokens (
			id         SERIAL PRIMARY KEY,
			user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			token      TEXT NOT NULL,
			platform   TEXT NOT NULL CHECK (platform IN ('android', 'ios')),
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)
	`); err != nil {
		return err
	}

	if _, err := db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS user_push_tokens_user_id_token_key
		ON user_push_tokens (user_id, token)
	`); err != nil {
		return err
	}

	if _, err := db.Exec(`
		CREATE INDEX IF NOT EXISTS index_user_push_tokens_on_user_id
		ON user_push_tokens (user_id)
	`); err != nil {
		return err
	}

	return nil
}
