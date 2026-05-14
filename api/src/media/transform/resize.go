package transform

import (
	"image"

	"github.com/disintegration/imaging"
)

// Fit selects how Resize reconciles a target width/height with the
// input's aspect ratio.
type Fit int

const (
	// FitContain preserves aspect and fits inside the given box —
	// the output is at most w wide and at most h tall. Either
	// dimension may come out smaller; the other hits the target.
	FitContain Fit = iota
	// FitCover preserves aspect and fills the box exactly, then
	// center-crops the overflow. Output is always exactly w×h.
	FitCover
	// FitStretch ignores aspect and produces exactly w×h by
	// stretching. Use sparingly — distorts the image.
	FitStretch
)

// Resize produces a w×h render of img according to the Fit mode.
//
// A zero w or h is treated as "derive from aspect" for FitContain
// (respecting the non-zero side) and as "use the input's
// dimension" otherwise. When BOTH dimensions are zero the input
// is returned unchanged so callers can feed user-supplied
// parameters without extra branches.
//
// Output is always a fresh image; mutating the return value
// never affects img.
func Resize(img image.Image, w, h int, fit Fit) image.Image {
	if img == nil {
		return nil
	}
	if w <= 0 && h <= 0 {
		return cloneImage(img)
	}
	b := img.Bounds()
	iw, ih := b.Dx(), b.Dy()
	if iw == 0 || ih == 0 {
		return cloneImage(img)
	}

	// Defaults for zero dims depend on fit mode; see docstring.
	switch fit {
	case FitContain:
		// imaging.Fit refuses to upscale, so compute the target
		// dimensions ourselves and delegate to imaging.Resize.
		// That way "contain" means "preserve aspect, fit inside
		// the box" whether we're growing or shrinking.
		tw, th := containDims(iw, ih, w, h)
		return imaging.Resize(img, tw, th, imaging.Lanczos)
	case FitCover:
		return imaging.Fill(img, nonZero(w, iw), nonZero(h, ih), imaging.Center, imaging.Lanczos)
	case FitStretch:
		return imaging.Resize(img, nonZero(w, iw), nonZero(h, ih), imaging.Lanczos)
	}
	return cloneImage(img)
}

// containDims returns the (w, h) that preserves the aspect of
// (iw, ih) while fitting inside the (tw, th) box. Either target
// may be zero, in which case it's inferred from the other side
// via the input's aspect ratio.
func containDims(iw, ih, tw, th int) (int, int) {
	if tw <= 0 && th <= 0 {
		return iw, ih
	}
	if tw <= 0 {
		// Scale by height.
		return int(float64(iw) * float64(th) / float64(ih)), th
	}
	if th <= 0 {
		return tw, int(float64(ih) * float64(tw) / float64(iw))
	}
	// Both targets set — pick the tighter scale factor.
	scaleW := float64(tw) / float64(iw)
	scaleH := float64(th) / float64(ih)
	scale := scaleW
	if scaleH < scaleW {
		scale = scaleH
	}
	return int(float64(iw) * scale), int(float64(ih) * scale)
}

// cloneImage returns a fresh RGBA copy of img so callers can
// mutate without poisoning the original.
func cloneImage(img image.Image) image.Image {
	return imaging.Clone(img)
}

func nonZero(v, fallback int) int {
	if v <= 0 {
		return fallback
	}
	return v
}
