package authentication

import (
	"crypto/rsa"
	"errors"
	"time"

	"github.com/a-digi/coco-iam/src/auth/oauth"
	oauth_lib "github.com/a-digi/coco-oauth/oauth"
	jwtv5 "github.com/golang-jwt/jwt/v5"
)

// LoadPublicKeyFunc returns the RSA public key identified by `kid` for
// this renew's application. The caller closes the appID over the
// function so the core doesn't need the keys service concretely; this
// makes the core trivially fakeable from a test.
type LoadPublicKeyFunc func(kid string) (*rsa.PublicKey, error)

// MintTokensFunc mints a fresh access + refresh pair. In production
// this wraps `oauth.IssueAppLoginTokens`; tests inject a fake that
// returns a canned LoginTokenResponse or a prescribed error.
type MintTokensFunc func(
	appID string,
	cfg oauth_lib.AuthConfig,
	subject string,
	scopes []string,
	resourceIDs map[string][]string,
) (oauth.LoginTokenResponse, error)

// LoadResourceIDsFunc returns the scope→resource_ids map re-read from
// storage on every renew. Returning nil is a valid result — it just
// means none of the caller's scopes are resource-constrained.
type LoadResourceIDsFunc func(appID, userID string) map[string][]string

// RenewError classifies why a renew failed so the HTTP handler can
// return the right status code + obfuscated user message. The core
// never writes the HTTP response itself — that's the glue's job.
type RenewError struct {
	Status  int    // http.StatusBadRequest / StatusUnauthorized / StatusInternalServerError
	Message string // user-facing message (already obfuscated where needed)
}

func (e *RenewError) Error() string { return e.Message }

// renewAppToken is the pure decision layer of the renew endpoint: it
// takes a refresh-token string plus enough effectful callbacks to
// verify it and mint fresh tokens, and returns either a fresh token
// pair or a classified RenewError.
//
// It does no I/O of its own — the three callbacks are the only
// non-pure surface. That keeps the unit-test story trivial:
//
//   - loadPublicKey: return a test RSA key or an error
//   - mint: return canned LoginTokenResponse or an error
//   - loadResourceIDs: return any map
//
// The handler wraps this function with HTTP parsing + writing only.
func renewAppToken(
	refreshToken string,
	appID string,
	cfg oauth_lib.AuthConfig,
	loadPublicKey LoadPublicKeyFunc,
	mint MintTokensFunc,
	loadResourceIDs LoadResourceIDsFunc,
	now time.Time,
) (oauth.LoginTokenResponse, error) {
	if refreshToken == "" {
		return oauth.LoginTokenResponse{}, &RenewError{Status: 400, Message: "refresh_token is required"}
	}

	parser := jwtv5.NewParser(
		jwtv5.WithValidMethods([]string{"RS256"}),
		jwtv5.WithoutClaimsValidation(),
	)
	// Key is resolved from the token's own `kid` header — different
	// tokens can be signed by different keys during a rotation's
	// grace window. Non-string / missing kid is a signal the token
	// is malformed, not a transient error.
	tok, err := parser.Parse(refreshToken, func(t *jwtv5.Token) (interface{}, error) {
		kidRaw, ok := t.Header["kid"]
		if !ok {
			return nil, errors.New("refresh token is missing kid header")
		}
		kid, ok := kidRaw.(string)
		if !ok || kid == "" {
			return nil, errors.New("refresh token kid header is not a string")
		}
		return loadPublicKey(kid)
	})
	if err != nil && !errors.Is(err, jwtv5.ErrTokenExpired) {
		return oauth.LoginTokenResponse{}, &RenewError{Status: 401, Message: "invalid refresh token: " + err.Error()}
	}
	if tok == nil {
		return oauth.LoginTokenResponse{}, &RenewError{Status: 401, Message: "invalid refresh token"}
	}

	claims, ok := tok.Claims.(jwtv5.MapClaims)
	if !ok {
		return oauth.LoginTokenResponse{}, &RenewError{Status: 401, Message: "invalid refresh token claims"}
	}

	sub, _ := claims["sub"].(string)
	if sub == "" {
		return oauth.LoginTokenResponse{}, &RenewError{Status: 401, Message: "refresh token missing subject"}
	}

	// Confirm this is a refresh token, not a stolen access token
	// being played back into the refresh slot.
	scope, _ := claims["scope"].(string)
	if !containsScope(scope, oauth.RefreshScope) {
		return oauth.LoginTokenResponse{}, &RenewError{Status: 401, Message: "not a refresh token"}
	}

	// 15-min grace past `exp` so a target that saw a 401 right at
	// expiry still gets a new pair without forcing re-login.
	if expF, ok := claims["exp"].(float64); ok {
		expiry := time.Unix(int64(expF), 0)
		if expiry.Add(renewGrace).Before(now) {
			return oauth.LoginTokenResponse{}, &RenewError{Status: 401, Message: "refresh token expired"}
		}
	}

	// New access token inherits the original scopes minus the
	// refresh-only marker. If the original only carried
	// `token:refresh`, fall back to `user:me` — the same default the
	// login path would have emitted for a non-pwd-reset user.
	newScopes := stripRefreshScope(splitScope(scope))
	if len(newScopes) == 0 {
		newScopes = []string{"user:me"}
	}

	resourceIDs := loadResourceIDs(appID, sub)
	fresh, err := mint(appID, cfg, sub, newScopes, resourceIDs)
	if err != nil {
		return oauth.LoginTokenResponse{}, &RenewError{Status: 500, Message: "token signing failed: " + err.Error()}
	}
	return fresh, nil
}
