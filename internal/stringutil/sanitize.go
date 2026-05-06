package stringutil

import (
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
