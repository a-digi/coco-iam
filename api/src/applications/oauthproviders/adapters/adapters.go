// Package adapters hosts the provider-specific HTTP clients that
// translate "(authorize URL, code exchange, userinfo fetch)" into
// the canonical entity.Identity shape the login handshake
// consumes. One adapter per IdP; the IdentityResolver interface
// is the narrow seam so the handler layer depends on it rather
// than the concrete types (ISP + DIP).
//
// All adapters are HTTP-only (no IdP SDKs) so we don't drag
// transitive deps for something that fits in a few hundred lines.
package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/a-digi/coco-iam/src/applications/oauthproviders/entity"
)

// IdentityResolver is the interface the login handshake consumes.
// Each concrete provider (Google / GitHub / Microsoft) implements
// it via an independent adapter struct; tests substitute fakes.
type IdentityResolver interface {
	// AuthorizeURL returns the URL the browser should be sent to.
	// state is the random CSRF token, codeChallenge is the
	// S256-hashed PKCE verifier, redirectURI is our callback.
	AuthorizeURL(cfg entity.ProviderConfig, state, codeChallenge, redirectURI string) (string, error)

	// ExchangeCode trades the authorization code for an access
	// token at the IdP's token endpoint. codeVerifier is the raw
	// PKCE verifier (S256 of which is the challenge); redirectURI
	// MUST exactly match the value used in AuthorizeURL.
	ExchangeCode(ctx context.Context, cfg entity.ProviderConfig, code, codeVerifier, redirectURI string) (accessToken, idToken string, err error)

	// FetchIdentity uses the access token to retrieve the user's
	// profile. Fields the IdP doesn't surface (e.g. GitHub without
	// profile scope) land as zero values; the caller handles
	// absence.
	FetchIdentity(ctx context.Context, cfg entity.ProviderConfig, accessToken, idToken string) (entity.Identity, error)
}

// ErrInvalidResponse surfaces when the IdP's response is
// syntactically wrong (non-JSON body, missing required fields).
// Callers map this to a 502-style error in the UI.
var ErrInvalidResponse = errors.New("adapters: invalid provider response")

// ErrTokenExchange surfaces when the IdP refuses the code
// exchange (code expired / already used / bad secret). Callers
// map this to a 400 + "please retry".
var ErrTokenExchange = errors.New("adapters: token exchange failed")

// ErrUserinfo surfaces when the userinfo fetch fails
// structurally (network, 5xx, malformed JSON).
var ErrUserinfo = errors.New("adapters: userinfo failed")

// defaultHTTPClient is used when an adapter doesn't override it.
// Short timeout because a slow IdP shouldn't hang our handler.
var defaultHTTPClient = &http.Client{Timeout: 10 * time.Second}

// For returns the resolver matching provider. Returns nil for an
// unknown provider so callers can 400 cleanly without a switch at
// the call site.
func For(provider entity.Provider) IdentityResolver {
	switch provider {
	case entity.ProviderGoogle:
		return &GoogleResolver{Client: defaultHTTPClient}
	case entity.ProviderGitHub:
		return &GitHubResolver{Client: defaultHTTPClient}
	case entity.ProviderMicrosoft:
		return &MicrosoftResolver{Client: defaultHTTPClient}
	}
	return nil
}

// -- helpers shared by all adapters --------------------------------

// postForm posts application/x-www-form-urlencoded to the IdP's
// token endpoint, decoding the JSON response into out. Providers
// all follow RFC 6749 here, so the helper is generic.
func postForm(ctx context.Context, client *http.Client, tokenURL string, form url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL,
		strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("adapters: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("adapters: http: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%w: status %d body %q", ErrTokenExchange, resp.StatusCode, string(body))
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidResponse, err)
	}
	return nil
}

// getJSON fetches a URL with a bearer token and decodes the JSON
// response into out. Used by every userinfo endpoint.
func getJSON(ctx context.Context, client *http.Client, u, token string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("adapters: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("adapters: http: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%w: status %d body %q", ErrUserinfo, resp.StatusCode, string(body))
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidResponse, err)
	}
	return nil
}

// defaultScopes picks a sensible fallback when the admin hasn't
// configured any scopes for the provider.
func defaultScopes(provider entity.Provider) []string {
	switch provider {
	case entity.ProviderGoogle:
		return []string{"openid", "email", "profile"}
	case entity.ProviderGitHub:
		return []string{"read:user", "user:email"}
	case entity.ProviderMicrosoft:
		return []string{"openid", "email", "profile"}
	}
	return nil
}
