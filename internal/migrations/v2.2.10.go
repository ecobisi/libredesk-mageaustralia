package migrations

import (
	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

// V2_2_10 seeds the "Added as follower" email template (UX5). Sent to an
// agent when another agent adds them as a follower / watcher on a
// conversation. The template receives Conversation + Recipient + Author
// data, mirroring the shape used by the existing TmplNewReply / TmplMentioned
// renderers.
//
// Idempotent: insert only when no row with the same name exists. An admin
// who has already customised this template (or migrated it across from a
// v1.0.3 export) keeps their version untouched on re-run.
func V2_2_10(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf) error {
	const subject = `{{ .Author.FullName }} added you as a follower on #{{ .Conversation.ReferenceNumber }}`
	const body = `<p>Hi {{ .Recipient.FirstName }},</p>
<p><strong>{{ .Author.FullName }}</strong> added you as a follower on <a href="{{ RootURL }}/inboxes/assigned/conversation/{{ .Conversation.UUID }}"><strong>#{{ .Conversation.ReferenceNumber }}</strong></a>.</p>
<p><strong>Subject:</strong> {{ .Conversation.Subject }}</p>
<p>You will now receive notifications when there is new activity on this conversation.</p>
<p style="margin-top: 16px;">
    <a href="{{ RootURL }}/inboxes/assigned/conversation/{{ .Conversation.UUID }}">View Conversation</a>
</p>`

	// `templates.name` has no unique constraint, so we can't use ON CONFLICT.
	// Insert only when the template doesn't already exist — re-runs of this
	// migration on a DB that already has it become a silent no-op.
	_, err := db.Exec(`
		INSERT INTO templates ("type", name, subject, body, is_default, is_builtin)
		SELECT 'email_notification'::template_type, $1, $2, $3, false, true
		WHERE NOT EXISTS (SELECT 1 FROM templates WHERE name = $1)
	`, "Added as follower", subject, body)
	return err
}
