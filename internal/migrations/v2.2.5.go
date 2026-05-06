package migrations

import (
	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

// V2_2_5 adds the `country` column to activity_logs for FS18 (country flags
// in the activity log). Stored as a 2-letter ISO 3166-1 alpha-2 code so the
// frontend can render an emoji flag (each code maps to a regional indicator
// pair in the Unicode range starting at U+1F1E6). NOT NULL with a '' default
// so existing rows back-fill cleanly and the insert path can stay positional.
//
// The country code is read from the `CF-IPCountry` request header (set by
// Cloudflare); deployments not behind Cloudflare will get '' and render no
// flag, which is the intended graceful-degradation behaviour.
//
// Idempotent: ADD COLUMN IF NOT EXISTS guards re-runs.
func V2_2_5(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf) error {
	if _, err := db.Exec(`
		ALTER TABLE activity_logs
		ADD COLUMN IF NOT EXISTS country VARCHAR(2) NOT NULL DEFAULT ''
	`); err != nil {
		return err
	}
	return nil
}
