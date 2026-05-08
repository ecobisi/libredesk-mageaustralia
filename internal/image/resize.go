// Package image — resize.go provides image resizing utilities for the
// multimodal-AI pipeline. RAG generate (T3e) ships up to N image
// attachments alongside the customer text; sending originals straight
// to OpenAI / OpenRouter would burn token budget on full-resolution
// photos, so attachments are scaled to fit MaxAIDimension on the long
// side before base64 encoding.
//
// Sits next to image.go's GetDimensions / CreateThumb (T3a-era) and
// reuses the same disintegration/imaging library so the encoder/decoder
// behaviour stays consistent with thumbnail generation.
package image

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"

	"github.com/disintegration/imaging"
)

const (
	// MaxAIDimension is the max width or height for images sent to AI
	// APIs. 500px on the long side hits OpenAI/OpenRouter's "low detail"
	// tier comfortably and keeps a 4-image multimodal payload under a
	// few hundred KB of base64.
	MaxAIDimension = 500
	// JpegQuality is the quality setting for JPEG encoding. 85 is the
	// long-standing "visually lossless" sweet spot the imaging package
	// recommends.
	JpegQuality = 85
)

// ResizeForAI reads an image, resizes it to fit within MaxAIDimension
// on the long side (preserving aspect ratio), and returns the encoded
// bytes plus the output content-type. Output format follows the input:
// PNG/GIF round-trip in their native format, everything else encodes
// as JPEG so non-photographic inputs (e.g. WebP, BMP) still produce a
// payload OpenAI's image_url accepts.
//
// Already-small images skip the Fit step but still re-encode through
// the imaging library so the output is normalised (e.g. EXIF rotation
// applied, ICC profile dropped) — the alternative of returning the
// original bytes can pass through orientation metadata that the AI
// vision pipeline ignores, leaving sideways images.
func ResizeForAI(reader io.Reader, contentType string) ([]byte, string, error) {
	img, err := imaging.Decode(reader)
	if err != nil {
		return nil, "", fmt.Errorf("decoding image: %w", err)
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Output format selected from input content-type. Anything that
	// isn't PNG/GIF falls through to JPEG so unknown image/* mimetypes
	// (image/webp, image/bmp, image/tiff …) still produce a usable
	// payload.
	format := imaging.JPEG
	outputContentType := "image/jpeg"
	switch contentType {
	case "image/png":
		format = imaging.PNG
		outputContentType = "image/png"
	case "image/gif":
		format = imaging.GIF
		outputContentType = "image/gif"
	}

	// Fit preserves aspect ratio and only downscales — small images
	// pass through untouched (re-encoded below for normalisation).
	if width > MaxAIDimension || height > MaxAIDimension {
		img = imaging.Fit(img, MaxAIDimension, MaxAIDimension, imaging.Lanczos)
	}

	var buf bytes.Buffer
	opts := []imaging.EncodeOption{}
	if format == imaging.JPEG {
		opts = append(opts, imaging.JPEGQuality(JpegQuality))
	}
	if err := imaging.Encode(&buf, img, format, opts...); err != nil {
		return nil, "", fmt.Errorf("encoding image: %w", err)
	}

	return buf.Bytes(), outputContentType, nil
}

// ToBase64DataURL converts image bytes to a data URL the OpenAI /
// OpenRouter chat-completions image_url field accepts directly.
// Format: data:<content-type>;base64,<encoded-data>
func ToBase64DataURL(data []byte, contentType string) string {
	return fmt.Sprintf("data:%s;base64,%s", contentType, base64.StdEncoding.EncodeToString(data))
}

// ResizeAndEncodeForAI is the convenience wrapper used by the RAG
// pipeline: read raw image bytes, resize, return a base64 data URL
// ready to drop into a multimodal prompt payload.
func ResizeAndEncodeForAI(reader io.Reader, contentType string) (string, error) {
	data, outputContentType, err := ResizeForAI(reader, contentType)
	if err != nil {
		return "", err
	}
	return ToBase64DataURL(data, outputContentType), nil
}
