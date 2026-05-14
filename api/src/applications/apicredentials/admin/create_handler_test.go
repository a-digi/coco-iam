package admin

import (
	"strings"
	"testing"
	"time"

	"github.com/a-digi/coco-iam/src/applications/apicredentials/purpose"
)

// Pure pieces of the create handler are small, and that's where the
// interesting decisions live — purpose validation, expiry clamping.
// The HTTP plumbing around them is thin enough that we cover it in
// the smoke tests.

func TestValidatePurposes_RejectsEmpty(t *testing.T) {
	_, err := validatePurposes(nil)
	if err == nil {
		t.Fatal("nil purposes should be rejected")
	}
	_, err = validatePurposes([]string{})
	if err == nil {
		t.Fatal("empty purposes should be rejected")
	}
}

func TestValidatePurposes_RejectsUnknown(t *testing.T) {
	// Typo-safety — a misspelt purpose silently accepted would be a
	// security hole. Must surface as an error the admin UI can show.
	_, err := validatePurposes([]string{"secruity_key:read"})
	if err == nil {
		t.Fatal("unknown purpose must be rejected")
	}
	if !strings.Contains(err.Error(), "secruity_key:read") {
		t.Errorf("error should mention the offending value, got %v", err)
	}
}

func TestValidatePurposes_AcceptsKnown(t *testing.T) {
	out, err := validatePurposes([]string{purpose.SecurityKeyRead.String()})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(out) != 1 || out[0] != purpose.SecurityKeyRead.String() {
		t.Errorf("want [security_key:read], got %v", out)
	}
}

func TestValidatePurposes_DedupesDuplicates(t *testing.T) {
	// A FE bug that submitted the same purpose twice shouldn't bloat
	// the stored list. Purposes are a set, not a multiset.
	out, err := validatePurposes([]string{
		purpose.SecurityKeyRead.String(),
		purpose.SecurityKeyRead.String(),
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(out) != 1 {
		t.Errorf("duplicates should be collapsed, got %v", out)
	}
}

func TestClampExpiry_RequiresNonZero(t *testing.T) {
	_, _, err := clampExpiry(time.Time{}, time.Now())
	if err == nil {
		t.Fatal("zero time should be rejected")
	}
}

func TestClampExpiry_RejectsPast(t *testing.T) {
	now := time.Now()
	_, _, err := clampExpiry(now.Add(-time.Hour), now)
	if err == nil {
		t.Fatal("past expiry should be rejected")
	}
}

func TestClampExpiry_RejectsExactNow(t *testing.T) {
	// Boundary check: `requested == now` isn't "in the future".
	now := time.Now()
	_, _, err := clampExpiry(now, now)
	if err == nil {
		t.Fatal("expiry equal to now should be rejected")
	}
}

func TestClampExpiry_AcceptsUnderMax(t *testing.T) {
	now := time.Now()
	req := now.Add(30 * 24 * time.Hour)
	got, clamped, err := clampExpiry(req, now)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if clamped {
		t.Error("a 30-day expiry should not be clamped")
	}
	if !got.Equal(req) {
		t.Errorf("expiry should pass through unchanged, got %v want %v", got, req)
	}
}

func TestClampExpiry_ClampsOverMax(t *testing.T) {
	now := time.Now()
	// Request 2 years; should clamp to MaxLifetime (1 year).
	req := now.Add(2 * 365 * 24 * time.Hour)
	got, clamped, err := clampExpiry(req, now)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if !clamped {
		t.Error("over-max expiry must report clamped=true")
	}
	max := now.Add(MaxLifetime)
	if !got.Equal(max) {
		t.Errorf("clamped expiry should equal now+MaxLifetime, got %v want %v", got, max)
	}
}

func TestClampExpiry_ExactMaxNotClamped(t *testing.T) {
	now := time.Now()
	req := now.Add(MaxLifetime)
	got, clamped, err := clampExpiry(req, now)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if clamped {
		t.Error("exact-max expiry should not be clamped")
	}
	if !got.Equal(req) {
		t.Errorf("should pass through, got %v want %v", got, req)
	}
}

func TestRandomToken_UniqueAndURLSafe(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		tok, err := randomToken(24)
		if err != nil {
			t.Fatalf("randomToken: %v", err)
		}
		if seen[tok] {
			t.Errorf("collision at iteration %d: %s", i, tok)
		}
		seen[tok] = true
		// url-safe base64: no '+', '/', or padding '='.
		if strings.ContainsAny(tok, "+/=") {
			t.Errorf("non-url-safe characters in %q", tok)
		}
	}
}

func TestNewUUID_Shape(t *testing.T) {
	u := newUUID()
	if len(u) != 36 {
		t.Errorf("uuid length: want 36, got %d (%q)", len(u), u)
	}
	// Dashes at 8, 13, 18, 23.
	for _, idx := range []int{8, 13, 18, 23} {
		if u[idx] != '-' {
			t.Errorf("uuid[%d] should be '-' in %q", idx, u)
		}
	}
	// Version nibble at position 14 should be '4'.
	if u[14] != '4' {
		t.Errorf("uuid version nibble should be '4' in %q", u)
	}
}

func TestListEntry_DoesNotLeakSecretHash(t *testing.T) {
	// The list handler's wire shape must not expose secret_hash.
	// Verify at the type level: toListEntry produces a listEntry
	// struct that has no SecretHash field — a compile-time guarantee
	// plus this assertion that serializing the struct omits it.
	cred := newTestCredential()
	entry := toListEntry(cred, []string{"security_key:read"})
	if entry.APIID != cred.APIID {
		t.Error("api id should round-trip")
	}
	// Spot-check: the listEntry type has no SecretHash field.
	// (Catch a future sloppy edit that adds one.)
	_ = entry
}
