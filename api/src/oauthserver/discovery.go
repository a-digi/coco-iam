package oauthserver

import (
	"encoding/json"
	"net/http"
	"strings"
)

// DiscoveryMetadata is the response shape of
// /.well-known/openid-configuration (OIDC Discovery 1.0) and
// (a superset of) RFC 8414 §3 OAuth 2.0 Authorization Server
// Metadata. Only fields we actually advertise are populated;
// JSON tags use omitempty so the wire response only carries
// non-zero values.
type DiscoveryMetadata struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	UserinfoEndpoint                  string   `json:"userinfo_endpoint,omitempty"`
	RevocationEndpoint                string   `json:"revocation_endpoint,omitempty"`
	IntrospectionEndpoint             string   `json:"introspection_endpoint,omitempty"`
	JWKSURI                           string   `json:"jwks_uri"`
	ScopesSupported                   []string `json:"scopes_supported,omitempty"`
	ResponseTypesSupported            []string `json:"response_types_supported"`
	GrantTypesSupported               []string `json:"grant_types_supported"`
	SubjectTypesSupported             []string `json:"subject_types_supported"`
	IDTokenSigningAlgValuesSupported  []string `json:"id_token_signing_alg_values_supported"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
	CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported"`
	ClaimsSupported                   []string `json:"claims_supported,omitempty"`
}

// BuildDiscovery constructs the metadata document for a single
// issuer. Pure function — no I/O — so handler tests can assert
// on the wire shape without spinning a server.
func BuildDiscovery(issuer, basePath string, scopesSupported []string) DiscoveryMetadata {
	base := strings.TrimRight(issuer, "/")
	bp := strings.TrimRight(basePath, "/")
	return DiscoveryMetadata{
		Issuer:                base,
		AuthorizationEndpoint: base + bp + "/oauth/authorize",
		TokenEndpoint:         base + bp + "/oauth/token",
		UserinfoEndpoint:      base + bp + "/oauth/userinfo",
		RevocationEndpoint:    base + bp + "/oauth/revoke",
		IntrospectionEndpoint: base + bp + "/oauth/introspect",
		JWKSURI:               base + bp + "/.well-known/jwks.json",
		ScopesSupported:       scopesSupported,
		ResponseTypesSupported: []string{"code"},
		GrantTypesSupported: []string{
			"authorization_code", "refresh_token",
		},
		SubjectTypesSupported:            []string{"public"},
		IDTokenSigningAlgValuesSupported: []string{"RS256"},
		TokenEndpointAuthMethodsSupported: []string{
			"client_secret_post", "none",
		},
		CodeChallengeMethodsSupported: []string{"S256"},
		ClaimsSupported: []string{
			"sub", "iss", "aud", "exp", "iat", "nonce", "at_hash",
			"email", "email_verified",
			"name", "given_name", "family_name", "picture",
			"updated_at", "locale", "zoneinfo",
		},
	}
}

// DiscoveryHandler serves GET /.well-known/openid-configuration.
// Wiring layer supplies the issuer + base path closures so the
// library doesn't carry slug-routing concerns.
type DiscoveryHandler struct {
	IssuerFromRequest   func(r *http.Request) string
	BasePathFromRequest func(r *http.Request) string
	// ScopesSupported is the static list of scope names the
	// admin marked exposable. Wiring filters to the active
	// `application_scopes` rows where expose_over_oauth = true.
	ScopesSupported []string
}

// @Summary     OpenID Connect discovery document
// @Description Returns the OIDC discovery metadata for the application. Standard .well-known/openid-configuration endpoint.
// @Tags        oauth
// @Produce     json
// @Success     200 {object} DiscoveryMetadata
// @Router      /a/{orgSlug}/{wsSlug}/{appSlug}/.well-known/openid-configuration [get]
func (h *DiscoveryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.IssuerFromRequest == nil || h.BasePathFromRequest == nil {
		writeJSONError(w, "server_error", "discovery handler not configured", http.StatusInternalServerError)
		return
	}
	if r.Method != http.MethodGet {
		writeJSONError(w, "invalid_request", "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	meta := BuildDiscovery(h.IssuerFromRequest(r), h.BasePathFromRequest(r), h.ScopesSupported)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=300")
	// OIDC discovery documents are standardly fetched cross-origin by
	// relying parties — this endpoint previously had no CORS headers
	// at all, silently blocking that. A static "*" (not a per-origin
	// reflection) needs no Vary: Origin — the response never differs
	// by request Origin, so there's nothing a shared cache could
	// serve inconsistently across origins. See
	// plan/todo/security/header-and-cache-poisoning.md.
	w.Header().Set("Access-Control-Allow-Origin", "*")
	_ = json.NewEncoder(w).Encode(meta)
}
