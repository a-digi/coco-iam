package oauthserverwiring

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/a-digi/coco-iam/src/oauthserver"
)

func newStore(t *testing.T) *SessionStore {
	t.Helper()
	s, err := NewSessionStore("test-secret")
	if err != nil {
		t.Fatalf("NewSessionStore: %v", err)
	}
	return s
}

func TestNewSessionStore_RejectsEmptySecret(t *testing.T) {
	if _, err := NewSessionStore(""); err == nil {
		t.Error("empty secret should be rejected")
	}
	if _, err := NewSessionStore("   "); err == nil {
		t.Error("whitespace-only secret should be rejected")
	}
}

func TestSessionStore_RoundTrip(t *testing.T) {
	s := newStore(t)
	cookie, err := s.Issue("user-1", "org-1")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if cookie.Name != SessionCookieName {
		t.Errorf("cookie name: %q", cookie.Name)
	}
	if !cookie.HttpOnly || !cookie.Secure {
		t.Error("session cookie must be HttpOnly + Secure")
	}
	got, err := s.Verify(cookie.Value)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if got.UserID != "user-1" || got.OrganizationID != "org-1" {
		t.Errorf("payload mismatch: %+v", got)
	}
}

func TestSessionStore_TamperedTagRejected(t *testing.T) {
	s := newStore(t)
	cookie, _ := s.Issue("user-1", "org-1")
	// Flip the last character of the cookie value (the tag).
	tampered := cookie.Value[:len(cookie.Value)-1] + flipB64Char(cookie.Value[len(cookie.Value)-1])
	if _, err := s.Verify(tampered); err == nil {
		t.Error("tampered tag must fail verify")
	}
}

func TestSessionStore_TamperedPayloadRejected(t *testing.T) {
	s := newStore(t)
	cookie, _ := s.Issue("user-1", "org-1")
	parts := strings.SplitN(cookie.Value, ".", 2)
	if len(parts) != 2 {
		t.Fatalf("cookie format wrong: %s", cookie.Value)
	}
	// Flip a character in the payload portion.
	bad := parts[0][:len(parts[0])-1] + flipB64Char(parts[0][len(parts[0])-1]) + "." + parts[1]
	if _, err := s.Verify(bad); err == nil {
		t.Error("tampered payload must fail verify")
	}
}

func TestSessionStore_ExpiredRejected(t *testing.T) {
	s := newStore(t)
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	s.Now = func() time.Time { return now }
	cookie, _ := s.Issue("user-1", "org-1")
	// Move past expiry.
	s.Now = func() time.Time { return now.Add(SessionTTL + time.Minute) }
	if _, err := s.Verify(cookie.Value); err == nil {
		t.Error("expired session must fail verify")
	}
}

func TestSessionStore_DifferentSecretRejected(t *testing.T) {
	s, _ := NewSessionStore("secret-A")
	cookie, _ := s.Issue("u", "o")
	other, _ := NewSessionStore("secret-B")
	if _, err := other.Verify(cookie.Value); err == nil {
		t.Error("different secret must fail verify")
	}
}

func TestSessionStore_EmptyValueRejected(t *testing.T) {
	s := newStore(t)
	if _, err := s.Verify(""); err == nil {
		t.Error("empty value must fail")
	}
	if _, err := s.Verify("not-even-dotted"); err == nil {
		t.Error("malformed value must fail")
	}
}

func TestSessionAuthenticator_NoCookieReturnsEmpty(t *testing.T) {
	a := &SessionAuthenticator{Store: newStore(t)}
	user, org, err := a.CurrentUser(context.Background(), oauthserver.RequestInfo{})
	if err != nil {
		t.Errorf("no cookie should not error, got %v", err)
	}
	if user != "" || org != "" {
		t.Errorf("no cookie should return empty: %q %q", user, org)
	}
}

func TestSessionAuthenticator_HappyPath(t *testing.T) {
	store := newStore(t)
	cookie, _ := store.Issue("user-1", "org-1")
	a := &SessionAuthenticator{Store: store}
	user, org, err := a.CurrentUser(context.Background(), oauthserver.RequestInfo{CookieValue: cookie.Value})
	if err != nil {
		t.Fatalf("CurrentUser: %v", err)
	}
	if user != "user-1" || org != "org-1" {
		t.Errorf("got user=%q org=%q", user, org)
	}
}

func TestSessionStore_Clear(t *testing.T) {
	s := newStore(t)
	c := s.Clear()
	if c.MaxAge != -1 || c.Value != "" {
		t.Errorf("clear cookie should expire immediately: %+v", c)
	}
}

func flipB64Char(c byte) string {
	if c == 'A' {
		return "B"
	}
	return "A"
}
