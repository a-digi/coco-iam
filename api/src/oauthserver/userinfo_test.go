package oauthserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeVerifier struct {
	user   string
	scopes []string
	err    error
}

func (f *fakeVerifier) VerifyAccessToken(_ context.Context, _, _ string) (string, []string, error) {
	return f.user, f.scopes, f.err
}

func TestUserinfo_HappyPathReturnsAllLoadedClaims(t *testing.T) {
	// Scopes do not include "profile" — userinfo returns everything
	// LoadClaims provides regardless, because the claims reader is
	// the authoritative filter, not the scope list.
	v := &fakeVerifier{user: "user-1", scopes: []string{"openid", "offline_access"}}
	c := &fakeClaims{extra: map[string]any{
		"email":              "alice@example.com",
		"email_verified":     true,
		"preferred_username": "alice",
	}}
	h := &UserinfoHandler{
		ApplicationIDFromRequest: func(_ *http.Request) (string, string, error) {
			return "app-1", "org-1", nil
		},
		Verifier: v, Claims: c,
	}
	r := httptest.NewRequest(http.MethodGet, "/oauth/userinfo", nil)
	r.Header.Set("Authorization", "Bearer ACCESS-JWT")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if out["sub"] != "user-1" {
		t.Errorf("sub: want user-1, got %v", out["sub"])
	}
	if out["email"] != "alice@example.com" {
		t.Errorf("email missing or wrong: %v", out["email"])
	}
	if out["preferred_username"] != "alice" {
		t.Errorf("preferred_username missing or wrong: %v", out["preferred_username"])
	}
	if out["email_verified"] != true {
		t.Errorf("email_verified missing or wrong: %v", out["email_verified"])
	}
}

func TestUserinfo_MissingBearerReturns401(t *testing.T) {
	h := &UserinfoHandler{
		ApplicationIDFromRequest: func(_ *http.Request) (string, string, error) {
			return "app-1", "org-1", nil
		},
		Verifier: &fakeVerifier{}, Claims: &fakeClaims{},
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/oauth/userinfo", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rec.Code)
	}
	if rec.Header().Get("WWW-Authenticate") == "" {
		t.Error("WWW-Authenticate header missing on 401")
	}
}

func TestUserinfo_InvalidTokenReturns401(t *testing.T) {
	v := &fakeVerifier{err: errors.New("expired")}
	h := &UserinfoHandler{
		ApplicationIDFromRequest: func(_ *http.Request) (string, string, error) {
			return "app-1", "org-1", nil
		},
		Verifier: v, Claims: &fakeClaims{},
	}
	r := httptest.NewRequest(http.MethodGet, "/oauth/userinfo", nil)
	r.Header.Set("Authorization", "Bearer EXPIRED")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rec.Code)
	}
}

func TestBearerFromRequest(t *testing.T) {
	cases := []struct {
		header, want string
	}{
		{"", ""},
		{"Bearer abc", "abc"},
		{"Bearer  spaced  ", "spaced"},
		{"Basic xyz", ""},
		{"bearer abc", ""}, // case-sensitive prefix per spec
	}
	for _, tc := range cases {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		if tc.header != "" {
			r.Header.Set("Authorization", tc.header)
		}
		if got := bearerFromRequest(r); got != tc.want {
			t.Errorf("Authorization=%q got %q want %q", tc.header, got, tc.want)
		}
	}
}
