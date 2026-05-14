package transform

import (
	"bytes"
	"errors"
	"image/color"
	"strings"
	"testing"
)

func TestDecode_PNGRoundTrip(t *testing.T) {
	// Red 8×8 → PNG bytes → Decode → sniff format + pixel (0,0).
	src := solidRGBA(8, 8, color.RGBA{R: 255, A: 255})
	pngBytes := encodePNG(t, src)

	img, f, err := Decode(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if f != FormatPNG {
		t.Errorf("format: got %s want png", f)
	}
	nearRGBA(t, pxAt(img, 0, 0), color.RGBA{R: 255, A: 255}, 0, "px(0,0)")
	writeOut(t, "decode_png_input", "png", pngBytes)
}

func TestDecode_JPEGRoundTrip(t *testing.T) {
	// JPEG is lossy; we allow a tolerance.
	src := solidRGBA(16, 16, color.RGBA{R: 255, A: 255})
	jpegBytes := encodeJPEG(t, src, 90)

	img, f, err := Decode(bytes.NewReader(jpegBytes))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if f != FormatJPEG {
		t.Errorf("format: got %s want jpeg", f)
	}
	nearRGBA(t, pxAt(img, 0, 0), color.RGBA{R: 255, A: 255}, 3, "px(0,0)")
	writeOut(t, "decode_jpeg_input", "jpg", jpegBytes)
}

func TestDecode_GIFRoundTrip(t *testing.T) {
	// A single-frame GIF through Encode(...FormatGIF) → Decode.
	src := solidRGBA(8, 8, color.RGBA{G: 255, A: 255})
	var buf bytes.Buffer
	if err := Encode(&buf, src, FormatGIF, EncodeOpts{}); err != nil {
		t.Fatalf("Encode gif: %v", err)
	}

	img, f, err := Decode(&buf)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if f != FormatGIF {
		t.Errorf("format: got %s want gif", f)
	}
	nearRGBA(t, pxAt(img, 0, 0), color.RGBA{G: 255, A: 255}, 4, "px(0,0)")
}

func TestDecode_AnimatedGIFFirstFrameOnly(t *testing.T) {
	// Contract: this package hands back the first frame only.
	// The fixture is red first, green second.
	gifBytes := encodeGIFAnimated(t)

	img, f, err := Decode(bytes.NewReader(gifBytes))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if f != FormatGIF {
		t.Errorf("format: got %s want gif", f)
	}
	got := pxAt(img, 0, 0)
	if got.R < 200 || got.G > 40 {
		t.Errorf("animated gif: expected first (red) frame, got %+v", got)
	}
	writeOut(t, "decode_gif_animated_input", "gif", gifBytes)
}

func TestDecode_TruncatedReturnsError(t *testing.T) {
	src := solidRGBA(8, 8, color.RGBA{B: 255, A: 255})
	pngBytes := encodePNG(t, src)
	if len(pngBytes) < 20 {
		t.Fatalf("encodePNG too small to truncate")
	}
	truncated := pngBytes[:10]

	_, _, err := Decode(bytes.NewReader(truncated))
	if err == nil {
		t.Fatal("expected decode error on truncated bytes")
	}
	if !errors.Is(err, ErrDecode) {
		t.Errorf("error does not wrap ErrDecode: %v", err)
	}
}

func TestDecode_NilReaderReturnsError(t *testing.T) {
	_, _, err := Decode(nil)
	if err == nil {
		t.Fatal("expected error on nil reader")
	}
	if !errors.Is(err, ErrDecode) {
		t.Errorf("error does not wrap ErrDecode: %v", err)
	}
}

func TestDecodeBytes_EmptyInputErrors(t *testing.T) {
	_, _, err := DecodeBytes(nil)
	if err == nil {
		t.Fatal("expected error on empty input")
	}
}

func TestFormat_MIMEAndExtRoundTrip(t *testing.T) {
	cases := []struct {
		f    Format
		mime string
		ext  string
	}{
		{FormatPNG, "image/png", "png"},
		{FormatJPEG, "image/jpeg", "jpg"},
		{FormatGIF, "image/gif", "gif"},
		{FormatWebP, "image/webp", "webp"},
		{FormatUnknown, "", ""},
	}
	for _, tc := range cases {
		if got := tc.f.MIME(); got != tc.mime {
			t.Errorf("%s: MIME got %q want %q", tc.f, got, tc.mime)
		}
		if got := tc.f.Ext(); got != tc.ext {
			t.Errorf("%s: Ext got %q want %q", tc.f, got, tc.ext)
		}
	}

	// FormatFromMIME is case-insensitive and strips params.
	if FormatFromMIME("IMAGE/PNG") != FormatPNG {
		t.Errorf("case-insensitive match broken")
	}
	if FormatFromMIME("image/jpeg; charset=binary") != FormatJPEG {
		t.Errorf("param stripping broken")
	}
	if FormatFromMIME("application/pdf") != FormatUnknown {
		t.Errorf("unrecognised mime should be unknown")
	}
	// image/jpg is a common misspelling people send us.
	if FormatFromMIME("image/jpg") != FormatJPEG {
		t.Errorf("image/jpg should alias to JPEG")
	}

	if !strings.Contains(FormatUnknown.String(), "unknown") {
		t.Errorf("FormatUnknown.String should mention unknown")
	}
}
