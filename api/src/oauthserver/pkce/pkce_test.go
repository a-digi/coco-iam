package pkce

import (
	"errors"
	"strings"
	"testing"
)

// Known RFC 7636 Appendix B vector — pins that our S256
// derivation matches the spec exactly. Any drift here breaks
// every client.
func TestDeriveChallenge_KnownVector(t *testing.T) {
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	want := "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	if got := DeriveChallenge(verifier); got != want {
		t.Errorf("DeriveChallenge(%q) = %q, want %q", verifier, got, want)
	}
}

func TestVerify_Success(t *testing.T) {
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	stored := DeriveChallenge(verifier)
	if err := Verify(stored, MethodS256, verifier); err != nil {
		t.Fatalf("Verify should succeed, got %v", err)
	}
}

func TestVerify_SuccessDefaultsMethodToS256(t *testing.T) {
	// Empty method string is treated as S256 so clients that
	// omit code_challenge_method (some older libs do) still
	// work when the authorize request stored S256 implicitly.
	verifier := strings.Repeat("a", MinVerifierLen)
	stored := DeriveChallenge(verifier)
	if err := Verify(stored, "", verifier); err != nil {
		t.Fatalf("empty method must behave like S256, got %v", err)
	}
}

func TestVerify_RejectsPlainMethod(t *testing.T) {
	err := Verify("anything", "plain", "anything")
	if !errors.Is(err, ErrUnsupportedMethod) {
		t.Errorf("expected ErrUnsupportedMethod, got %v", err)
	}
}

func TestVerify_RejectsMismatch(t *testing.T) {
	verifier := strings.Repeat("a", MinVerifierLen)
	stored := DeriveChallenge(verifier + "x") // challenge for a different verifier
	err := Verify(stored, MethodS256, verifier)
	if !errors.Is(err, ErrBadVerifier) {
		t.Errorf("expected ErrBadVerifier on mismatch, got %v", err)
	}
}

func TestVerify_RejectsShortVerifier(t *testing.T) {
	short := strings.Repeat("a", MinVerifierLen-1)
	err := Verify(DeriveChallenge(short), MethodS256, short)
	if !errors.Is(err, ErrBadVerifier) {
		t.Errorf("short verifier should fail, got %v", err)
	}
}

func TestVerify_RejectsLongVerifier(t *testing.T) {
	long := strings.Repeat("a", MaxVerifierLen+1)
	err := Verify(DeriveChallenge(long), MethodS256, long)
	if !errors.Is(err, ErrBadVerifier) {
		t.Errorf("long verifier should fail, got %v", err)
	}
}

func TestVerify_RejectsIllegalCharset(t *testing.T) {
	// Space is not in the allowed RFC 7636 charset.
	bad := strings.Repeat("a", MinVerifierLen-1) + " "
	err := Verify(DeriveChallenge(bad), MethodS256, bad)
	if !errors.Is(err, ErrBadVerifier) {
		t.Errorf("illegal charset must fail, got %v", err)
	}
}

func TestVerifierCharset_Contains(t *testing.T) {
	// Trivial sanity — make sure the charset constant does
	// include the documented characters.
	for _, sample := range []rune{'A', 'z', '0', '-', '.', '_', '~'} {
		if !strings.ContainsRune(VerifierCharset, sample) {
			t.Errorf("charset missing %q", sample)
		}
	}
	// And doesn't include boundary-break characters.
	for _, sample := range []rune{' ', '\t', '+', '/', '='} {
		if strings.ContainsRune(VerifierCharset, sample) {
			t.Errorf("charset must NOT contain %q", sample)
		}
	}
}
