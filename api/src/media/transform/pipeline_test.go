package transform

import (
	"bytes"
	"image"
	"image/color"
	"testing"
)

func TestPipeline_NoOpsClonesInput(t *testing.T) {
	src := solidRGBA(4, 4, color.RGBA{R: 123, A: 255})
	got := NewPipeline(src).Build()
	if got == nil {
		t.Fatal("Build returned nil")
	}
	if pxAt(got, 0, 0).R != 123 {
		t.Errorf("empty pipeline should preserve pixels")
	}
	// Clone, so pointer identity differs.
	if any(got) == any(image.Image(src)) {
		t.Errorf("empty pipeline should NOT hand back the input pointer")
	}
}

func TestPipeline_SingleResizeEquivalentToDirect(t *testing.T) {
	// NewPipeline(img).Resize(...).Build() must byte-match a
	// direct Resize call — confirms the pipeline doesn't add
	// its own quirks.
	src := gradientRGBA(40, 40)
	direct := Resize(src, 20, 20, FitCover)
	piped := NewPipeline(src).Resize(20, 20, FitCover).Build()

	db := encodePNG(t, direct)
	pb := encodePNG(t, piped)
	if !bytes.Equal(db, pb) {
		t.Errorf("pipeline diverges from direct call: direct=%d bytes, piped=%d bytes",
			len(db), len(pb))
	}
}

func TestPipeline_OrderMatters(t *testing.T) {
	// Grayscale → Sepia: input gets desaturated first, then sepia
	// tint applied → result is a warm-toned grey.
	// Sepia → Grayscale: sepia applied first, then grayscale
	// removes the tint entirely → result is pure grey.
	// Pin the order by checking channel equality on the second
	// form (grayscale output always has R==G==B, within 1).
	src := solidRGBA(16, 16, color.RGBA{R: 128, G: 128, B: 128, A: 255})

	grayFirst := NewPipeline(src).Grayscale().Sepia().Build()
	sepiaFirst := NewPipeline(src).Sepia().Grayscale().Build()

	gfPx := pxAt(grayFirst, 0, 0)
	sfPx := pxAt(sepiaFirst, 0, 0)

	// Sepia-after-grayscale still produces R > G > B because
	// sepia's last step.
	if gfPx.R <= gfPx.B {
		t.Errorf("gray→sepia: expected warm tone, got %+v", gfPx)
	}
	// Grayscale-after-sepia flattens back to equal channels.
	if abs8(sfPx.R, sfPx.G) > 2 || abs8(sfPx.G, sfPx.B) > 2 {
		t.Errorf("sepia→gray: expected equal channels, got %+v", sfPx)
	}

	writeImageOut(t, "pipeline_order_gray_then_sepia", grayFirst, FormatPNG)
	writeImageOut(t, "pipeline_order_sepia_then_gray", sepiaFirst, FormatPNG)
}

func TestPipeline_EncodeWritesValidJPEG(t *testing.T) {
	src := gradientRGBA(32, 32)
	var buf bytes.Buffer
	err := NewPipeline(src).
		Resize(16, 16, FitContain).
		Sepia().
		Encode(&buf, FormatJPEG, EncodeOpts{JPEGQuality: 80})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	b := buf.Bytes()
	if len(b) < 2 || b[0] != 0xFF || b[1] != 0xD8 {
		t.Errorf("pipeline encode did not produce JPEG: %x", b[:min(4, len(b))])
	}
	writeOut(t, "pipeline_resize_sepia_encode", "jpg", b)
}

func TestPipeline_ReusableNoSharedState(t *testing.T) {
	// Build twice; each result must be a distinct image instance
	// so mutating one can never affect the other.
	src := solidRGBA(8, 8, color.RGBA{R: 200, A: 255})
	p := NewPipeline(src).Resize(4, 4, FitCover)
	a := p.Build()
	b := p.Build()
	if any(a) == any(b) {
		t.Errorf("pipeline returned same image pointer on second Build")
	}
}
