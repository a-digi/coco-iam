package recovery

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

const tokenBytes = 32

// GenerateToken returns a fresh base64url-encoded token. Plaintext
// goes into the email link; the hash goes to the store.
func GenerateToken() (string, error) {
	buf := make([]byte, tokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("recovery: token rng: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// HashToken is the SHA-256 (hex) of a plaintext token. Stored on
// password_recoveries.token_hash so a DB breach doesn't leak tokens.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
