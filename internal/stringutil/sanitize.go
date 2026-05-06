package stringutil

import (
	"regexp"
	"strings"

	"github.com/microcosm-cc/bluemonday"
)

// notePolicy is a shared bluemonday policy for sanitizing user-generated HTML notes.
// Allows basic formatting but strips dangerous elements (script, iframe, form, etc.).
var notePolicy = bluemonday.UGCPolicy()

// SanitizeHTML sanitizes user-generated HTML content, allowing safe formatting
// tags while stripping potentially dangerous elements and attributes.
func SanitizeHTML(s string) string {
	return notePolicy.Sanitize(s)
}

// Email-HTML cleanup regexes. These tidy whitespace gunk from email clients
// (especially Outlook). They are NOT a security boundary; they only remove
// empty/redundant elements for cleaner display. Bluemonday is deliberately
// not applied to incoming email HTML: UGCPolicy strips inline styles,
// classes, and many table attributes that legitimate email markup relies on.
var (
	// Two or more consecutive <br> tags (with optional whitespace/attributes).
	emailMultipleBrRegex = regexp.MustCompile(`(<br\s*/?>\s*){2,}`)

	// Empty <div> with no inner content.
	emailEmptyDivRegex = regexp.MustCompile(`<div[^>]*>\s*</div>`)

	// <div> containing only whitespace and/or a single <br>.
	emailWhitespaceDivRegex = regexp.MustCompile(`<div[^>]*>\s*(<br\s*/?>)?\s*</div>`)

	// Three or more consecutive newlines.
	emailMultipleNewlinesRegex = regexp.MustCompile(`\n{3,}`)

	// Outlook's empty class="elementToProof" wrapper divs.
	emailOutlookEmptyRegex = regexp.MustCompile(`<div[^>]*class="elementToProof"[^>]*>\s*(<br\s*/?>)?\s*</div>`)
)

// SanitizeEmailHTML cleans up messy HTML produced by email clients
// (notably Outlook) before storing the message body. It removes excessive
// whitespace, empty divs, and runs of <br> tags so the rendered conversation
// thread reads cleanly. Not a security sanitiser; see SanitizeHTML for that.
func SanitizeEmailHTML(html string) string {
	html = emailOutlookEmptyRegex.ReplaceAllString(html, "")
	html = emailEmptyDivRegex.ReplaceAllString(html, "")
	html = emailWhitespaceDivRegex.ReplaceAllString(html, "")
	html = emailMultipleBrRegex.ReplaceAllString(html, "<br>")
	html = emailMultipleNewlinesRegex.ReplaceAllString(html, "\n\n")
	return strings.TrimSpace(html)
}
