package transform

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"io"

	// Register image/gif + image/jpeg + image/png decoders for
	// image.Decode's format dispatch. Blank imports only — we
	// call the stdlib's decoder registry through image.Decode.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	// Register the WebP decoder. Pure-Go, decode-only — matches
	// our "no CGo" rule. Encoding WebP is not supported in this
	// package; see Encode.
	_ "golang.org/x/image/webp"
)

// ErrDecode is wrapped around the underlying image.Decode error
// so callers can distinguish a decode failure from, say, an I/O
// failure on the reader.
var ErrDecode = errors.New("transform: decode failed")

// Decode reads an image from r and reports what format the bytes
// actually were (sniffed via the standard registry — the client-
// supplied MIME / file extension is NOT trusted). An unreadable
// stream, a truncated file, or bytes whose magic number doesn't
// belong to any registered decoder returns an error that wraps
// ErrDecode.
//
// Animated GIFs are decoded as their FIRST FRAME ONLY. This
// package does not preserve animation — callers that need it
// should decode via image/gif directly.
func Decode(r io.Reader) (image.Image, Format, error) {
	if r == nil {
		return nil, FormatUnknown, fmt.Errorf("%w: nil reader", ErrDecode)
	}
	img, name, err := image.Decode(r)
	if err != nil {
		return nil, FormatUnknown, fmt.Errorf("%w: %v", ErrDecode, err)
	}
	return img, formatFromRegistryName(name), nil
}

// DecodeBytes is a thin convenience around Decode for callers
// holding a byte slice. Equivalent to
// Decode(bytes.NewReader(buf)).
func DecodeBytes(buf []byte) (image.Image, Format, error) {
	return Decode(bytes.NewReader(buf))
}

// formatFromRegistryName maps the lower-case format name the
// stdlib's image.Decode returns ("png", "jpeg", "gif", "webp") to
// our Format enum. Unknown names fall through to FormatUnknown.
func formatFromRegistryName(name string) Format {
	switch name {
	case "png":
		return FormatPNG
	case "jpeg":
		return FormatJPEG
	case "gif":
		return FormatGIF
	case "webp":
		return FormatWebP
	}
	return FormatUnknown
}
