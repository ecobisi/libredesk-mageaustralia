package migrations

import (
	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

// V2_2_4 adds the `color` column to conversation_statuses for FS17 (status
// colour pills). Stored as TEXT (palette key like "gray", "orange", "blue")
// rather than an ENUM so the frontend palette can change without schema work.
//
// The four built-in statuses are seeded to match the previously-hardcoded
// frontend palette (Open=orange, Snoozed=purple, Resolved=green,
// Closed=gray) so admins see the same pill colours after upgrading. Existing
// admin-created statuses get the column default ("gray") and can be
// recoloured from the admin UI's inline picker.
//
// Idempotent: ADD COLUMN IF NOT EXISTS guards re-runs.
func V2_2_4(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf) error {
	if _, err := db.Exec(`
		ALTER TABLE conversation_statuses
		ADD COLUMN IF NOT EXISTS color TEXT NOT NULL DEFAULT 'gray'
	`); err != nil {
		return err
	}

	// Seed the built-in defaults. Only updates rows that still have the
	// column default ('gray') so we don't clobber an admin who has already
	// recoloured a built-in status post-migration. Spam/Trashed stay 'gray'.
	if _, err := db.Exec(`
		UPDATE conversation_statuses SET color = 'orange'
		WHERE name = 'Open' AND color = 'gray'
	`); err != nil {
		return err
	}
	if _, err := db.Exec(`
		UPDATE conversation_statuses SET color = 'purple'
		WHERE name = 'Snoozed' AND color = 'gray'
	`); err != nil {
		return err
	}
	if _, err := db.Exec(`
		UPDATE conversation_statuses SET color = 'green'
		WHERE name = 'Resolved' AND color = 'gray'
	`); err != nil {
		return err
	}
	return nil
}
