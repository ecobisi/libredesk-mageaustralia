package migrations

import (
	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

// V2_2_14 inserts default rows for the T3v voicemail-transcription settings.
// The setting manager's update statement is a plain UPDATE (not UPSERT) so
// every settings key the UI saves must exist in the table beforehand —
// without these rows the admin form silently no-ops and the toggle never
// persists across restarts.
//
// Defaults match a fresh install state: transcription disabled, "local"
// provider pre-selected (the cheaper / private path) so flipping the
// toggle doesn't immediately demand an OpenAI key. Both keys are added
// idempotently via ON CONFLICT DO NOTHING for re-runs.
func V2_2_14(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf) error {
	if _, err := db.Exec(`
		INSERT INTO settings (key, value) VALUES
			('ai.transcription_enabled', 'false'::jsonb),
			('ai.transcription_provider', '"local"'::jsonb)
		ON CONFLICT (key) DO NOTHING
	`); err != nil {
		return err
	}
	return nil
}
