package transform

import (
	"image"
	"image/color"

	"github.com/disintegration/imaging"
)

// Grayscale desaturates img, using standard luminance weights.
// Alpha is preserved.
func Grayscale(img image.Image) image.Image {
	if img == nil {
		return nil
	}
	return imaging.Grayscale(img)
}

// Invert flips each colour channel (255-x). Alpha is preserved.
func Invert(img image.Image) image.Image {
	if img == nil {
		return nil
	}
	return imaging.Invert(img)
}

// Brightness shifts each channel by pct (-100..+100). Positive
// values brighten, negative darken. Out-of-range values are
// clamped to ±100 so callers can pass user input directly
// without panicking.
func Brightness(img image.Image, pct float64) image.Image {
	if img == nil {
		return nil
	}
	pct = clampPct(pct)
	return imaging.AdjustBrightness(img, pct)
}

// Contrast expands or collapses the pixel distribution around
// the midpoint. pct is in -100..+100; positive boosts contrast,
// negative flattens it. Clamped like Brightness.
func Contrast(img image.Image, pct float64) image.Image {
	if img == nil {
		return nil
	}
	pct = clampPct(pct)
	return imaging.AdjustContrast(img, pct)
}

// Blur applies a Gaussian blur with the given standard
// deviation. sigma <= 0 returns the input unchanged (cloned so
// callers can mutate freely).
func Blur(img image.Image, sigma float64) image.Image {
	if img == nil {
		return nil
	}
	if sigma <= 0 {
		return cloneImage(img)
	}
	return imaging.Blur(img, sigma)
}

// Sharpen applies an unsharp-mask style sharpen. sigma sets the
// blur radius used to build the mask; sigma <= 0 returns input
// unchanged.
func Sharpen(img image.Image, sigma float64) image.Image {
	if img == nil {
		return nil
	}
	if sigma <= 0 {
		return cloneImage(img)
	}
	return imaging.Sharpen(img, sigma)
}

// Sepia applies the classic warm-tone filter via the standard
// sepia matrix:
//
//	R' = 0.393 R + 0.769 G + 0.189 B
//	G' = 0.349 R + 0.686 G + 0.168 B
//	B' = 0.272 R + 0.534 G + 0.131 B
//
// imaging doesn't ship a sepia; we implement it ourselves so
// callers can chain it with the library's effects through the
// Pipeline.
func Sepia(img image.Image) image.Image {
	if img == nil {
		return nil
	}
	b := img.Bounds()
	out := image.NewNRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, a := img.At(x, y).RGBA()
			// Work in 8-bit space for the matrix.
			r8, g8, b8 := float64(r>>8), float64(g>>8), float64(bl>>8)
			nr := 0.393*r8 + 0.769*g8 + 0.189*b8
			ng := 0.349*r8 + 0.686*g8 + 0.168*b8
			nb := 0.272*r8 + 0.534*g8 + 0.131*b8
			out.SetNRGBA(x, y, color.NRGBA{
				R: clamp8(nr),
				G: clamp8(ng),
				B: clamp8(nb),
				A: uint8(a >> 8),
			})
		}
	}
	return out
}

func clampPct(v float64) float64 {
	if v > 100 {
		return 100
	}
	if v < -100 {
		return -100
	}
	return v
}

func clamp8(v float64) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}
