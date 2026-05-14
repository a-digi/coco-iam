package oauth

import (
	"crypto/rsa"
	"fmt"
	"strings"
	"time"

	"github.com/a-digi/coco-iam/src/applications/keys"
	oauth_lib "github.com/a-digi/coco-oauth/oauth"
	oauth_model "github.com/a-digi/coco-oauth/oauth/model"
	jwtv5 "github.com/golang-jwt/jwt/v5"
)

const (
	DefaultExpiryMinutes = 30
	// RefreshExpiryHours is the TTL we mint into the refresh JWT on
	// application login. The renew endpoint accepts any still-valid
	// token (plus a 15-min grace past expiry), so this gives a client
	// ~24h to call `/oauth/renew` after the access token has expired.
	RefreshExpiryHours = 24
)

// RefreshScope is the only scope on a refresh JWT. It exists solely
// to prove "this holder can call /oauth/renew" and must not grant any
// application-level permission.
const RefreshScope = "token:refresh"

// LoginTokenResponse extends the standard oauth TokenResponse with a
// refresh token. We keep it local to coco-iam so we don't fork the
// vendored library. The JSON layout is identical to TokenResponse for
// everything except the extra `refresh_token` field.
type LoginTokenResponse struct {
	AccessToken  string              `json:"access_token"`
	RefreshToken string              `json:"refresh_token"`
	TokenType    string              `json:"token_type"`
	ExpiresAt    int64               `json:"expires_at"`
	User         oauth_model.UserInfo `json:"user"`
}

func ProvideTimeToLive() time.Duration {
	return DefaultExpiryMinutes * time.Minute
}

// IssueToken issues a JWT token response using a config struct
func IssueToken(cfg oauth_lib.AuthConfig, subject string, scopes []string) (oauth_model.TokenResponse, error) {
	if cfg.HS256Secret == "" {
		return oauth_model.TokenResponse{}, nil
	}
	ttl := ProvideTimeToLive()
	token, exp, err := oauth_lib.SignHS256([]byte(cfg.HS256Secret), cfg.Issuer, cfg.Audience, subject, scopes, ttl)

	if err != nil {
		return oauth_model.TokenResponse{}, err
	}

	return oauth_model.TokenResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresAt:   exp.Unix(),
		User:        oauth_model.UserInfo{ID: subject},
	}, nil
}

// IssueAppLoginTokens mints an access + refresh pair signed with the
// application's **active** RSA private key (RS256). The `kid` JWT
// header is set to the key's row id so verifiers can pick the right
// public key out of the `/.well-known/jwks.json` response — which
// carries active + still-verifiable deactivated keys during a
// rotation's 24-hour grace window.
//
// resourceIDs is an optional map of scope → list-of-ids. Downstream
// services use it to enforce id-level authorisation ("this scope
// applies only to these ids"); coco-iam's own public management API
// always re-reads fresh values from the DB rather than trusting the
// claim.
func IssueAppLoginTokens(keysSvc *keys.Service, appID string, cfg oauth_lib.AuthConfig, subject string, scopes []string, resourceIDs map[string][]string) (LoginTokenResponse, error) {
	if keysSvc == nil {
		return LoginTokenResponse{}, fmt.Errorf("keys service not available")
	}
	active, err := keysSvc.ActiveRow(appID)
	if err != nil {
		return LoginTokenResponse{}, fmt.Errorf("load active key for %s: %w", appID, err)
	}
	priv, err := keysSvc.LoadPrivateKey(appID, active.ID)
	if err != nil {
		return LoginTokenResponse{}, fmt.Errorf("load private key for %s/%s: %w", appID, active.ID, err)
	}
	access, accessExp, err := signRS256(priv, cfg.Issuer, cfg.Audience, subject, active.ID, scopes, resourceIDs, ProvideTimeToLive())
	if err != nil {
		return LoginTokenResponse{}, err
	}
	refresh, _, err := signRS256(
		priv, cfg.Issuer, cfg.Audience, subject, active.ID,
		[]string{RefreshScope}, nil, time.Duration(RefreshExpiryHours)*time.Hour,
	)
	if err != nil {
		return LoginTokenResponse{}, err
	}
	return LoginTokenResponse{
		AccessToken:  access,
		RefreshToken: refresh,
		TokenType:    "Bearer",
		ExpiresAt:    accessExp.Unix(),
		User:         oauth_model.UserInfo{ID: subject},
	}, nil
}

// signRS256 builds and signs a single RS256 JWT. Kept small and
// side-effect free so both access and refresh codepaths can reuse it.
// `resourceIDs`, when non-empty, lands as the `resource_ids` claim.
func signRS256(priv *rsa.PrivateKey, issuer, audience, subject, kid string, scopes []string, resourceIDs map[string][]string, ttl time.Duration) (string, time.Time, error) {
	exp := time.Now().Add(ttl)
	claims := jwtv5.MapClaims{
		"iss": issuer,
		"aud": audience,
		"sub": subject,
		"exp": exp.Unix(),
		"iat": time.Now().Unix(),
	}
	if len(scopes) > 0 {
		claims["scope"] = strings.Join(scopes, " ")
	}
	if len(resourceIDs) > 0 {
		claims["resource_ids"] = resourceIDs
	}
	tok := jwtv5.NewWithClaims(jwtv5.SigningMethodRS256, claims)
	if kid != "" {
		tok.Header["kid"] = kid
	}
	signed, err := tok.SignedString(priv)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign RS256: %w", err)
	}
	return signed, exp, nil
}

// IssueLoginTokens mints both the access JWT (same shape IssueToken
// returns) and a longer-lived refresh JWT. The refresh token carries
// only `token:refresh` as its scope so it can be distinguished from an
// access token if anyone inspects the claims.
func IssueLoginTokens(cfg oauth_lib.AuthConfig, subject string, scopes []string) (LoginTokenResponse, error) {
	if cfg.HS256Secret == "" {
		return LoginTokenResponse{}, nil
	}
	accessTTL := ProvideTimeToLive()
	access, accessExp, err := oauth_lib.SignHS256(
		[]byte(cfg.HS256Secret), cfg.Issuer, cfg.Audience, subject, scopes, accessTTL,
	)
	if err != nil {
		return LoginTokenResponse{}, err
	}
	refreshTTL := time.Duration(RefreshExpiryHours) * time.Hour
	refresh, _, err := oauth_lib.SignHS256(
		[]byte(cfg.HS256Secret), cfg.Issuer, cfg.Audience, subject, []string{RefreshScope}, refreshTTL,
	)
	if err != nil {
		return LoginTokenResponse{}, err
	}
	return LoginTokenResponse{
		AccessToken:  access,
		RefreshToken: refresh,
		TokenType:    "Bearer",
		ExpiresAt:    accessExp.Unix(),
		User:         oauth_model.UserInfo{ID: subject},
	}, nil
}
