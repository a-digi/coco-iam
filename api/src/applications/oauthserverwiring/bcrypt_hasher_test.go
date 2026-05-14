package oauthserverwiring

import "testing"

func TestBcryptHasher_HashVerifyRoundTrip(t *testing.T) {
	// Min cost for fast test execution. Production wiring uses
	// bcrypt.DefaultCost; the hasher respects whatever the
	// caller passes.
	h := NewBcryptHasher(4)
	hashed, err := h.Hash("s3cret")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if hashed == "s3cret" {
		t.Error("hashed output must not equal plaintext")
	}
	if err := h.Verify(hashed, "s3cret"); err != nil {
		t.Fatalf("matching Verify should succeed, got %v", err)
	}
	if err := h.Verify(hashed, "wrong"); err == nil {
		t.Error("wrong password should fail verify")
	}
}

func TestBcryptHasher_DefaultCostWhenZero(t *testing.T) {
	h := NewBcryptHasher(0)
	if h.Cost == 0 {
		t.Errorf("zero cost should be replaced by bcrypt default, got %d", h.Cost)
	}
}

func TestBcryptHasher_MalformedHashRejects(t *testing.T) {
	h := NewBcryptHasher(4)
	if err := h.Verify("not-a-bcrypt-hash", "x"); err == nil {
		t.Error("malformed hash should fail verify")
	}
}
