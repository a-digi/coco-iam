package transform

import (
	"image"

	"github.com/disintegration/imaging"
)

// Crop returns the sub-image defined by r. The rectangle is
// translated into the input's coordinate system and clipped to
// its bounds — rectangles extending past the edge are trimmed,
// never an error. A rectangle wholly outside the image produces
// an empty 0×0 result (the caller should guard against that
// case; we don't return an error to keep the API simple for
// pipeline use).
//
// The output is a fresh image — mutating it never affects img.
func Crop(img image.Image, r image.Rectangle) image.Image {
	if img == nil {
		return nil
	}
	// Normalise r into the input's coordinate system. imaging.Crop
	// interprets r as absolute coordinates, so we intersect with
	// img.Bounds() before calling it.
	clipped := r.Intersect(img.Bounds())
	if clipped.Empty() {
		return image.NewRGBA(image.Rect(0, 0, 0, 0))
	}
	return imaging.Crop(img, clipped)
}

// CropCenter returns a w×h rectangle centered on the input. If
// w or h exceeds the input's dimensions, the crop is clamped to
// the input — we never upscale by cropping. Passing w<=0 or
// h<=0 returns a clone of the input (no-op semantics).
func CropCenter(img image.Image, w, h int) image.Image {
	if img == nil {
		return nil
	}
	if w <= 0 || h <= 0 {
		return cloneImage(img)
	}
	b := img.Bounds()
	iw, ih := b.Dx(), b.Dy()
	// Clamp so the crop never exceeds the source — cropping
	// larger than the source would force a (silent) upscale,
	// and callers should use Resize for that intent.
	if w > iw {
		w = iw
	}
	if h > ih {
		h = ih
	}
	return imaging.CropCenter(img, w, h)
}
