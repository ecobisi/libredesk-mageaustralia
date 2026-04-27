package migrations

import (
	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

// V1_0_6 adds the `sending` value to the message_status enum so the
// outgoing-message dispatcher can mark a row as in-flight before SMTP and
// prevent the pending-scanner from re-picking it up.
func V1_0_6(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf) error {
	_, err := db.Exec(`
		ALTER TYPE message_status ADD VALUE IF NOT EXISTS 'sending';
	`)
	return err
}
