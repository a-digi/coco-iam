package transform

import (
	"bytes"
	"errors"
	"image/color"
	"testing"
)

func TestEncode_PNGHasMagicHeader(t *testing.T) {
	img := solidRGBA(8, 8, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := Encode(&buf, img, FormatPNG, EncodeOpts{}); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	b := buf.Bytes()
	// PNG magic: 89 50 4E 47 0D 0A 1A 0A
	want := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	if !bytes.HasPrefix(b, want) {
		t.Fatalf("not PNG: first 8 bytes = %x", b[:min(8, len(b))])
	}
	writeOut(t, "encode_png", "png", b)
}

func TestEncode_JPEGHasMagicHeader(t *testing.T) {
	img := solidRGBA(16, 16, color.RGBA{B: 255, A: 255})
	var buf bytes.Buffer
	if err := Encode(&buf, img, FormatJPEG, EncodeOpts{JPEGQuality: 85}); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	b := buf.Bytes()
	// JPEG SOI: FF D8.
	if len(b) < 2 || b[0] != 0xFF || b[1] != 0xD8 {
		t.Fatalf("not JPEG: first 2 bytes = %x", b[:min(2, len(b))])
	}
	if len(b) == 0 || len(b) > 200_000 {
		t.Errorf("JPEG size sanity: got %d bytes", len(b))
	}
	writeOut(t, "encode_jpeg_q85", "jpg", b)
}

func TestEncode_JPEGLowerQualityIsSmaller(t *testing.T) {
	// Same gradient image, different quality knobs. Quality=1
	// must produce fewer bytes than quality=95. Pins that the
	// knob actually drives the encoder.
	img := gradientRGBA(64, 64)
	var hi, lo bytes.Buffer
	if err := Encode(&hi, img, FormatJPEG, EncodeOpts{JPEGQuality: 95}); err != nil {
		t.Fatalf("hi encode: %v", err)
	}
	if err := Encode(&lo, img, FormatJPEG, EncodeOpts{JPEGQuality: 1}); err != nil {
		t.Fatalf("lo encode: %v", err)
	}
	if lo.Len() >= hi.Len() {
		t.Errorf("quality knob broken: q=1 size %d >= q=95 size %d",
			lo.Len(), hi.Len())
	}
	writeOut(t, "encode_jpeg_q95", "jpg", hi.Bytes())
	writeOut(t, "encode_jpeg_q1", "jpg", lo.Bytes())
}

func TestEncode_GIFHasMagicHeader(t *testing.T) {
	img := solidRGBA(8, 8, color.RGBA{G: 255, A: 255})
	var buf bytes.Buffer
	if err := Encode(&buf, img, FormatGIF, EncodeOpts{}); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	b := buf.Bytes()
	// GIF89a or GIF87a.
	if !bytes.HasPrefix(b, []byte("GIF87a")) && !bytes.HasPrefix(b, []byte("GIF89a")) {
		t.Fatalf("not GIF: header %q", string(b[:min(6, len(b))]))
	}
	writeOut(t, "encode_gif", "gif", b)
}

func TestEncode_WebPReturnsUnsupported(t *testing.T) {
	img := solidRGBA(4, 4, color.RGBA{A: 255})
	err := Encode(&bytes.Buffer{}, img, FormatWebP, EncodeOpts{})
	if err == nil {
		t.Fatal("expected unsupported-encode error on WebP")
	}
	if !errors.Is(err, ErrUnsupportedEncode) {
		t.Errorf("error does not wrap ErrUnsupportedEncode: %v", err)
	}
}

func TestEncode_NilArgsReturnErrors(t *testing.T) {
	img := solidRGBA(4, 4, color.RGBA{A: 255})
	if err := Encode(nil, img, FormatPNG, EncodeOpts{}); err == nil {
		t.Error("want error on nil writer")
	}
	if err := Encode(&bytes.Buffer{}, nil, FormatPNG, EncodeOpts{}); err == nil {
		t.Error("want error on nil image")
	}
	if err := Encode(&bytes.Buffer{}, img, FormatUnknown, EncodeOpts{}); err == nil {
		t.Error("want error on FormatUnknown")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
