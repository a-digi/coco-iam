package transform_test

import (
	"bytes"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/a-digi/coco-iam/src/media/transform"
)

// External test package — imports ONLY the public API. Pins that
// a realistic caller can build a real pipeline, encode it, decode
// the bytes back, and end up with a valid image. Matches the
// "round-trip integration" case described in the plan.
func TestPipelineRoundTrip_ResizeSepiaEncodeJPEGDecode(t *testing.T) {
	// 256×256 horizontal red gradient.
	src := image.NewRGBA(image.Rect(0, 0, 256, 256))
	for y := 0; y < 256; y++ {
		for x := 0; x < 256; x++ {
			src.SetRGBA(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 0, A: 255})
		}
	}

	var buf bytes.Buffer
	err := transform.NewPipeline(src).
		Resize(128, 128, transform.FitCover).
		Sepia().
		Encode(&buf, transform.FormatJPEG, transform.EncodeOpts{JPEGQuality: 80})
	if err != nil {
		t.Fatalf("pipeline encode: %v", err)
	}

	// Decode the bytes back — the public API's contract is that
	// what Encode writes, Decode can read.
	out, f, err := transform.Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if f != transform.FormatJPEG {
		t.Fatalf("decoded format: got %s want jpeg", f)
	}
	b := out.Bounds()
	if b.Dx() != 128 || b.Dy() != 128 {
		t.Fatalf("decoded size: got %dx%d want 128x128", b.Dx(), b.Dy())
	}
	// A sepia-tinted sample must have R > B.
	c := out.At(64, 64)
	r, _, bl, _ := c.RGBA()
	if r>>8 <= bl>>8 {
		t.Errorf("sepia tint missing on decoded sample: R=%d B=%d",
			r>>8, bl>>8)
	}

	// Write to the gallery via the filesystem directly — the
	// external package doesn't have access to writeOut().
	outPath := filepath.Join("testdata", "out", "external_roundtrip.jpg")
	_ = os.MkdirAll(filepath.Dir(outPath), 0o755)
	if err := os.WriteFile(outPath, buf.Bytes(), 0o644); err != nil {
		t.Logf("gallery write: %v", err)
	}
}
