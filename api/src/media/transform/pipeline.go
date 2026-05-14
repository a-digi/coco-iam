package transform

import (
	"image"
	"io"
)

// Pipeline chains transforms so a caller can write
//
//	NewPipeline(img).Resize(256,256,FitCover).Sepia().Encode(w, FormatJPEG, opts)
//
// instead of nesting function calls. Each method returns the
// same receiver so chains read left-to-right. Build() materialises
// the result image; Encode() is a convenience that builds + writes
// in one step.
//
// Pipelines are reusable: Build() can be called multiple times
// and each call produces a fresh image with no shared mutable
// state. Internally we store the starting image and a list of
// ops; Build walks the list each time.
type Pipeline struct {
	start image.Image
	ops   []opFunc
}

type opFunc func(image.Image) image.Image

// NewPipeline starts a pipeline from img. img is kept by reference;
// it should not be mutated while the pipeline is still in use.
// Later Build() calls will re-apply the ops to this starting
// image, so effectively it is the "zero step" — everything
// derives from it.
func NewPipeline(img image.Image) *Pipeline {
	return &Pipeline{start: img}
}

// Resize appends a resize operation. See Resize().
func (p *Pipeline) Resize(w, h int, fit Fit) *Pipeline {
	p.ops = append(p.ops, func(src image.Image) image.Image {
		return Resize(src, w, h, fit)
	})
	return p
}

// Crop appends a crop to the given rectangle. See Crop().
func (p *Pipeline) Crop(r image.Rectangle) *Pipeline {
	p.ops = append(p.ops, func(src image.Image) image.Image {
		return Crop(src, r)
	})
	return p
}

// CropCenter appends a centered w×h crop. See CropCenter().
func (p *Pipeline) CropCenter(w, h int) *Pipeline {
	p.ops = append(p.ops, func(src image.Image) image.Image {
		return CropCenter(src, w, h)
	})
	return p
}

// Grayscale appends a grayscale step.
func (p *Pipeline) Grayscale() *Pipeline {
	p.ops = append(p.ops, Grayscale)
	return p
}

// Sepia appends a sepia step.
func (p *Pipeline) Sepia() *Pipeline {
	p.ops = append(p.ops, Sepia)
	return p
}

// Invert appends an invert step.
func (p *Pipeline) Invert() *Pipeline {
	p.ops = append(p.ops, Invert)
	return p
}

// Brightness appends a brightness shift. See Brightness().
func (p *Pipeline) Brightness(pct float64) *Pipeline {
	p.ops = append(p.ops, func(src image.Image) image.Image {
		return Brightness(src, pct)
	})
	return p
}

// Contrast appends a contrast adjustment. See Contrast().
func (p *Pipeline) Contrast(pct float64) *Pipeline {
	p.ops = append(p.ops, func(src image.Image) image.Image {
		return Contrast(src, pct)
	})
	return p
}

// Blur appends a Gaussian blur. See Blur().
func (p *Pipeline) Blur(sigma float64) *Pipeline {
	p.ops = append(p.ops, func(src image.Image) image.Image {
		return Blur(src, sigma)
	})
	return p
}

// Sharpen appends an unsharp-mask sharpen. See Sharpen().
func (p *Pipeline) Sharpen(sigma float64) *Pipeline {
	p.ops = append(p.ops, func(src image.Image) image.Image {
		return Sharpen(src, sigma)
	})
	return p
}

// Build applies the ops and returns the final image. Safe to
// call multiple times — each call produces a fresh result.
// Returns nil if the pipeline was constructed with a nil image
// and no ops (or with a nil image that no op could rescue).
func (p *Pipeline) Build() image.Image {
	if p == nil {
		return nil
	}
	img := p.start
	if len(p.ops) == 0 && img != nil {
		img = cloneImage(img)
	}
	for _, op := range p.ops {
		img = op(img)
	}
	return img
}

// Encode builds the image and writes it to w in the requested
// format. Equivalent to Encode(w, p.Build(), f, opts) but reads
// more naturally at call sites.
func (p *Pipeline) Encode(w io.Writer, f Format, opts EncodeOpts) error {
	return Encode(w, p.Build(), f, opts)
}
