package migrations

import (
	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

// V2_2_11 adds the `color` column to `teams` for UX26 (team badge colour
// alternative to emoji). Stored as TEXT holding a `#rrggbb` hex string. When
// set, the team renders as a coloured square containing the first letter of
// the team name; emoji takes precedence if both are set so existing emoji-
// configured teams continue to render unchanged.
//
// A CHECK constraint enforces the hex shape so the frontend colour picker
// can't smuggle non-hex values past the API. The schema.sql baseline carries
// the same constraint so fresh installs match.
//
// Idempotent: ADD COLUMN IF NOT EXISTS guards re-runs. The CHECK is added
// separately and tolerates an already-present constraint with the same name.
func V2_2_11(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf) error {
	if _, err := db.Exec(`
		ALTER TABLE teams
		ADD COLUMN IF NOT EXISTS color TEXT NULL
	`); err != nil {
		return err
	}

	// Add the CHECK constraint only if it doesn't already exist. PG doesn't
	// have ADD CONSTRAINT IF NOT EXISTS, so we look it up in pg_constraint
	// first.
	_, err := db.Exec(`
		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM pg_constraint
				WHERE conname = 'constraint_teams_on_color'
			) THEN
				ALTER TABLE teams
				ADD CONSTRAINT constraint_teams_on_color
				CHECK (color IS NULL OR color ~ '^#[0-9a-fA-F]{6}$');
			END IF;
		END$$
	`)
	return err
}
