package userprofile

import (
	"crypto/rsa"
	"errors"
	"strings"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
)

// AuthError classifies why auth on the /profile/me endpoint failed
// so the handler can respond with the right HTTP status. The
// handler collapses every AuthError to a generic 401 body on the
// wire — callers of this function never see distinguishing text —
// but keeping the reasons structured in the error makes tests
// precise and makes future logging / metrics straightforward.
type AuthError struct {
	Status int    // http.StatusUnauthorized / StatusInternalServerError
	Reason string // stable machine-readable marker for tests + logs
}

func (e *AuthError) Error() string { return e.Reason }

// Enumerated AuthError reasons. The wire response is always
// "unauthorized"; these are for tests + observability.
const (
	ReasonMissingHeader      = "missing_authorization_header"
	ReasonBadScheme          = "bad_auth_scheme"
	ReasonEmptyToken         = "empty_token"
	ReasonInvalidSignature   = "invalid_signature"
	ReasonMalformedClaims    = "malformed_claims"
	ReasonIsRefreshToken     = "refresh_token_not_allowed_here"
	ReasonMissingSubject     = "missing_subject"
	ReasonUnknownUser        = "unknown_user"
	ReasonCrossOrg           = "cross_org_user"
	ReasonInternal           = "internal"
)

// LoadPublicKeyFunc returns the RSA public key identified by `kid`
// for the application the URL resolved to. In production this
// closes over `keys.Service.LoadVerifiablePublicKey(appID, kid)`;
// tests inject a fake that returns a canned key or error.
type LoadPublicKeyFunc func(kid string) (*rsa.PublicKey, error)

// UserOrgLookupFunc returns the organisation id for the given user
// id (or an error when the user is missing). In production this
// queries the main DB; tests supply a map-backed fake.
type UserOrgLookupFunc func(userID string) (orgID string, err error)

// authenticateUser is the full auth + authorisation decision layer
// for `GET /a/.../profile/me`. It's pure-ish: the three effectful
// dependencies are injected as callbacks. The handler calls this
// once and then either responds with the error's status or
// proceeds to fetch the profile data.
//
// The flow mirrors the plan's invariants:
//   1. Bearer-scheme parse.
//   2. JWT signature verification against the app's RS256 key
//      (cross-app rejection is implicit — a token signed by app B
//      fails against app A's key).
//   3. Reject tokens carrying `token:refresh` scope.
//   4. Extract `sub`; reject on empty.
//   5. Confirm the user exists AND sits in `expectedOrgID`.
//
// `now` is injected so tests don't have to sleep to exercise
// expiry behaviour.
//
// Exported as AuthenticateUser so sibling packages (e.g. the
// OAuth-server connected-apps surface) can reuse the same
// auth check without copying the security-critical parsing.
func AuthenticateUser(
	header string,
	expectedOrgID string,
	loadPublicKey LoadPublicKeyFunc,
	userOrgLookup UserOrgLookupFunc,
	now time.Time,
) (userID string, authErr *AuthError) {
	return authenticateUser(header, expectedOrgID, loadPublicKey, userOrgLookup, now)
}

func authenticateUser(
	header string,
	expectedOrgID string,
	loadPublicKey LoadPublicKeyFunc,
	userOrgLookup UserOrgLookupFunc,
	now time.Time,
) (userID string, authErr *AuthError) {
	rawToken, parseErr := parseBearerToken(header)
	if parseErr != nil {
		return "", parseErr
	}

	parser := jwtv5.NewParser(
		jwtv5.WithValidMethods([]string{"RS256"}),
		jwtv5.WithTimeFunc(func() time.Time { return now }),
	)
	tok, err := parser.Parse(rawToken, func(t *jwtv5.Token) (interface{}, error) {
		kidRaw, ok := t.Header["kid"]
		if !ok {
			return nil, errors.New("missing kid")
		}
		kid, ok := kidRaw.(string)
		if !ok || kid == "" {
			return nil, errors.New("kid is not a string")
		}
		return loadPublicKey(kid)
	})
	if err != nil || tok == nil || !tok.Valid {
		return "", &AuthError{Status: 401, Reason: ReasonInvalidSignature}
	}
	claims, ok := tok.Claims.(jwtv5.MapClaims)
	if !ok {
		return "", &AuthError{Status: 401, Reason: ReasonMalformedClaims}
	}

	// Refresh tokens carry `token:refresh` in the scope claim.
	// Accepting them here would let a lost refresh token read
	// profile data — meaningless but reject for hygiene.
	scope, _ := claims["scope"].(string)
	if hasScope(scope, "token:refresh") {
		return "", &AuthError{Status: 401, Reason: ReasonIsRefreshToken}
	}

	sub, _ := claims["sub"].(string)
	if sub == "" {
		return "", &AuthError{Status: 401, Reason: ReasonMissingSubject}
	}

	userOrgID, err := userOrgLookup(sub)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return "", &AuthError{Status: 401, Reason: ReasonUnknownUser}
		}
		return "", &AuthError{Status: 500, Reason: ReasonInternal}
	}
	if userOrgID != expectedOrgID {
		return "", &AuthError{Status: 401, Reason: ReasonCrossOrg}
	}
	return sub, nil
}

// hasScope returns true when the space-delimited `scope` claim
// contains the target value. Used to reject refresh tokens on
// the read surface.
func hasScope(scope, want string) bool {
	for _, s := range strings.Fields(scope) {
		if s == want {
			return true
		}
	}
	return false
}

// parseBearerToken pulls the raw JWT out of an
// `Authorization: Bearer …` header. Returns an AuthError (not a
// plain error) so the caller's branching stays flat.
func parseBearerToken(header string) (string, *AuthError) {
	if header == "" {
		return "", &AuthError{Status: 401, Reason: ReasonMissingHeader}
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", &AuthError{Status: 401, Reason: ReasonBadScheme}
	}
	raw := strings.TrimSpace(header[len(prefix):])
	if raw == "" {
		return "", &AuthError{Status: 401, Reason: ReasonEmptyToken}
	}
	return raw, nil
}
