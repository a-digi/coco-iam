package transform

import "strings"

// Format enumerates the image encodings this package understands.
// FormatUnknown is the zero value; a decoder that can't sniff the
// input returns it alongside an error.
type Format int

const (
	FormatUnknown Format = iota
	FormatPNG
	FormatJPEG
	FormatGIF
	FormatWebP
)

// MIME returns the canonical MIME type for the format.
// FormatUnknown maps to "" so callers can detect it.
func (f Format) MIME() string {
	switch f {
	case FormatPNG:
		return "image/png"
	case FormatJPEG:
		return "image/jpeg"
	case FormatGIF:
		return "image/gif"
	case FormatWebP:
		return "image/webp"
	}
	return ""
}

// Ext returns the canonical file extension for the format without
// a leading dot. Empty for FormatUnknown.
func (f Format) Ext() string {
	switch f {
	case FormatPNG:
		return "png"
	case FormatJPEG:
		return "jpg"
	case FormatGIF:
		return "gif"
	case FormatWebP:
		return "webp"
	}
	return ""
}

// String lets Format print cleanly in tests and logs.
func (f Format) String() string {
	if m := f.MIME(); m != "" {
		return m
	}
	return "image/unknown"
}

// FormatFromMIME maps an "image/..." MIME type back to a Format.
// The match is case-insensitive and ignores any parameters (e.g.
// "; charset=binary"). An unrecognised MIME returns
// FormatUnknown — callers decide whether that's an error.
func FormatFromMIME(mime string) Format {
	mime = strings.TrimSpace(mime)
	if idx := strings.Index(mime, ";"); idx >= 0 {
		mime = strings.TrimSpace(mime[:idx])
	}
	switch strings.ToLower(mime) {
	case "image/png":
		return FormatPNG
	case "image/jpeg", "image/jpg":
		return FormatJPEG
	case "image/gif":
		return FormatGIF
	case "image/webp":
		return FormatWebP
	}
	return FormatUnknown
}
