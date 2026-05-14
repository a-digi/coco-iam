package oauthserverwiring

import (
	"context"
	"errors"
	"fmt"

	"github.com/a-digi/coco-iam/src/applications/keys"
	"github.com/a-digi/coco-iam/src/oauthserver"
	jwtv5 "github.com/golang-jwt/jwt/v5"
)

// KeysSigner adapts coco-iam's per-application keys.Service as
// the oauthserver.TokenSigner. Access tokens and id tokens are
// both RS256-signed with the application's currently-active
// private key; the `kid` header carries the key row id so any
// JWKS consumer can route to the right public key during a
// rotation grace window.
type KeysSigner struct {
	Service *keys.Service
}

// NewKeysSigner wraps the given service.
func NewKeysSigner(s *keys.Service) *KeysSigner { return &KeysSigner{Service: s} }

// SignAccessToken implements oauthserver.TokenSigner.
func (s *KeysSigner) SignAccessToken(_ context.Context, applicationID string, claims map[string]any) (string, error) {
	return s.sign(applicationID, claims)
}

// SignIDToken implements oauthserver.TokenSigner. Same RS256
// path as access tokens — OIDC requires id tokens be JWTs
// signed with the issuer's keys.
func (s *KeysSigner) SignIDToken(_ context.Context, applicationID string, claims map[string]any) (string, error) {
	return s.sign(applicationID, claims)
}

func (s *KeysSigner) sign(applicationID string, claims map[string]any) (string, error) {
	if s.Service == nil {
		return "", errors.New("oauthserverwiring: KeysSigner has nil keys.Service")
	}
	active, err := s.Service.ActiveRow(applicationID)
	if err != nil {
		return "", fmt.Errorf("oauthserverwiring: load active key for %s: %w", applicationID, err)
	}
	priv, err := s.Service.LoadPrivateKey(applicationID, active.ID)
	if err != nil {
		return "", fmt.Errorf("oauthserverwiring: load private key %s/%s: %w", applicationID, active.ID, err)
	}
	mc := jwtv5.MapClaims{}
	for k, v := range claims {
		mc[k] = v
	}
	tok := jwtv5.NewWithClaims(jwtv5.SigningMethodRS256, mc)
	tok.Header["kid"] = active.ID
	signed, err := tok.SignedString(priv)
	if err != nil {
		return "", fmt.Errorf("oauthserverwiring: sign RS256: %w", err)
	}
	return signed, nil
}

// KeysVerifier adapts keys.Service as the
// oauthserver.AccessTokenVerifier. Verifies RS256 JWTs whose
// `kid` header points at a still-verifiable key for the given
// application.
type KeysVerifier struct {
	Service *keys.Service
}

// NewKeysVerifier wraps the given service.
func NewKeysVerifier(s *keys.Service) *KeysVerifier { return &KeysVerifier{Service: s} }

// VerifyAccessToken implements oauthserver.AccessTokenVerifier.
// Returns the subject + the scope list parsed from the "scope"
// claim. Any structural / signature / expiry failure surfaces
// as ErrAccessTokenInvalid so the caller can collapse to a
// generic 401 without leaking the reason.
func (v *KeysVerifier) VerifyAccessToken(_ context.Context, applicationID, bearer string) (string, []string, error) {
	if v.Service == nil {
		return "", nil, errors.New("oauthserverwiring: KeysVerifier has nil keys.Service")
	}
	parsed, err := jwtv5.Parse(bearer, func(t *jwtv5.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwtv5.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, fmt.Errorf("missing kid header")
		}
		return v.Service.LoadVerifiablePublicKey(applicationID, kid)
	})
	if err != nil || parsed == nil || !parsed.Valid {
		return "", nil, oauthserver.ErrAccessTokenInvalid
	}
	claims, ok := parsed.Claims.(jwtv5.MapClaims)
	if !ok {
		return "", nil, oauthserver.ErrAccessTokenInvalid
	}
	sub, _ := claims["sub"].(string)
	if sub == "" {
		return "", nil, oauthserver.ErrAccessTokenInvalid
	}
	scopeStr, _ := claims["scope"].(string)
	scopes := splitScopes(scopeStr)
	return sub, scopes, nil
}

// splitScopes wraps strings.Fields so this file doesn't import
// strings just for the one call.
func splitScopes(s string) []string {
	if s == "" {
		return nil
	}
	out := []string{}
	cur := ""
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' || s[i] == '\t' {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
		} else {
			cur += string(s[i])
		}
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

// Compile-time guards.
var _ oauthserver.TokenSigner = (*KeysSigner)(nil)
var _ oauthserver.AccessTokenVerifier = (*KeysVerifier)(nil)
