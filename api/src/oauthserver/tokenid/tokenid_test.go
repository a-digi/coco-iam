package tokenid

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestGenerate_DefaultLength(t *testing.T) {
	tok, err := Generate(0)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	// 32 bytes → 43-char base64url.
	if len(tok) != 43 {
		t.Errorf("unexpected length %d for default 32-byte token", len(tok))
	}
}

func TestGenerate_CustomLengthRespected(t *testing.T) {
	tok, err := Generate(16)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	// 16 bytes → 22 chars (base64url no-pad).
	if len(tok) != 22 {
		t.Errorf("unexpected length %d for 16-byte token", len(tok))
	}
}

func TestGenerate_RejectsNegativeLength(t *testing.T) {
	_, err := Generate(-1)
	if !errors.Is(err, ErrBadLength) {
		t.Fatalf("want ErrBadLength, got %v", err)
	}
}

func TestGenerate_PropagatesReaderError(t *testing.T) {
	orig := randReader
	t.Cleanup(func() { randReader = orig })
	randReader = errReader{}
	_, err := Generate(0)
	if err == nil {
		t.Fatal("expected entropy error to propagate")
	}
}

func TestGenerate_DistinctPerCall(t *testing.T) {
	// Trivial entropy check — two consecutive calls should
	// never collide. Not a full randomness test, just a
	// smoke check that we aren't seeding deterministically
	// in production.
	seen := map[string]struct{}{}
	for i := 0; i < 16; i++ {
		tok, err := Generate(0)
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		if _, dup := seen[tok]; dup {
			t.Fatalf("collision after %d calls: %q", i, tok)
		}
		seen[tok] = struct{}{}
	}
}

func TestHash_DeterministicAndSHA256Length(t *testing.T) {
	a := Hash("hello")
	b := Hash("hello")
	if a != b {
		t.Error("Hash should be deterministic for the same input")
	}
	// SHA-256 → 32 bytes → 43-char base64url no-pad.
	if len(a) != 43 {
		t.Errorf("unexpected hash length %d", len(a))
	}
	if Hash("hello") == Hash("world") {
		t.Error("different inputs should hash distinctly")
	}
}

func TestHash_RawTokenNotPresentInHash(t *testing.T) {
	// Security invariant: the hash must not literally contain
	// the raw token. If someone changes Hash to a no-op or a
	// weak transform, this catches it early.
	raw := strings.Repeat("a", 40)
	if bytes.Contains([]byte(Hash(raw)), []byte(raw)) {
		t.Error("Hash leaks the raw token")
	}
}

// errReader is a failing io.Reader used to exercise the error
// path inside Generate.
type errReader struct{}

func (errReader) Read(_ []byte) (int, error) { return 0, io.ErrUnexpectedEOF }
