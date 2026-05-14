package bcrypt

import (
	"testing"
)

func TestHashAndVerify(t *testing.T) {
	pw := "S3cureP@ssw0rd"
	hash, err := HashPassword(pw, 10)
	if err != nil {
		t.Fatalf("hash error: %v", err)
	}
	if hash == "" {
		t.Fatalf("empty hash")
	}
	if err := VerifyPassword(hash, pw); err != nil {
		t.Fatalf("verify should succeed: %v", err)
	}
	if err := VerifyPassword(hash, "wrong"); err == nil {
		t.Fatalf("verify should fail for wrong password")
	}
}

func TestNeedsRehash(t *testing.T) {
	pw := "abc123"
	hash, err := HashPassword(pw, 8)
	if err != nil {
		t.Fatalf("hash error: %v", err)
	}
	need, err := NeedsRehash(hash, 12)
	if err != nil {
		t.Fatalf("needsrehash error: %v", err)
	}
	if !need {
		t.Fatalf("expected need rehash when desired cost is higher")
	}
}
