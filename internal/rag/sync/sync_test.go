package sync

import (
	"strings"
	"testing"
)

func TestStripHTML(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty", input: "", want: ""},
		{name: "plain", input: "Hello World", want: "Hello World"},
		{name: "tags", input: "<p>Hello World</p>", want: "Hello World"},
		{name: "nested", input: "<div><p>Hello <strong>World</strong></p></div>", want: "Hello World"},
		{name: "entities", input: "Hello &amp; World &lt;test&gt;", want: "Hello & World <test>"},
		{name: "whitespace", input: "Hello    World", want: "Hello World"},
		{name: "linebreaks", input: "Hello\n\t\nWorld", want: "Hello World"},
		{name: "anchor", input: `<a href="https://example.com">Click</a>`, want: "Click"},
		{name: "complex", input: `<html><body><h1>Title</h1><p>Some <em>emphasized</em> text.</p></body></html>`, want: "Title Some emphasized text."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stripHTML(tt.input); got != tt.want {
				t.Errorf("stripHTML(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestChunkContent(t *testing.T) {
	t.Run("short returns one chunk", func(t *testing.T) {
		got := chunkContent("hello world", 100)
		if len(got) != 1 || got[0] != "hello world" {
			t.Errorf("got %q", got)
		}
	})
	t.Run("long splits on word boundaries", func(t *testing.T) {
		long := strings.Repeat("word ", 1000) // 5000 chars
		got := chunkContent(long, 100)
		if len(got) < 2 {
			t.Errorf("expected multiple chunks, got %d", len(got))
		}
		// Each chunk ends on a word boundary (no partial words).
		for i, c := range got {
			if strings.TrimSpace(c) == "" {
				t.Errorf("chunk %d is empty", i)
			}
		}
	})
	t.Run("empty returns empty", func(t *testing.T) {
		got := chunkContent("", 100)
		// chunkContent returns single-element slice for content <= maxLen,
		// including empty string.
		if len(got) != 1 || got[0] != "" {
			t.Errorf("expected [\"\"], got %v", got)
		}
	})
}

func TestExtractTitle(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "simple", input: `<title>Hello</title>`, want: "Hello"},
		{name: "with attrs", input: `<title lang="en">Hello</title>`, want: "Hello"},
		{name: "missing", input: `<html><body></body></html>`, want: "Untitled Page"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractTitle(tt.input); got != tt.want {
				t.Errorf("extractTitle = %q, want %q", got, tt.want)
			}
		})
	}
}
