package oauthserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBuildDiscovery_PopulatesEndpoints(t *testing.T) {
	meta := BuildDiscovery(
		"https://iam.example",
		"/a/acme/prod/web",
		[]string{"openid", "email", "profile"},
	)
	if meta.Issuer != "https://iam.example" {
		t.Errorf("issuer: %q", meta.Issuer)
	}
	if !strings.HasSuffix(meta.AuthorizationEndpoint, "/a/acme/prod/web/oauth/authorize") {
		t.Errorf("authorize endpoint: %q", meta.AuthorizationEndpoint)
	}
	if !strings.HasSuffix(meta.JWKSURI, "/a/acme/prod/web/.well-known/jwks.json") {
		t.Errorf("jwks: %q", meta.JWKSURI)
	}
	if len(meta.ScopesSupported) != 3 {
		t.Errorf("scopes: %v", meta.ScopesSupported)
	}
}

func TestBuildDiscovery_TrimsTrailingSlashes(t *testing.T) {
	meta := BuildDiscovery("https://iam.example/", "/a/acme/prod/web/", nil)
	if strings.Contains(meta.AuthorizationEndpoint, "//oauth") {
		t.Errorf("double slash from un-trimmed input: %q", meta.AuthorizationEndpoint)
	}
}

func TestBuildDiscovery_SupportsExpectedAlgsAndGrants(t *testing.T) {
	meta := BuildDiscovery("https://iam.example", "/a/acme/prod/web", nil)
	expectContains := func(label string, list []string, want string) {
		t.Helper()
		for _, v := range list {
			if v == want {
				return
			}
		}
		t.Errorf("%s: %v missing %q", label, list, want)
	}
	expectContains("response_types", meta.ResponseTypesSupported, "code")
	expectContains("grant_types", meta.GrantTypesSupported, "authorization_code")
	expectContains("grant_types", meta.GrantTypesSupported, "refresh_token")
	expectContains("alg", meta.IDTokenSigningAlgValuesSupported, "RS256")
	expectContains("auth_method", meta.TokenEndpointAuthMethodsSupported, "client_secret_post")
	expectContains("auth_method", meta.TokenEndpointAuthMethodsSupported, "none")
	expectContains("pkce", meta.CodeChallengeMethodsSupported, "S256")
}

func TestDiscoveryHandler_ServesJSON(t *testing.T) {
	h := &DiscoveryHandler{
		IssuerFromRequest:   func(_ *http.Request) string { return "https://iam.example" },
		BasePathFromRequest: func(_ *http.Request) string { return "/a/acme/prod/web" },
		ScopesSupported:     []string{"openid", "email"},
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/.well-known/openid-configuration", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("content-type: %q", rec.Header().Get("Content-Type"))
	}
	var out DiscoveryMetadata
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Issuer != "https://iam.example" {
		t.Errorf("issuer: %q", out.Issuer)
	}
}

func TestDiscoveryHandler_RejectsPost(t *testing.T) {
	h := &DiscoveryHandler{
		IssuerFromRequest:   func(_ *http.Request) string { return "x" },
		BasePathFromRequest: func(_ *http.Request) string { return "/x" },
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/.well-known/openid-configuration", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("want 405, got %d", rec.Code)
	}
}
