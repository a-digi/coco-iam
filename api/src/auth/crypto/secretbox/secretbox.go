// Package secretbox is the project's symmetric-encryption helper.
// Stores secrets (e.g. OAuth client secrets) in the DB as
// AES-256-GCM ciphertext with a random 12-byte nonce prepended,
// all base64-url-encoded for easy column storage.
//
// The master key is sourced in this priority order:
//  1. Env var COCO_IAM_MASTER_KEY — 32 raw bytes base64-std-
//     encoded. Required in production.
//  2. Dev fallback — a deterministic key derived from the
//     constant "coco-iam-dev-master". Good enough for local
//     development but MUST NEVER ship a prod deployment.
//
// The fallback is logged once at process start so operators
// notice if prod somehow lands on it.
package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
)

// DevMasterKeySeed is the string fed through SHA-256 to produce
// the dev-fallback key. Exported so test code can assert it.
const DevMasterKeySeed = "coco-iam-dev-master"

// EnvVarMasterKey names the env var the operator sets in prod.
const EnvVarMasterKey = "COCO_IAM_MASTER_KEY"

var (
	ErrMissingKey        = errors.New("secretbox: master key not configured and dev fallback disabled")
	ErrDecrypt           = errors.New("secretbox: decrypt failed")
	ErrCiphertextMalformed = errors.New("secretbox: ciphertext malformed")
)

// DisableDevFallback lets tests or prod startup code turn the
// fallback off — after calling this, a missing env var means
// ErrMissingKey on every operation instead of a silent derive.
var DisableDevFallback bool

var (
	once sync.Once
	key  []byte
	keyErr error
)

func loadKey() ([]byte, error) {
	once.Do(func() {
		if raw := os.Getenv(EnvVarMasterKey); raw != "" {
			decoded, err := base64.StdEncoding.DecodeString(raw)
			if err != nil {
				keyErr = fmt.Errorf("secretbox: COCO_IAM_MASTER_KEY must be base64: %w", err)
				return
			}
			if len(decoded) != 32 {
				keyErr = fmt.Errorf("secretbox: COCO_IAM_MASTER_KEY must decode to 32 bytes, got %d", len(decoded))
				return
			}
			key = decoded
			return
		}
		if DisableDevFallback {
			keyErr = ErrMissingKey
			return
		}
		// Dev fallback — derive a stable 32-byte key from a
		// constant seed. Suitable for local development only.
		sum := sha256.Sum256([]byte(DevMasterKeySeed))
		key = sum[:]
	})
	return key, keyErr
}

// resetForTest clears the lazily-loaded key so tests can switch
// env-var states between cases. Package-internal only.
func resetForTest() {
	once = sync.Once{}
	key = nil
	keyErr = nil
}

// Encrypt returns the base64-url-encoded ciphertext of plaintext
// under the master key. Nonce is freshly random per call.
func Encrypt(plaintext string) (string, error) {
	k, err := loadKey()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(k)
	if err != nil {
		return "", fmt.Errorf("secretbox: cipher init: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("secretbox: gcm init: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("secretbox: nonce: %w", err)
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

// Decrypt is the inverse of Encrypt. Returns ErrDecrypt when the
// ciphertext is authentic but the wrong key is used (or the
// value has been tampered with), and ErrCiphertextMalformed for
// structural problems.
func Decrypt(ciphertextB64 string) (string, error) {
	k, err := loadKey()
	if err != nil {
		return "", err
	}
	raw, err := base64.RawURLEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return "", fmt.Errorf("%w: base64: %v", ErrCiphertextMalformed, err)
	}
	block, err := aes.NewCipher(k)
	if err != nil {
		return "", fmt.Errorf("secretbox: cipher init: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("secretbox: gcm init: %w", err)
	}
	if len(raw) < gcm.NonceSize() {
		return "", fmt.Errorf("%w: too short", ErrCiphertextMalformed)
	}
	nonce, payload := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, payload, nil)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrDecrypt, err)
	}
	return string(plain), nil
}

// MaskSecret returns a UI-safe placeholder for a secret value.
// We never round-trip the actual secret through admin list
// responses; the admin UI shows this mask + an "update secret"
// flow for rotation.
func MaskSecret() string { return "••••••••" }
