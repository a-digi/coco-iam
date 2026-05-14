// Package tokenid mints + hashes opaque OAuth tokens
// (authorization codes, refresh tokens). The raw token is a
// base64url string of random bytes; we store only its SHA-256
// so a DB snapshot can't be used to impersonate anyone.
package tokenid

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
)

// DefaultLenBytes is the raw-entropy length for opaque tokens.
// 32 bytes → 43-char base64url — plenty of bits, fits neatly in
// HTTP headers and URLs.
const DefaultLenBytes = 32

// ErrBadLength is returned when the caller asks for a non-
// positive length.
var ErrBadLength = errors.New("tokenid: length must be positive")

// randReader is the entropy source. Overridable in tests so we
// can seed deterministically without plumbing io.Reader through
// every call site.
var randReader io.Reader = rand.Reader

// Generate returns a freshly random base64url token of the
// given raw-byte length. Pass 0 for DefaultLenBytes.
func Generate(lenBytes int) (string, error) {
	if lenBytes < 0 {
		return "", ErrBadLength
	}
	if lenBytes == 0 {
		lenBytes = DefaultLenBytes
	}
	buf := make([]byte, lenBytes)
	if _, err := io.ReadFull(randReader, buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// Hash returns the SHA-256 of the token as base64url. Storage
// path: caller computes Hash(raw) once at issue time, persists
// the hash, and recomputes Hash(raw) on lookup.
func Hash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
