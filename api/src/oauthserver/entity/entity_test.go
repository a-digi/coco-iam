package entity

import (
	"testing"
)

// Pure-type tests for the entity package — exercise every
// method that makes a decision so handler logic downstream
// can trust these as primitives.

func TestClient_PermitsRedirect_ExactMatch(t *testing.T) {
	c := &Client{RedirectURIs: []string{
		"https://app.example/cb",
		"http://localhost:3000/callback",
	}}
	cases := []struct {
		uri  string
		want bool
	}{
		{"https://app.example/cb", true},
		{"http://localhost:3000/callback", true},
		{"https://app.example/cb/", false},               // trailing slash differs
		{"https://app.example/CB", false},                // case-sensitive path
		{"https://app.example/cb?x=1", false},            // query strings differ
		{"https://app.EXAMPLE/cb", false},                // host case-sensitive (per our rule)
		{"https://attacker.example/cb", false},
		{"", false},
	}
	for _, tc := range cases {
		t.Run(tc.uri, func(t *testing.T) {
			if got := c.PermitsRedirect(tc.uri); got != tc.want {
				t.Errorf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestClient_IsPublic(t *testing.T) {
	if !(&Client{Type: ClientTypePublic}).IsPublic() {
		t.Error("public client reported as not public")
	}
	if (&Client{Type: ClientTypeConfidential}).IsPublic() {
		t.Error("confidential client reported as public")
	}
	// Zero value defaults to "not public" — safer.
	if (&Client{}).IsPublic() {
		t.Error("zero-value client should not be public")
	}
}

func TestRefreshToken_IsRevoked(t *testing.T) {
	cases := []struct {
		name string
		r    RefreshToken
		want bool
	}{
		{"fresh", RefreshToken{RevokedAt: ""}, false},
		{"whitespace-only", RefreshToken{RevokedAt: "   "}, false},
		{"marked-revoked", RefreshToken{RevokedAt: "2026-04-24T12:00:00Z"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.r.IsRevoked(); got != tc.want {
				t.Errorf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestConsent_IsRevoked(t *testing.T) {
	if (&Consent{}).IsRevoked() {
		t.Error("fresh consent should not be revoked")
	}
	if !(&Consent{RevokedAt: "2026-04-24T12:00:00Z"}).IsRevoked() {
		t.Error("marked-revoked consent not reported as revoked")
	}
}

func TestOAuthError_ErrorStringCarriesCodeAndDescription(t *testing.T) {
	err := NewOAuthError(ErrCodeInvalidRequest, "missing client_id", 400)
	got := err.Error()
	if got != "invalid_request: missing client_id" {
		t.Errorf("unexpected string: %q", got)
	}
}

func TestOAuthError_ErrorStringWithoutDescription(t *testing.T) {
	err := NewOAuthError(ErrCodeServerError, "", 500)
	if err.Error() != "server_error" {
		t.Errorf("expected bare code when description empty, got %q", err.Error())
	}
}
