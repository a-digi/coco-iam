package transform

import (
	"image/color"
	"testing"
)

func TestGrayscale_RedBecomesEqualChannels(t *testing.T) {
	src := solidRGBA(16, 16, color.RGBA{R: 255, A: 255})
	got := Grayscale(src)
	px := pxAt(got, 0, 0)
	if abs8(px.R, px.G) > 1 || abs8(px.G, px.B) > 1 {
		t.Errorf("grayscale: channels unequal: %+v", px)
	}
	if px.A != 255 {
		t.Errorf("grayscale: alpha dropped: %+v", px)
	}
	writeImageOut(t, "effect_grayscale", got, FormatPNG)
}

func TestSepia_ProducesWarmTone(t *testing.T) {
	// Mid-grey (128,128,128) so the matrix output doesn't
	// saturate — then we can assert R > G > B on every channel.
	src := solidRGBA(16, 16, color.RGBA{R: 128, G: 128, B: 128, A: 255})
	got := Sepia(src)
	px := pxAt(got, 0, 0)
	if px.R <= px.G || px.G <= px.B {
		t.Errorf("sepia tone broken on mid-grey: R=%d G=%d B=%d (want R>G>B)",
			px.R, px.G, px.B)
	}
	// Pure white saturates on R and G, but B still drops below
	// them — separate, weaker assertion on that path.
	whiteSep := Sepia(solidRGBA(4, 4, color.RGBA{R: 255, G: 255, B: 255, A: 255}))
	wpx := pxAt(whiteSep, 0, 0)
	if wpx.B >= wpx.R {
		t.Errorf("sepia on white: B=%d should be < R=%d", wpx.B, wpx.R)
	}
	writeImageOut(t, "effect_sepia_on_grey", got, FormatPNG)
	writeImageOut(t, "effect_sepia_on_gradient",
		Sepia(gradientRGBA(128, 128)), FormatPNG)
}

func TestInvert_RedBecomesCyan(t *testing.T) {
	src := solidRGBA(16, 16, color.RGBA{R: 255, A: 255})
	got := Invert(src)
	px := pxAt(got, 0, 0)
	nearRGBA(t, px, color.RGBA{G: 255, B: 255, A: 255}, 1, "inverted red")
	writeImageOut(t, "effect_invert", got, FormatPNG)
}

func TestBrightness_PositiveLightens(t *testing.T) {
	src := solidRGBA(16, 16, color.RGBA{R: 100, G: 100, B: 100, A: 255})
	got := Brightness(src, 50)
	px := pxAt(got, 0, 0)
	if px.R <= 100 || px.G <= 100 || px.B <= 100 {
		t.Errorf("brightness +50: expected lighter than 100, got %+v", px)
	}
	writeImageOut(t, "effect_brightness_plus50", got, FormatPNG)
}

func TestBrightness_NegativeDarkens(t *testing.T) {
	src := solidRGBA(16, 16, color.RGBA{R: 100, G: 100, B: 100, A: 255})
	got := Brightness(src, -50)
	px := pxAt(got, 0, 0)
	if px.R >= 100 || px.G >= 100 || px.B >= 100 {
		t.Errorf("brightness -50: expected darker than 100, got %+v", px)
	}
	writeImageOut(t, "effect_brightness_minus50", got, FormatPNG)
}

func TestBrightness_OutOfRangeIsClamped(t *testing.T) {
	src := solidRGBA(4, 4, color.RGBA{R: 100, G: 100, B: 100, A: 255})
	// +1000 gets clamped to +100, saturating to 255.
	got := Brightness(src, 1000)
	px := pxAt(got, 0, 0)
	if px.R != 255 || px.G != 255 || px.B != 255 {
		t.Errorf("clamped brightness: got %+v want full white", px)
	}
}

