package oauthserverwiring

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/a-digi/coco-iam/src/oauthserver"
)

// SessionCookieName is the cookie the password-login flow sets
// after a successful authentication and the OAuth authorize
// handler reads to decide whether the caller already has a
// usable session. HTTP-only + SameSite=Lax + 15-minute lifetime
// per the plan.
const SessionCookieName = "coco_iam_auth"

// SessionTTL bounds how long a freshly-issued session cookie
// stays valid. Short by design — it covers the few minutes
// between a successful login and the OAuth client's redirect
// back to /authorize.
const SessionTTL = 15 * time.Minute

// SessionPayload is the data we encode into the cookie value.
// Compact JSON so the cookie stays small. Verified via HMAC-
// SHA256 with a server-wide secret pulled from config.json.
type SessionPayload struct {
	UserID         string `json:"u"`
	OrganizationID string `json:"o"`
	IssuedAt       int64  `json:"i"`
	ExpiresAt      int64  `json:"x"`
}

// SessionStore mints + verifies the cookie payload. The secret
// is the same HS256 secret coco-iam already uses for admin
// session tokens (`auth.hs256_secret` in config.json).
type SessionStore struct {
	Secret []byte
	Now    func() time.Time
}

// NewSessionStore returns a SessionStore over the given secret.
// Empty secret is rejected loudly so a misconfiguration can't
// silently degrade to "all sessions valid forever".
func NewSessionStore(secret string) (*SessionStore, error) {
	if strings.TrimSpace(secret) == "" {
		return nil, errors.New("oauthserverwiring: SessionStore requires a non-empty secret")
	}
	return &SessionStore{Secret: []byte(secret), Now: time.Now}, nil
}

// Issue mints a freshly-signed cookie value carrying the user
// id + organization id. Caller writes the returned http.Cookie
// onto the login response.
func (s *SessionStore) Issue(userID, organizationID string) (*http.Cookie, error) {
	now := s.Now().UTC()
	payload := SessionPayload{
		UserID:         userID,
		OrganizationID: organizationID,
		IssuedAt:       now.Unix(),
		ExpiresAt:      now.Add(SessionTTL).Unix(),
	}
	value, err := s.encode(payload)
	if err != nil {
		return nil, err
	}
	return &http.Cookie{
		Name:     SessionCookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Expires:  now.Add(SessionTTL),
		MaxAge:   int(SessionTTL.Seconds()),
	}, nil
}

// Verify decodes the cookie value and checks the HMAC tag +
// the expiry. Any tampering or expiry returns an error so the
// authorize handler treats the session as missing.
func (s *SessionStore) Verify(value string) (*SessionPayload, error) {
	if strings.TrimSpace(value) == "" {
		return nil, errors.New("oauthserverwiring: empty session value")
	}
	parts := strings.SplitN(value, ".", 2)
	if len(parts) != 2 {
		return nil, errors.New("oauthserverwiring: malformed session value")
	}
	payloadRaw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("oauthserverwiring: decode session payload: %w", err)
	}
	gotTag, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("oauthserverwiring: decode session tag: %w", err)
	}
	wantTag := s.tag(payloadRaw)
	if !hmac.Equal(gotTag, wantTag) {
		return nil, errors.New("oauthserverwiring: session signature mismatch")
	}
	var p SessionPayload
	if err := json.Unmarshal(payloadRaw, &p); err != nil {
		return nil, fmt.Errorf("oauthserverwiring: parse session payload: %w", err)
	}
	if s.Now().Unix() >= p.ExpiresAt {
		return nil, errors.New("oauthserverwiring: session expired")
	}
	if p.UserID == "" {
		return nil, errors.New("oauthserverwiring: session has no user id")
	}
	return &p, nil
}

// Clear returns the cookie that the caller should write to
// expire the session (logout flow).
func (s *SessionStore) Clear() *http.Cookie {
	return &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	}
}

func (s *SessionStore) encode(p SessionPayload) (string, error) {
	raw, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("oauthserverwiring: marshal session: %w", err)
	}
	tag := s.tag(raw)
	return base64.RawURLEncoding.EncodeToString(raw) + "." +
		base64.RawURLEncoding.EncodeToString(tag), nil
}

func (s *SessionStore) tag(payload []byte) []byte {
	mac := hmac.New(sha256.New, s.Secret)
	_, _ = mac.Write(payload)
	return mac.Sum(nil)
}

// SessionAuthenticator implements oauthserver.UserAuthenticator
// by delegating to a SessionStore.
type SessionAuthenticator struct {
	Store *SessionStore
}

// CurrentUser implements oauthserver.UserAuthenticator. Returns
// ("", "", nil) when no session is present so the authorize
// handler treats it as "redirect to login" rather than an
// internal failure.
func (a *SessionAuthenticator) CurrentUser(_ context.Context, info oauthserver.RequestInfo) (string, string, error) {
	if a.Store == nil || strings.TrimSpace(info.CookieValue) == "" {
		return "", "", nil
	}
	p, err := a.Store.Verify(info.CookieValue)
	if err != nil {
		return "", "", nil
	}
	return p.UserID, p.OrganizationID, nil
}

var _ oauthserver.UserAuthenticator = (*SessionAuthenticator)(nil)
