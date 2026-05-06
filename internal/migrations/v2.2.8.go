package migrations

import (
	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

// V2_2_8 adds the `sending` value to the message_status enum so the outgoing
// message dispatcher can mark a row in-flight before the SMTP call returns
// (IP3). This closes a duplicate-send race where a slow SMTP would let the
// 50ms pending-scanner pick the same conversation_messages row up twice. The
// in-memory exclusion map alone does not cover process restarts mid-send.
//
// Idempotent: ADD VALUE IF NOT EXISTS is safe to re-run.
func V2_2_8(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf) error {
	if _, err := db.Exec(`ALTER TYPE message_status ADD VALUE IF NOT EXISTS 'sending';`); err != nil {
		return err
	}
	return nil
}
