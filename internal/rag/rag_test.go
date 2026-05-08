package rag

import (
	"io"
	"testing"

	"github.com/zerodha/logf"
)

func TestHashContent(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "empty string",
			content: "",
			want:    "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:    "simple string",
			content: "hello world",
			want:    "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HashContent(tt.content); got != tt.want {
				t.Errorf("HashContent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHashContentDeterministic(t *testing.T) {
	content := "The same content always hashes to the same value."
	if HashContent(content) != HashContent(content) {
		t.Error("HashContent should be deterministic")
	}
}

func TestHashContentDistinct(t *testing.T) {
	if HashContent("a") == HashContent("b") {
		t.Error("HashContent should differ for different content")
	}
}

func TestFloat32SliceToVector(t *testing.T) {
	tests := []struct {
		name string
		v    []float32
		want string
	}{
		{name: "empty", v: []float32{}, want: "[]"},
		{name: "single", v: []float32{1.0}, want: "[1.000000]"},
		{name: "multi", v: []float32{1.0, 2.5, 3.14159}, want: "[1.000000,2.500000,3.141590]"},
		{name: "negatives", v: []float32{-1.0, 0.0, 1.0}, want: "[-1.000000,0.000000,1.000000]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Float32SliceToVector(tt.v); got != tt.want {
				t.Errorf("Float32SliceToVector() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFloat32SliceToVectorPgvectorShape(t *testing.T) {
	// The textual form must be parseable by pgvector's `::vector` cast.
	// At minimum: bracket-wrapped, comma-separated, no extra whitespace.
	result := Float32SliceToVector([]float32{0.1, 0.2, 0.3})
	if result[0] != '[' || result[len(result)-1] != ']' {
		t.Errorf("must be bracket-enclosed: %q", result)
	}
}

// TestGetConversationImagesNilMediaBlobFunc — when the manager has no
// mediaBlobFunc wired, the multimodal pipeline must degrade gracefully
// to a text-only prompt rather than tripping a nil-deref. cmd/rag.go's
// generate handler treats (nil, nil) as "no images", so the contract
// here is the empty-slice-no-error return.
func TestGetConversationImagesNilMediaBlobFunc(t *testing.T) {
	lo := logf.New(logf.Opts{Writer: io.Discard})
	m := &Manager{lo: &lo}
	images, err := m.GetConversationImages(123, 3)
	if err != nil {
		t.Errorf("expected nil error when mediaBlobFunc is nil, got %v", err)
	}
	if len(images) != 0 {
		t.Errorf("expected no images, got %d", len(images))
	}
}
