package migrations

import (
	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

// V2_2_9 seeds the trash.activity_purge_days retention setting that drives
// the hourly activity-message purge added with Reports > Recent Activities
// (RA1). Default is 7 days; admins can set 0 to disable. Idempotent.
func V2_2_9(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf) error {
	if _, err := db.Exec(
		`INSERT INTO settings (key, value) VALUES ('trash.activity_purge_days', '7'::jsonb)
		ON CONFLICT (key) DO NOTHING`,
	); err != nil {
		return err
	}
	return nil
}
