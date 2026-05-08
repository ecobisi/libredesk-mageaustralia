package migrations

import (
	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

// V2_2_12 adds PCI redaction tracking columns + supporting partial index for
// T3y. Columns mirror the schema.sql baseline so fresh installs and upgraded
// instances converge on the same shape.
//
// has_pci_data is the agent-UI banner trigger: set by the on-ingest go-pci-
// scrub pass, cleared by manual or auto redaction. pci_detected_at backs the
// 7-day auto-redact safety net (the conversation/pci_redact.go loop selects
// rows older than 7 days and scrubs them).
//
// The partial index keeps the auto-redact scan O(rows_with_pci_data) rather
// than scanning the whole conversation_messages table — for a typical
// instance only a handful of messages will ever carry the flag.
//
// Idempotent: ADD COLUMN IF NOT EXISTS + CREATE INDEX IF NOT EXISTS guard
// re-runs.
func V2_2_12(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf) error {
	if _, err := db.Exec(`
		ALTER TABLE conversation_messages
		ADD COLUMN IF NOT EXISTS has_pci_data BOOLEAN DEFAULT FALSE NOT NULL,
		ADD COLUMN IF NOT EXISTS pci_detected_at TIMESTAMPTZ NULL
	`); err != nil {
		return err
	}

	if _, err := db.Exec(`
		CREATE INDEX IF NOT EXISTS index_conversation_messages_on_has_pci_data
		ON conversation_messages (has_pci_data, pci_detected_at)
		WHERE has_pci_data = true
	`); err != nil {
		return err
	}

	return nil
}
