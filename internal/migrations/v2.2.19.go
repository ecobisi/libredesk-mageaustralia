package migrations

import (
	"github.com/abhinavxd/libredesk/internal/rag"
	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

// V2_2_19 seeds the `ai.system_prompt` setting with an example template
// (rag.DefaultSystemPrompt) so admins see a documented starting point
// in /admin/ai instead of an empty textarea.
//
// Idempotent + non-destructive:
//   - Only updates rows whose current value is the empty-string sentinel
//     ('""' as JSON). If an admin has set their own prompt, it's preserved.
//   - If the row doesn't exist (shouldn't happen — v2.2.15 inserted it),
//     this migration is still a no-op rather than recreating it; the
//     row's existence is the v2.2.15 contract and would be a bug elsewhere.
//
// The runtime fallback in cmd/rag.go references the same constant
// (rag.DefaultSystemPrompt), so an admin who clears the field back to
// empty still gets the same example behaviour.
func V2_2_19(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf) error {
	// Only overwrite the empty-string default. ::text on `value` because
	// settings.value is jsonb and we want literal string equality, not
	// JSON-path comparison.
	_, err := db.Exec(`
		UPDATE settings
		SET value = to_jsonb($1::text), updated_at = NOW()
		WHERE key = 'ai.system_prompt'
		  AND value::text = '""'
	`, rag.DefaultSystemPrompt)
	return err
}
