package userprofile

import (
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"testing"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
)

// testKey generates a fresh RSA key per subtest. Per-run keys mean
// no test can accidentally verify a token signed outside this
// process — keeps these pure and hermetic.
func testKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	return key
}

// signJWT builds an RS256 token with a `kid=test-kid` header and
// the supplied claims. Callers deliberately break individual
// claims / headers to test the rejection paths.
func signJWT(t *testing.T, key *rsa.PrivateKey, claims jwtv5.MapClaims) string {
	t.Helper()
	tok := jwtv5.NewWithClaims(jwtv5.SigningMethodRS256, claims)
	tok.Header["kid"] = "test-kid"
	signed, err := tok.SignedString(key)
	if err != nil {
		t.Fatalf("sign jwt: %v", err)
	}
	return signed
}

// validClaims returns a claim set that clears every
// authenticateUser check: a subject, a non-expired exp, and no
// refresh-scope marker.
func validClaims(sub string) jwtv5.MapClaims {
	return jwtv5.MapClaims{
		"sub":   sub,
		"scope": "user:me",
		"exp":   float64(time.Now().Add(time.Hour).Unix()),
	}
}

// buildHeader prefixes a raw token with `Bearer `.
func buildHeader(raw string) string { return "Bearer " + raw }

// assertAuthError fails the test unless `err` is a non-nil
// AuthError with the expected status + reason.
func assertAuthError(t *testing.T, err *AuthError, wantStatus int, wantReason string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected AuthError, got nil")
	}
	if err.Status != wantStatus {
		t.Errorf("status: want %d, got %d", wantStatus, err.Status)
	}
	if err.Reason != wantReason {
		t.Errorf("reason: want %q, got %q", wantReason, err.Reason)
	}
}

// mapLookup wraps a map so tests can hand a fake UserOrgLookupFunc.
// The missingErr is what's returned when the user id isn't in the
// map — pass ErrUserNotFound to exercise the "unknown user" path,
// or a generic error to exercise the 500 path.
func mapLookup(m map[string]string, missingErr error) UserOrgLookupFunc {
	return func(userID string) (string, error) {
		if orgID, ok := m[userID]; ok {
			return orgID, nil
		}
		return "", missingErr
	}
}

// staticKey returns a LoadPublicKeyFunc that always returns the
// supplied public key. Simulates the production KeyLoader
// succeeding for any kid.
func staticKey(pub *rsa.PublicKey) LoadPublicKeyFunc {
	return func(kid string) (*rsa.PublicKey, error) {
		return pub, nil
	}
}

// ------------------------------------------------------------------

func TestAuthenticateUser_MissingHeader(t *testing.T) {
	_, err := authenticateUser("", "org-1", staticKey(nil),
		mapLookup(nil, ErrUserNotFound), time.Now())
	assertAuthError(t, err, 401, ReasonMissingHeader)
}

func TestAuthenticateUser_BadScheme(t *testing.T) {
	// Basic auth on this endpoint is a confused caller — reject
	// before looking anything up.
	_, err := authenticateUser("Basic abc==", "org-1", staticKey(nil),
		mapLookup(nil, ErrUserNotFound), time.Now())
	assertAuthError(t, err, 401, ReasonBadScheme)
}

func TestAuthenticateUser_EmptyToken(t *testing.T) {
	_, err := authenticateUser("Bearer ", "org-1", staticKey(nil),
		mapLookup(nil, ErrUserNotFound), time.Now())
	assertAuthError(t, err, 401, ReasonEmptyToken)
}

func TestAuthenticateUser_GarbageToken(t *testing.T) {
	_, err := authenticateUser("Bearer not-a-jwt", "org-1", staticKey(nil),
		mapLookup(nil, ErrUserNotFound), time.Now())
	assertAuthError(t, err, 401, ReasonInvalidSignature)
}

func TestAuthenticateUser_SignedByDifferentKey(t *testing.T) {
	// Primary rejection path for cross-app tokens. A token
	// signed by app B's key is presented to the key loader bound
	// to app A; the signature check fails.
	signKey := testKey(t)
	otherKey := testKey(t)
	token := signJWT(t, signKey, validClaims("user-1"))

	_, err := authenticateUser(buildHeader(token), "org-1",
		staticKey(&otherKey.PublicKey),
		mapLookup(map[string]string{"user-1": "org-1"}, ErrUserNotFound),
		time.Now())
	assertAuthError(t, err, 401, ReasonInvalidSignature)
}

func TestAuthenticateUser_MissingKidHeader(t *testing.T) {
	key := testKey(t)
	tok := jwtv5.NewWithClaims(jwtv5.SigningMethodRS256, validClaims("user-1"))
	// Intentionally no kid header.
	signed, err := tok.SignedString(key)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	_, authErr := authenticateUser(buildHeader(signed), "org-1",
		staticKey(&key.PublicKey),
		mapLookup(map[string]string{"user-1": "org-1"}, ErrUserNotFound),
		time.Now())
	assertAuthError(t, authErr, 401, ReasonInvalidSignature)
}

