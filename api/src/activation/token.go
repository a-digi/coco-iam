package activation

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// tokenBytes = 32 random bytes → 43-char base64url (no padding) — opaque
// and unguessable, comfortably within typical URL length limits.
const tokenBytes = 32

// GenerateToken returns a fresh base64url-encoded token. The caller puts
// the plaintext in the email link and hands the hash (HashToken) to the
// store.
func GenerateToken() (string, error) {
	buf := make([]byte, tokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("activation: token rng: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// HashToken returns the hex-encoded SHA-256 of a plaintext token. Stored
// on `user_activations.token_hash` so a DB breach does not leak tokens.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// tempPasswordAlphabet — avoids visually ambiguous glyphs (0/O, 1/l/I)
// so admins can type the password from the email without mis-reading.
const tempPasswordAlphabet = "abcdefghjkmnpqrstuvwxyzABCDEFGHJKMNPQRSTUVWXYZ23456789"

const tempPasswordLength = 12

// GenerateTempPassword returns a random 12-char password drawn from a
// readability-filtered alphabet.
func GenerateTempPassword() (string, error) {
	buf := make([]byte, tempPasswordLength)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("activation: temp password rng: %w", err)
	}
	out := make([]byte, tempPasswordLength)
	for i, b := range buf {
		out[i] = tempPasswordAlphabet[int(b)%len(tempPasswordAlphabet)]
	}
	return string(out), nil
}
