package image

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/disintegration/imaging"
)

// makeTestImage builds a synthetic RGBA image at the requested
// dimensions and encodes it in the requested imaging format. Used by
// the resize tests to exercise both downscale and pass-through paths
// without dragging in a fixture file.
func makeTestImage(t *testing.T, w, h int, format imaging.Format) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	// Solid colour fill — Lanczos behaviour on a flat image is
	// deterministic, which we want for the dimension assertions.
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 0, G: 128, B: 255, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := imaging.Encode(&buf, img, format); err != nil {
		t.Fatalf("encoding test image: %v", err)
	}
	return buf.Bytes()
}

func TestResizeForAI_DownscalesLargeImage(t *testing.T) {
	src := makeTestImage(t, 1200, 800, imaging.JPEG)
	out, contentType, err := ResizeForAI(bytes.NewReader(src), "image/jpeg")
	if err != nil {
		t.Fatalf("ResizeForAI: %v", err)
	}
	if contentType != "image/jpeg" {
		t.Errorf("contentType = %q, want image/jpeg", contentType)
	}
	decoded, err := imaging.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("decoding output: %v", err)
	}
	b := decoded.Bounds()
	if b.Dx() > MaxAIDimension || b.Dy() > MaxAIDimension {
		t.Errorf("output %dx%d exceeds MaxAIDimension %d", b.Dx(), b.Dy(), MaxAIDimension)
	}
	// 1200x800 input → long side 1200 → scaled to 500x333 (preserves aspect).
	if b.Dx() != MaxAIDimension {
		t.Errorf("expected long side = %d, got %d", MaxAIDimension, b.Dx())
	}
}

func TestResizeForAI_PassesThroughSmallImage(t *testing.T) {
	src := makeTestImage(t, 200, 150, imaging.PNG)
	out, contentType, err := ResizeForAI(bytes.NewReader(src), "image/png")
	if err != nil {
		t.Fatalf("ResizeForAI: %v", err)
	}
	if contentType != "image/png" {
		t.Errorf("contentType = %q, want image/png", contentType)
	}
	decoded, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("decoding PNG output: %v", err)
	}
	b := decoded.Bounds()
	if b.Dx() != 200 || b.Dy() != 150 {
		t.Errorf("dimensions changed unexpectedly: got %dx%d, want 200x150", b.Dx(), b.Dy())
	}
}

func TestResizeForAI_UnknownContentTypeFallsBackToJPEG(t *testing.T) {
	src := makeTestImage(t, 100, 100, imaging.JPEG)
	out, contentType, err := ResizeForAI(bytes.NewReader(src), "image/webp")
	if err != nil {
		t.Fatalf("ResizeForAI: %v", err)
	}
	if contentType != "image/jpeg" {
		t.Errorf("unknown input type should fall back to JPEG, got %q", contentType)
	}
	if len(out) == 0 {
		t.Error("expected non-empty output")
	}
}

func TestResizeForAI_GIFOutputForGIFInput(t *testing.T) {
	src := makeTestImage(t, 100, 100, imaging.GIF)
	_, contentType, err := ResizeForAI(bytes.NewReader(src), "image/gif")
	if err != nil {
		t.Fatalf("ResizeForAI: %v", err)
	}
	if contentType != "image/gif" {
		t.Errorf("contentType = %q, want image/gif", contentType)
	}
}

func TestResizeForAI_DecodeError(t *testing.T) {
	_, _, err := ResizeForAI(bytes.NewReader([]byte("not an image")), "image/jpeg")
	if err == nil {
		t.Error("expected decode error on garbage input")
	}
}

func TestToBase64DataURL(t *testing.T) {
	got := ToBase64DataURL([]byte("hi"), "image/jpeg")
	want := "data:image/jpeg;base64,aGk="
	if got != want {
		t.Errorf("ToBase64DataURL = %q, want %q", got, want)
	}
}

func TestResizeAndEncodeForAI(t *testing.T) {
	src := makeTestImage(t, 600, 400, imaging.JPEG)
	dataURL, err := ResizeAndEncodeForAI(bytes.NewReader(src), "image/jpeg")
	if err != nil {
		t.Fatalf("ResizeAndEncodeForAI: %v", err)
	}
	if !strings.HasPrefix(dataURL, "data:image/jpeg;base64,") {
		t.Errorf("expected data:image/jpeg;base64, prefix, got %q (len=%d)", dataURL[:40], len(dataURL))
	}
	if len(dataURL) <= len("data:image/jpeg;base64,") {
		t.Error("expected non-empty base64 payload")
	}
}
