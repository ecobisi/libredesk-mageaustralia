package migrations

import (
	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

// V2_2_6 wraps the bare ticket reference number in built-in notification
// email templates with a clickable link to the conversation (FS19). Without
// this, agents see "#1234" as plain text and have to hop into the app and
// search by reference; the linked form drops them straight onto the ticket.
//
// Three built-in templates are touched:
//   - "SLA breach warning"      -> /inboxes/assigned/conversation/<uuid>
//   - "SLA breached"            -> /inboxes/assigned/conversation/<uuid>
//   - "Mentioned in conversation" -> /inboxes/mentioned/conversation/<uuid>
//
// Idempotent and conservative: each UPDATE matches only on the exact
// pre-link substring, so admins who have already customised their template
// (or upgraded once already) get no second-write. is_builtin = true scopes
// the search to seeded rows so a user-created template that happens to
// contain the same wording is left alone.
func V2_2_6(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf) error {
	// SLA approaching warning.
	if _, err := db.Exec(`
		UPDATE templates
		SET body = REPLACE(
			body,
			'<p>This is a notification that the SLA for conversation {{ .Conversation.ReferenceNumber }} is approaching the SLA deadline for {{ .SLA.Metric }}.</p>',
			'<p>This is a notification that the SLA for conversation <a href="{{ RootURL }}/inboxes/assigned/conversation/{{ .Conversation.UUID }}">#{{ .Conversation.ReferenceNumber }}</a> is approaching the SLA deadline for {{ .SLA.Metric }}.</p>'
		)
		WHERE is_builtin = true
		  AND name = 'SLA breach warning'
		  AND body LIKE '%<p>This is a notification that the SLA for conversation {{ .Conversation.ReferenceNumber }} is approaching%'
	`); err != nil {
		return err
	}

	// SLA breached urgent alert.
	if _, err := db.Exec(`
		UPDATE templates
		SET body = REPLACE(
			body,
			'<p>This is an urgent alert that the SLA for conversation {{ .Conversation.ReferenceNumber }} has been breached for {{ .SLA.Metric }}. Please take immediate action.</p>',
			'<p>This is an urgent alert that the SLA for conversation <a href="{{ RootURL }}/inboxes/assigned/conversation/{{ .Conversation.UUID }}">#{{ .Conversation.ReferenceNumber }}</a> has been breached for {{ .SLA.Metric }}. Please take immediate action.</p>'
		)
		WHERE is_builtin = true
		  AND name = 'SLA breached'
		  AND body LIKE '%<p>This is an urgent alert that the SLA for conversation {{ .Conversation.ReferenceNumber }} has been breached%'
	`); err != nil {
		return err
	}

	// Mention in private note.
	if _, err := db.Exec(`
		UPDATE templates
		SET body = REPLACE(
			body,
			'<p>{{ .MentionedBy.FullName }} mentioned you in a private note on conversation #{{ .Conversation.ReferenceNumber }}.</p>',
			'<p>{{ .MentionedBy.FullName }} mentioned you in a private note on conversation <a href="{{ RootURL }}/inboxes/mentioned/conversation/{{ .Conversation.UUID }}">#{{ .Conversation.ReferenceNumber }}</a>.</p>'
		)
		WHERE is_builtin = true
		  AND name = 'Mentioned in conversation'
		  AND body LIKE '%<p>{{ .MentionedBy.FullName }} mentioned you in a private note on conversation #{{ .Conversation.ReferenceNumber }}.</p>%'
	`); err != nil {
		return err
	}

	return nil
}
