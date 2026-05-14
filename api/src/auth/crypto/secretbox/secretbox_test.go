package secretbox

import (
	"encoding/base64"
	"errors"
	"os"
	"strings"
	"testing"
)

// These tests manipulate the package-level key cache via
// resetForTest, so they cannot run in parallel.

func TestEncryptDecrypt_RoundTripWithDevFallback(t *testing.T) {
	resetForTest()
	_ = os.Unsetenv(EnvVarMasterKey)
	DisableDevFallback = false

	plain := "hello world"
	ct, err := Encrypt(plain)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if ct == plain {
		t.Errorf("ciphertext must not equal plaintext")
	}
	got, err := Decrypt(ct)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if got != plain {
		t.Errorf("round-trip mismatch: got %q want %q", got, plain)
	}
}

func TestEncrypt_WithEnvVarUsesProvidedKey(t *testing.T) {
	resetForTest()
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = byte(i + 1)
	}
	t.Setenv(EnvVarMasterKey, base64.StdEncoding.EncodeToString(raw))
	DisableDevFallback = true
	t.Cleanup(func() { DisableDevFallback = false; resetForTest() })

	ct, err := Encrypt("secret")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	// Decrypt under the same key should work.
	if got, err := Decrypt(ct); err != nil || got != "secret" {
		t.Fatalf("round-trip broken: got=%q err=%v", got, err)
	}
}

func TestLoadKey_MissingEnvAndFallbackDisabledErrors(t *testing.T) {
	resetForTest()
	_ = os.Unsetenv(EnvVarMasterKey)
	DisableDevFallback = true
	t.Cleanup(func() { DisableDevFallback = false; resetForTest() })

	_, err := Encrypt("x")
	if !errors.Is(err, ErrMissingKey) {
		t.Fatalf("want ErrMissingKey, got %v", err)
	}
}

func TestLoadKey_BadBase64IsReported(t *testing.T) {
	resetForTest()
	t.Setenv(EnvVarMasterKey, "not-base64!!")
	t.Cleanup(func() { resetForTest() })

	_, err := Encrypt("x")
	if err == nil || !strings.Contains(err.Error(), "base64") {
		t.Fatalf("want base64-complaint error, got %v", err)
	}
}

func TestLoadKey_WrongLengthIsReported(t *testing.T) {
	resetForTest()
	// 16-byte key is valid base64 but wrong length for AES-256.
	t.Setenv(EnvVarMasterKey, base64.StdEncoding.EncodeToString(make([]byte, 16)))
	t.Cleanup(func() { resetForTest() })

	_, err := Encrypt("x")
	if err == nil || !strings.Contains(err.Error(), "32") {
		t.Fatalf("want length complaint, got %v", err)
	}
}

func TestDecrypt_CiphertextTamperedIsRejected(t *testing.T) {
	resetForTest()
	_ = os.Unsetenv(EnvVarMasterKey)
	DisableDevFallback = false

	ct, err := Encrypt("hello")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	// Flip the last base64 character — mangled ciphertext.
	tampered := ct[:len(ct)-1] + flipChar(ct[len(ct)-1])
	_, err = Decrypt(tampered)
	if err == nil {
		t.Fatal("tampered ciphertext should fail")
	}
}

func TestDecrypt_MalformedBase64(t *testing.T) {
	resetForTest()
	_ = os.Unsetenv(EnvVarMasterKey)
	_, err := Decrypt("***not-base64***")
	if !errors.Is(err, ErrCiphertextMalformed) {
		t.Errorf("want ErrCiphertextMalformed, got %v", err)
	}
}

func TestDecrypt_WrongKeyIsRejected(t *testing.T) {
	resetForTest()
	t.Setenv(EnvVarMasterKey, base64.StdEncoding.EncodeToString(make([]byte, 32)))
	ctA, _ := Encrypt("hello")

	resetForTest()
	altKey := make([]byte, 32)
	for i := range altKey {
		altKey[i] = 0xFF
	}
	t.Setenv(EnvVarMasterKey, base64.StdEncoding.EncodeToString(altKey))
	_, err := Decrypt(ctA)
	if !errors.Is(err, ErrDecrypt) {
		t.Fatalf("want ErrDecrypt when key mismatches, got %v", err)
	}
	t.Cleanup(func() { resetForTest() })
}

func TestMaskSecret_DoesNotLeak(t *testing.T) {
	m := MaskSecret()
	if strings.Contains(m, "secret") {
		t.Errorf("mask leaks word 'secret': %q", m)
	}
}

func flipChar(c byte) string {
	if c == 'A' {
		return "B"
	}
	return "A"
}