func TestContrast_IncreasesSpread(t *testing.T) {
	// Two neighbouring pixels at different values — after +50
	// contrast the darker one gets darker, the lighter lighter.
	src := gradientRGBA(32, 32)
	got := Contrast(src, 50)
	// Pixel near (0,0) is dark red; pixel near (31,31) is bright.
	dark := pxAt(got, 0, 0)
	bright := pxAt(got, 31, 31)
	if bright.R <= dark.R+100 {
		// Should be a substantial gap after boosting contrast.
		// Not a tight assertion — just pin the direction.
		t.Errorf("contrast gap too small: dark=%d bright=%d", dark.R, bright.R)
	}
	writeImageOut(t, "effect_contrast_plus50", got, FormatPNG)
}

func TestBlur_SmoothsHighFrequency(t *testing.T) {
	// 2×2 checkerboard: blur should move adjacent pixels toward
	// each other. After a sigma-3 blur the difference between
	// (0,0) and (1,0) must drop substantially vs the input.
	src := solidRGBA(32, 32, color.RGBA{A: 255})
	// Paint a checker: even x+y → white, odd → black.
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			if (x+y)%2 == 0 {
				src.SetRGBA(x, y, color.RGBA{R: 255, G: 255, B: 255, A: 255})
			}
		}
	}
	srcDiff := int(pxAt(src, 0, 0).R) - int(pxAt(src, 1, 0).R)
	if srcDiff < 0 {
		srcDiff = -srcDiff
	}

	got := Blur(src, 3)
	gotDiff := int(pxAt(got, 0, 0).R) - int(pxAt(got, 1, 0).R)
	if gotDiff < 0 {
		gotDiff = -gotDiff
	}
	if gotDiff >= srcDiff {
		t.Errorf("blur did not smooth: src diff=%d, blurred diff=%d",
			srcDiff, gotDiff)
	}
	writeImageOut(t, "effect_blur_sigma3", got, FormatPNG)
}

func TestSharpen_IncreasesLocalVariance(t *testing.T) {
	// Blurred image as input; sharpening should restore edge
	// contrast (neighbour pixel diff goes up vs the input).
	blurred := Blur(gradientRGBA(32, 32), 3)
	blurDiff := int(pxAt(blurred, 5, 5).R) - int(pxAt(blurred, 6, 5).R)
	if blurDiff < 0 {
		blurDiff = -blurDiff
	}

	sharp := Sharpen(blurred, 3)
	sharpDiff := int(pxAt(sharp, 5, 5).R) - int(pxAt(sharp, 6, 5).R)
	if sharpDiff < 0 {
		sharpDiff = -sharpDiff
	}
	if sharpDiff <= blurDiff {
		t.Errorf("sharpen did not raise local variance: blur=%d sharp=%d",
			blurDiff, sharpDiff)
	}
	writeImageOut(t, "effect_sharpen_sigma3", sharp, FormatPNG)
}

func TestBlurSharpen_ZeroSigmaReturnsInput(t *testing.T) {
	src := solidRGBA(4, 4, color.RGBA{R: 123, G: 45, B: 67, A: 255})
	b := Blur(src, 0)
	s := Sharpen(src, 0)
	if px := pxAt(b, 0, 0); px.R != 123 {
		t.Errorf("Blur(0) mutated image: %+v", px)
	}
	if px := pxAt(s, 0, 0); px.R != 123 {
		t.Errorf("Sharpen(0) mutated image: %+v", px)
	}
}

func TestBlur_NegativeSigmaTreatedAsNoOp(t *testing.T) {
	src := solidRGBA(4, 4, color.RGBA{R: 200, A: 255})
	got := Blur(src, -5)
	if pxAt(got, 0, 0).R != 200 {
		t.Errorf("negative sigma should be no-op")
	}
}

func TestEffects_SinglePixelImage(t *testing.T) {
	// Degenerate input — every effect must handle it without
	// panicking.
	src := solidRGBA(1, 1, color.RGBA{R: 123, A: 255})
	_ = Grayscale(src)
	_ = Sepia(src)
	_ = Invert(src)
	_ = Brightness(src, 20)
	_ = Contrast(src, 20)
	_ = Blur(src, 1)
	_ = Sharpen(src, 1)
}

func abs8(a, b uint8) int {
	d := int(a) - int(b)
	if d < 0 {
		return -d
	}
	return d
}