func TestAuthenticateUser_NonRS256Rejected(t *testing.T) {
	// An HS256 token must not be accepted — the parser pins
	// `WithValidMethods(["RS256"])`.
	tok := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, validClaims("user-1"))
	tok.Header["kid"] = "test-kid"
	signed, err := tok.SignedString([]byte("shared-secret"))
	if err != nil {
		t.Fatalf("sign hs256: %v", err)
	}
	key := testKey(t)
	_, authErr := authenticateUser(buildHeader(signed), "org-1",
		staticKey(&key.PublicKey),
		mapLookup(map[string]string{"user-1": "org-1"}, ErrUserNotFound),
		time.Now())
	assertAuthError(t, authErr, 401, ReasonInvalidSignature)
}

func TestAuthenticateUser_ExpiredToken(t *testing.T) {
	// Injected `now` puts us past the token's exp — the jwt
	// library rejects. Since we don't grant a grace window for
	// reads (unlike renew), expiry is a hard 401.
	key := testKey(t)
	claims := validClaims("user-1")
	claims["exp"] = float64(time.Now().Add(-time.Hour).Unix())
	token := signJWT(t, key, claims)

	_, err := authenticateUser(buildHeader(token), "org-1",
		staticKey(&key.PublicKey),
		mapLookup(map[string]string{"user-1": "org-1"}, ErrUserNotFound),
		time.Now())
	assertAuthError(t, err, 401, ReasonInvalidSignature)
}

func TestAuthenticateUser_RefreshTokenRejected(t *testing.T) {
	// A lost refresh token must not be usable on the read surface.
	// Hygiene, not a hard-security win, but pinned.
	key := testKey(t)
	claims := validClaims("user-1")
	claims["scope"] = "token:refresh user:me"
	token := signJWT(t, key, claims)

	_, err := authenticateUser(buildHeader(token), "org-1",
		staticKey(&key.PublicKey),
		mapLookup(map[string]string{"user-1": "org-1"}, ErrUserNotFound),
		time.Now())
	assertAuthError(t, err, 401, ReasonIsRefreshToken)
}

func TestAuthenticateUser_MissingSubject(t *testing.T) {
	key := testKey(t)
	claims := jwtv5.MapClaims{
		"scope": "user:me",
		"exp":   float64(time.Now().Add(time.Hour).Unix()),
	}
	token := signJWT(t, key, claims)

	_, err := authenticateUser(buildHeader(token), "org-1",
		staticKey(&key.PublicKey),
		mapLookup(nil, ErrUserNotFound),
		time.Now())
	assertAuthError(t, err, 401, ReasonMissingSubject)
}

func TestAuthenticateUser_UnknownUser(t *testing.T) {
	// Token is well-formed but the subject doesn't map to any
	// user in the main DB. Must 401 (not 500) so a stale token
	// for a deleted user gets handled cleanly.
	key := testKey(t)
	token := signJWT(t, key, validClaims("user-ghost"))

	_, err := authenticateUser(buildHeader(token), "org-1",
		staticKey(&key.PublicKey),
		mapLookup(nil, ErrUserNotFound),
		time.Now())
	assertAuthError(t, err, 401, ReasonUnknownUser)
}

func TestAuthenticateUser_CrossOrgRejected(t *testing.T) {
	// User is real but belongs to org-2. Caller is hitting a
	// URL whose org resolves to org-1. Reject — belt-and-braces
	// on top of the cross-app rejection that the signing-key
	// resolver already enforces.
	key := testKey(t)
	token := signJWT(t, key, validClaims("user-1"))

	_, err := authenticateUser(buildHeader(token), "org-1",
		staticKey(&key.PublicKey),
		mapLookup(map[string]string{"user-1": "org-OTHER"}, ErrUserNotFound),
		time.Now())
	assertAuthError(t, err, 401, ReasonCrossOrg)
}

func TestAuthenticateUser_InternalErrorOnLookupFailure(t *testing.T) {
	// A generic (non-NotFound) lookup error is the only 500 path
	// in the auth flow — every other failure is a 401. Keeping
	// them distinct means ops can distinguish "bad client" from
	// "DB outage" in logs.
	key := testKey(t)
	token := signJWT(t, key, validClaims("user-1"))

	_, err := authenticateUser(buildHeader(token), "org-1",
		staticKey(&key.PublicKey),
		mapLookup(nil, errors.New("db down")),
		time.Now())
	assertAuthError(t, err, 500, ReasonInternal)
}

func TestAuthenticateUser_HappyPath(t *testing.T) {
	key := testKey(t)
	token := signJWT(t, key, validClaims("user-1"))

	userID, err := authenticateUser(buildHeader(token), "org-1",
		staticKey(&key.PublicKey),
		mapLookup(map[string]string{"user-1": "org-1"}, ErrUserNotFound),
		time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %+v", err)
	}
	if userID != "user-1" {
		t.Errorf("userID: want user-1, got %q", userID)
	}
}
