package transform

import (
	"image/color"
	"testing"
)

func TestResize_FitContainSmaller(t *testing.T) {
	// 200×100 → fit inside 50×50. Expected: width 50, height 25
	// (aspect 2:1 preserved). The taller dimension of the box is
	// unused.
	src := gradientRGBA(200, 100)
	got := Resize(src, 50, 50, FitContain)
	b := got.Bounds()
	if b.Dx() != 50 || b.Dy() != 25 {
		t.Fatalf("FitContain 50×50 of 200×100: got %dx%d want 50x25",
			b.Dx(), b.Dy())
	}
	writeImageOut(t, "resize_contain_50x50_of_200x100", got, FormatPNG)
}

func TestResize_FitContainUpscaleAllowed(t *testing.T) {
	src := solidRGBA(10, 10, color.RGBA{R: 255, A: 255})
	got := Resize(src, 100, 100, FitContain)
	b := got.Bounds()
	if b.Dx() != 100 || b.Dy() != 100 {
		t.Fatalf("upscale 10×10 → 100×100: got %dx%d", b.Dx(), b.Dy())
	}
}

func TestResize_FitCoverExactBox(t *testing.T) {
	// 200×100 → Fill(50,50) is EXACTLY 50×50 with center-crop.
	src := gradientRGBA(200, 100)
	got := Resize(src, 50, 50, FitCover)
	b := got.Bounds()
	if b.Dx() != 50 || b.Dy() != 50 {
		t.Fatalf("FitCover 50×50 of 200×100: got %dx%d want 50x50",
			b.Dx(), b.Dy())
	}
	writeImageOut(t, "resize_cover_50x50_of_200x100", got, FormatPNG)
}

func TestResize_FitStretchDistortsAspect(t *testing.T) {
	// 200×100 → Stretch(50,50) DOES distort; output is 50×50 and
	// if we compared the aspect we'd see it's now 1:1 instead of
	// 2:1.
	src := gradientRGBA(200, 100)
	got := Resize(src, 50, 50, FitStretch)
	b := got.Bounds()
	if b.Dx() != 50 || b.Dy() != 50 {
		t.Fatalf("FitStretch 50×50 of 200×100: got %dx%d",
			b.Dx(), b.Dy())
	}
	writeImageOut(t, "resize_stretch_50x50_of_200x100", got, FormatPNG)
}

func TestResize_ZeroWidthFitContainDerivesFromAspect(t *testing.T) {
	// 200×100 → Resize(0, 50, Contain). Expected width inferred
	// from aspect: w=100. Contract: zero on either side is
	// treated as "fallback to the non-zero constraint".
	src := gradientRGBA(200, 100)
	got := Resize(src, 0, 50, FitContain)
	b := got.Bounds()
	if b.Dy() != 50 {
		t.Fatalf("Contain(0,50): height got %d want 50", b.Dy())
	}
	if b.Dx() != 100 {
		t.Fatalf("Contain(0,50): width got %d want 100 (aspect 2:1)",
			b.Dx())
	}
}

func TestResize_ZeroDimsReturnClone(t *testing.T) {
	src := gradientRGBA(40, 30)
	got := Resize(src, 0, 0, FitContain)
	if got == src {
		t.Errorf("zero dims must return a fresh image, not the input pointer")
	}
	b := got.Bounds()
	if b.Dx() != 40 || b.Dy() != 30 {
		t.Fatalf("zero dims should preserve size: got %dx%d",
			b.Dx(), b.Dy())
	}
}

func TestResize_OutputIsFreshImage(t *testing.T) {
	src := solidRGBA(10, 10, color.RGBA{R: 255, A: 255})
	got := Resize(src, 10, 10, FitContain)
	if pxAt(got, 0, 0).R != 255 {
		t.Fatalf("sanity check failed before mutation")
	}
	// We can't mutate `got` since it's an image.Image; the real
	// point is that it isn't the same pointer as src.
	if any(got) == any(src) {
		t.Errorf("Resize returned the input image directly")
	}
}

func TestResize_AlphaPreserved(t *testing.T) {
	// Semi-transparent red, resize, the alpha should survive.
	// Use NRGBA so "R:255, A:128" means the intuitive "50%
	// transparent fully-saturated red" — RGBA would premultiply
	// to R=128 which defeats the point.
	src := solidNRGBA(32, 32, color.NRGBA{R: 255, A: 128})
	got := Resize(src, 16, 16, FitCover)
	// At.RGBA returns premultiplied, so pxAt reports the
	// premultiplied R (≈128). Alpha is 128 either way.
	px := pxAt(got, 8, 8)
	if px.A < 120 || px.A > 136 {
		t.Errorf("alpha drifted: got A=%d want ~128", px.A)
	}
	// Premultiplied red channel ≈ 255 * (128/255) = 128.
	if px.R < 110 || px.R > 145 {
		t.Errorf("R channel (premultiplied) drifted: got R=%d want ~128", px.R)
	}
	writeImageOut(t, "resize_alpha_preserved", got, FormatPNG)
}

func any(x interface{}) interface{} { return x }
