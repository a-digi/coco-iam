package transform

import (
	"image"
	"image/color"
	"testing"
)

func TestCrop_InteriorRectangle(t *testing.T) {
	// 40×40 gradient. Crop (10,10)-(20,20). Pixel (0,0) of the
	// output should equal pixel (10,10) of the input.
	src := gradientRGBA(40, 40)
	want := pxAt(src, 10, 10)

	got := Crop(src, image.Rect(10, 10, 20, 20))
	b := got.Bounds()
	if b.Dx() != 10 || b.Dy() != 10 {
		t.Fatalf("crop size: got %dx%d want 10x10", b.Dx(), b.Dy())
	}
	nearRGBA(t, pxAt(got, 0, 0), want, 0, "top-left of crop")
	writeImageOut(t, "crop_interior_10x10", got, FormatPNG)
}

func TestCrop_RectExtendsPastBoundsIsClipped(t *testing.T) {
	// Crop(5,5)-(100,100) against a 40×40 source should produce
	// a 35×35 image (bounds-clipped), not an error.
	src := gradientRGBA(40, 40)
	got := Crop(src, image.Rect(5, 5, 100, 100))
	b := got.Bounds()
	if b.Dx() != 35 || b.Dy() != 35 {
		t.Fatalf("clipped crop: got %dx%d want 35x35", b.Dx(), b.Dy())
	}
}

func TestCrop_RectFullyOutsideReturnsEmpty(t *testing.T) {
	// Crop(100,100)-(110,110) on 40×40 is entirely outside; we
	// return an empty 0×0 image rather than nil or panic.
	src := gradientRGBA(40, 40)
	got := Crop(src, image.Rect(100, 100, 110, 110))
	b := got.Bounds()
	if b.Dx() != 0 || b.Dy() != 0 {
		t.Fatalf("outside crop: got %dx%d want 0x0", b.Dx(), b.Dy())
	}
}

func TestCropCenter_CenteredRectangle(t *testing.T) {
	// 200×200 gradient → CropCenter(100,100). The crop should
	// start at (50,50) of the source.
	src := gradientRGBA(200, 200)
	want := pxAt(src, 50, 50)

	got := CropCenter(src, 100, 100)
	b := got.Bounds()
	if b.Dx() != 100 || b.Dy() != 100 {
		t.Fatalf("CropCenter: got %dx%d want 100x100", b.Dx(), b.Dy())
	}
	nearRGBA(t, pxAt(got, 0, 0), want, 0, "CropCenter top-left")
	writeImageOut(t, "crop_center_100x100_of_200x200", got, FormatPNG)
}

func TestCropCenter_LargerThanSourceClamps(t *testing.T) {
	// Asking for 300×300 center-crop of a 100×100 source should
	// clamp to 100×100 — we never upscale by cropping.
	src := solidRGBA(100, 100, color.RGBA{B: 255, A: 255})
	got := CropCenter(src, 300, 300)
	b := got.Bounds()
	if b.Dx() != 100 || b.Dy() != 100 {
		t.Fatalf("CropCenter clamp: got %dx%d want 100x100",
			b.Dx(), b.Dy())
	}
}

func TestCropCenter_ZeroDimensionsAreNoop(t *testing.T) {
	src := solidRGBA(10, 10, color.RGBA{R: 255, A: 255})
	got := CropCenter(src, 0, 10)
	b := got.Bounds()
	if b.Dx() != 10 || b.Dy() != 10 {
		t.Fatalf("zero dim should clone: got %dx%d", b.Dx(), b.Dy())
	}
}
