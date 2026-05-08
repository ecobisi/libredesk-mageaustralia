package conversation

import (
	"strings"
	"testing"

	pciscrub "github.com/mageaustralia/go-pci-scrub"
)

// TestPCIScrubBehaviour locks in the contract our redaction pipeline depends
// on: that pciscrub.Scrub() actually masks the digits of a credit card while
// leaving surrounding text intact, and that ScrubWithSpans() reports
// non-empty Spans for input that contains card data so InsertMessage knows
// to flag the row.
//
// This is intentionally a thin smoke test — go-pci-scrub has its own test
// suite for the full Luhn/format coverage. We only verify the integration
// surface our code consumes.
func TestPCIScrubBehaviour(t *testing.T) {
	// 4111111111111111 is a Visa test number that passes the Luhn check;
	// pciscrub should detect + scrub it.
	const cardText = "Order placed by Jane. Card 4111-1111-1111-1111 exp 12/29."

	scrubbed := pciscrub.Scrub(cardText)
	if strings.Contains(scrubbed, "4111111111111111") {
		t.Errorf("expected card number digits to be scrubbed, got: %q", scrubbed)
	}
	if strings.Contains(scrubbed, "4111-1111-1111-1111") {
		t.Errorf("expected dash-separated card number to be scrubbed, got: %q", scrubbed)
	}
	if !strings.Contains(scrubbed, "Jane") {
		t.Errorf("expected non-card text to survive scrub, got: %q", scrubbed)
	}

	result := pciscrub.ScrubWithSpans(cardText)
	if len(result.Spans) == 0 {
		t.Errorf("expected ScrubWithSpans to detect at least one PCI span; got 0")
	}

	// Negative case: plain text without card data must not flag.
	plain := "Hello team, please ignore my last message."
	resultPlain := pciscrub.ScrubWithSpans(plain)
	if len(resultPlain.Spans) != 0 {
		t.Errorf("expected zero spans for non-card text; got %d", len(resultPlain.Spans))
	}
	if pciscrub.Scrub(plain) != plain {
		t.Errorf("expected non-card text to round-trip unchanged through Scrub")
	}
}
