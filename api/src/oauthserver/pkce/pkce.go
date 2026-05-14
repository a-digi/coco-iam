// Package pkce implements the RFC 7636 code-challenge
// verification path. Only S256 is supported — the spec's
// "plain" method is deprecated and we reject it at the edge.
//
// No secrets live here. The module is intentionally tiny so it
// can be pulled into any caller that needs the same check.
package pkce

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"strings"
)

// MethodS256 is the only allowed challenge method.
const MethodS256 = "S256"

// ErrBadVerifier is returned when the verifier fails any of
// the structural checks (length, charset) or the hash mismatch.
// Single error so callers can't accidentally leak the reason
// back to the OAuth client — a differentiating response would
// leak information about the verifier.
var ErrBadVerifier = errors.New("pkce: verifier invalid")

// ErrUnsupportedMethod is returned when the client used a
// method other than S256.
var ErrUnsupportedMethod = errors.New("pkce: only S256 is supported")

// VerifierCharset is the RFC 7636 §4.1 allowed character set
// (A-Z, a-z, 0-9, "-._~"). Exported for tests.
const VerifierCharset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~"

// Verifier length bounds per RFC 7636 §4.1.
const (
	MinVerifierLen = 43
	MaxVerifierLen = 128
)

// DeriveChallenge returns the S256 challenge for verifier. The
// authorization server stores the challenge at /authorize and
// the client replays the verifier at /token; DeriveChallenge(v)
// must equal the stored challenge.
func DeriveChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// Verify reports (nil, true) when the supplied verifier matches
// the stored challenge under the declared method. Any mismatch
// returns ErrBadVerifier; any unknown method returns
// ErrUnsupportedMethod. Uses constant-time comparison so a
// timing-observing client can't learn the stored challenge.
func Verify(storedChallenge, method, verifier string) error {
	if method != "" && method != MethodS256 {
		return ErrUnsupportedMethod
	}
	if !isValidVerifier(verifier) {
		return ErrBadVerifier
	}
	computed := DeriveChallenge(verifier)
	if subtle.ConstantTimeCompare([]byte(computed), []byte(storedChallenge)) != 1 {
		return ErrBadVerifier
	}
	return nil
}

// isValidVerifier enforces the RFC 7636 §4.1 charset + length
// bounds so we reject pathological inputs before hashing.
func isValidVerifier(v string) bool {
	n := len(v)
	if n < MinVerifierLen || n > MaxVerifierLen {
		return false
	}
	for i := 0; i < n; i++ {
		if !strings.ContainsRune(VerifierCharset, rune(v[i])) {
			return false
		}
	}
	return true
}
