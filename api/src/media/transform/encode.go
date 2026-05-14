package transform

import (
	"errors"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
)

// EncodeOpts carries format-specific knobs. Zero values pick
// sensible defaults. Fields unrelated to the target format are
// ignored (JPEGQuality is a no-op when encoding PNG, etc.).
type EncodeOpts struct {
	// JPEGQuality in 1..100. 0 ⇒ library default (75).
	JPEGQuality int
	// PNGCompression picks a level from image/png. 0 ⇒ library
	// default. Valid values: -3 (BestSpeed) .. +0 (Default) ..
	// +1 (BestCompression). Outside range falls back to default.
	PNGCompression png.CompressionLevel
	// GIFNumColors caps the palette size (1..256). 0 ⇒ default
	// (256). Lower values trade fidelity for file size.
	GIFNumColors int
}

// ErrEncode is wrapped around the underlying encoder error for
// error-inspection by callers.
var ErrEncode = errors.New("transform: encode failed")

// ErrUnsupportedEncode is returned when the caller asks for an
// output format this package can't produce. Today that is WebP
// (encoding WebP requires libwebp / CGo; we stay pure-Go).
var ErrUnsupportedEncode = errors.New("transform: encode format not supported")

// Encode writes img to w in the requested Format. The Format
// picks the codec; EncodeOpts tunes codec-specific parameters.
// A nil writer, nil image, FormatUnknown, or an unsupported
// target format all return a descriptive error.
func Encode(w io.Writer, img image.Image, f Format, opts EncodeOpts) error {
	if w == nil {
		return fmt.Errorf("%w: nil writer", ErrEncode)
	}
	if img == nil {
		return fmt.Errorf("%w: nil image", ErrEncode)
	}
	switch f {
	case FormatPNG:
		enc := &png.Encoder{CompressionLevel: opts.PNGCompression}
		if err := enc.Encode(w, img); err != nil {
			return fmt.Errorf("%w: %v", ErrEncode, err)
		}
		return nil
	case FormatJPEG:
		q := opts.JPEGQuality
		if q <= 0 {
			q = jpeg.DefaultQuality
		}
		if q > 100 {
			q = 100
		}
		if err := jpeg.Encode(w, img, &jpeg.Options{Quality: q}); err != nil {
			return fmt.Errorf("%w: %v", ErrEncode, err)
		}
		return nil
	case FormatGIF:
		n := opts.GIFNumColors
		if n <= 0 || n > 256 {
			n = 256
		}
		if err := gif.Encode(w, img, &gif.Options{NumColors: n}); err != nil {
			return fmt.Errorf("%w: %v", ErrEncode, err)
		}
		return nil
	case FormatWebP:
		return fmt.Errorf("%w: WebP encoding requires libwebp (CGo) — decode only", ErrUnsupportedEncode)
	case FormatUnknown:
		return fmt.Errorf("%w: FormatUnknown is not an output format", ErrEncode)
	}
	return fmt.Errorf("%w: format %d", ErrUnsupportedEncode, int(f))
}
