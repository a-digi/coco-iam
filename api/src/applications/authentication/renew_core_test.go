package authentication

import (
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"testing"
	"time"

	"github.com/a-digi/coco-iam/src/auth/oauth"
	oauth_lib "github.com/a-digi/coco-oauth/oauth"
	jwtv5 "github.com/golang-jwt/jwt/v5"
)

// testKey generates a fresh RSA key pair per subtest. Using a per-run
// key guarantees we can't accidentally verify a token signed outside
// this process.
func testKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return key
}

// signRefresh produces an RS256-signed JWT with the given claim shape
// and `kid: test-kid` header. The caller decides whether the claims
// describe a valid refresh token, a stolen access token, an expired
// token, etc.
func signRefresh(t *testing.T, key *rsa.PrivateKey, claims jwtv5.MapClaims) string {
	t.Helper()
	tok := jwtv5.NewWithClaims(jwtv5.SigningMethodRS256, claims)
	tok.Header["kid"] = "test-kid"
	signed, err := tok.SignedString(key)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

// validClaims builds the minimum claim set renewAppToken considers a
// valid refresh token: subject + refresh scope + unexpired exp.
func validClaims(extraScope string) jwtv5.MapClaims {
	scope := oauth.RefreshScope
	if extraScope != "" {
		scope += " " + extraScope
	}
	return jwtv5.MapClaims{
		"sub":   "user-123",
		"scope": scope,
		"exp":   float64(time.Now().Add(time.Hour).Unix()),
	}
}

// cannedMint records the last call and returns a fixed token pair so
// tests can assert what the core passed through to the minter.
type cannedMint struct {
	calledWith struct {
		appID       string
		subject     string
		scopes      []string
		resourceIDs map[string][]string
	}
	returnErr error
}

func (c *cannedMint) fn() MintTokensFunc {
	return func(appID string, _ oauth_lib.AuthConfig, sub string, scopes []string, resourceIDs map[string][]string) (oauth.LoginTokenResponse, error) {
		c.calledWith.appID = appID
		c.calledWith.subject = sub
		c.calledWith.scopes = scopes
		c.calledWith.resourceIDs = resourceIDs
		if c.returnErr != nil {
			return oauth.LoginTokenResponse{}, c.returnErr
		}
		return oauth.LoginTokenResponse{
			AccessToken:  "new-access",
			RefreshToken: "new-refresh",
			TokenType:    "Bearer",
		}, nil
	}
}

// staticKey returns a loader that always resolves to the given pubkey,
// simulating a happy-path keys service.
func staticKey(pub *rsa.PublicKey) LoadPublicKeyFunc {
	return func(kid string) (*rsa.PublicKey, error) {
		return pub, nil
	}
}

// noResources is the all-empty resource_ids loader — used when the
// test doesn't exercise resource-id behaviour.
func noResources(appID, userID string) map[string][]string {
	return nil
}

// assertRenewError unwraps the returned error as *RenewError and
// asserts status + message. Any other error type or nil fails.
func assertRenewError(t *testing.T, err error, wantStatus int, wantMsgContains string) {
	t.Helper()
	if err == nil {
		t.Fatalf("want RenewError, got nil")
	}
	var rerr *RenewError
	if !errors.As(err, &rerr) {
		t.Fatalf("want *RenewError, got %T: %v", err, err)
	}
	if rerr.Status != wantStatus {
		t.Errorf("status: want %d, got %d", wantStatus, rerr.Status)
	}
	if wantMsgContains != "" && !contains(rerr.Message, wantMsgContains) {
		t.Errorf("message %q does not contain %q", rerr.Message, wantMsgContains)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// ---------- Tests ----------

func TestRenewAppToken_EmptyRefreshTokenIsBadRequest(t *testing.T) {
	_, err := renewAppToken(
		"",
		"app-id",
		oauth_lib.AuthConfig{},
		staticKey(nil),
		(&cannedMint{}).fn(),
		noResources,
		time.Now(),
	)
	assertRenewError(t, err, 400, "refresh_token is required")
}

func TestRenewAppToken_TamperedSignatureIsUnauthorized(t *testing.T) {
	key := testKey(t)
	token := signRefresh(t, key, validClaims(""))
	// Flip the last byte to invalidate the signature.
	tampered := token[:len(token)-1] + "A"
	if tampered == token {
		tampered = token[:len(token)-1] + "B"
	}

	_, err := renewAppToken(
		tampered,
		"app-id",
		oauth_lib.AuthConfig{},
		staticKey(&key.PublicKey),
		(&cannedMint{}).fn(),
		noResources,
		time.Now(),
	)
	assertRenewError(t, err, 401, "invalid refresh token")
}

func TestRenewAppToken_WrongKeyRejected(t *testing.T) {
	signKey := testKey(t)
	otherKey := testKey(t) // public key from a different keypair
	token := signRefresh(t, signKey, validClaims(""))

	_, err := renewAppToken(
		token,
		"app-id",
		oauth_lib.AuthConfig{},
		staticKey(&otherKey.PublicKey),
		(&cannedMint{}).fn(),
		noResources,
		time.Now(),
	)
	assertRenewError(t, err, 401, "")
}

func TestRenewAppToken_MissingKidHeaderRejected(t *testing.T) {
	key := testKey(t)
	tok := jwtv5.NewWithClaims(jwtv5.SigningMethodRS256, validClaims(""))
	// Intentionally no kid header.
	signed, err := tok.SignedString(key)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	_, err = renewAppToken(
		signed,
		"app-id",
		oauth_lib.AuthConfig{},
		staticKey(&key.PublicKey),
		(&cannedMint{}).fn(),
		noResources,
		time.Now(),
	)
	assertRenewError(t, err, 401, "missing kid header")
}

func TestRenewAppToken_StolenAccessTokenInRefreshSlotRejected(t *testing.T) {
	key := testKey(t)
	// Looks like a valid user token but lacks the refresh scope —
	// this is what a stolen access token would look like if someone
	// tried to use it on the renew endpoint.
	claims := jwtv5.MapClaims{
		"sub":   "user-123",
		"scope": "user:me",
		"exp":   float64(time.Now().Add(time.Hour).Unix()),
	}
	token := signRefresh(t, key, claims)

	_, err := renewAppToken(
		token,
		"app-id",
		oauth_lib.AuthConfig{},
		staticKey(&key.PublicKey),
		(&cannedMint{}).fn(),
		noResources,
		time.Now(),
	)
	assertRenewError(t, err, 401, "not a refresh token")
}

func TestRenewAppToken_MissingSubjectRejected(t *testing.T) {
	key := testKey(t)
	claims := jwtv5.MapClaims{
		"scope": oauth.RefreshScope,
		"exp":   float64(time.Now().Add(time.Hour).Unix()),
	}
	token := signRefresh(t, key, claims)

	_, err := renewAppToken(
		token,
		"app-id",
		oauth_lib.AuthConfig{},
		staticKey(&key.PublicKey),
		(&cannedMint{}).fn(),
		noResources,
		time.Now(),
	)
	assertRenewError(t, err, 401, "missing subject")
}

func TestRenewAppToken_ExpiredBeyondGraceRejected(t *testing.T) {
	key := testKey(t)
	// Expired 20 minutes ago — past the 15-min grace window.
	claims := jwtv5.MapClaims{
		"sub":   "user-123",
		"scope": oauth.RefreshScope,
		"exp":   float64(time.Now().Add(-20 * time.Minute).Unix()),
	}
	token := signRefresh(t, key, claims)

	_, err := renewAppToken(
		token,
		"app-id",
		oauth_lib.AuthConfig{},
		staticKey(&key.PublicKey),
		(&cannedMint{}).fn(),
		noResources,
		time.Now(),
	)
	assertRenewError(t, err, 401, "expired")
}

func TestRenewAppToken_ExpiredWithinGraceIsAccepted(t *testing.T) {
	key := testKey(t)
	// Expired 5 minutes ago — inside the 15-min grace window.
	claims := jwtv5.MapClaims{
		"sub":   "user-123",
		"scope": oauth.RefreshScope,
		"exp":   float64(time.Now().Add(-5 * time.Minute).Unix()),
	}
	token := signRefresh(t, key, claims)
	mint := &cannedMint{}

	got, err := renewAppToken(
		token,
		"app-id",
		oauth_lib.AuthConfig{},
		staticKey(&key.PublicKey),
		mint.fn(),
		noResources,
		time.Now(),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.AccessToken != "new-access" {
		t.Errorf("want minted access token, got %q", got.AccessToken)
	}
}

func TestRenewAppToken_HappyPathMintsWithStrippedRefreshScope(t *testing.T) {
	key := testKey(t)
	// Original scopes include a real user scope alongside the
	// refresh marker. The new access token should carry only the
	// non-refresh scopes.
	token := signRefresh(t, key, validClaims("user:me admin:read"))
	mint := &cannedMint{}

	got, err := renewAppToken(
		token,
		"app-abc",
		oauth_lib.AuthConfig{},
		staticKey(&key.PublicKey),
		mint.fn(),
		func(appID, userID string) map[string][]string {
			return map[string][]string{"admin:read": {"resource-1"}}
		},
		time.Now(),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.AccessToken == "" {
		t.Fatal("expected minted access token")
	}
	if mint.calledWith.subject != "user-123" {
		t.Errorf("subject: want user-123, got %q", mint.calledWith.subject)
	}
	if mint.calledWith.appID != "app-abc" {
		t.Errorf("appID: want app-abc, got %q", mint.calledWith.appID)
	}
	// Refresh scope must be stripped; original user scopes preserved.
	if containsString(mint.calledWith.scopes, oauth.RefreshScope) {
		t.Errorf("refresh scope should be stripped, got %v", mint.calledWith.scopes)
	}
	if !containsString(mint.calledWith.scopes, "user:me") {
		t.Errorf("user:me should survive, got %v", mint.calledWith.scopes)
	}
	if !containsString(mint.calledWith.scopes, "admin:read") {
		t.Errorf("admin:read should survive, got %v", mint.calledWith.scopes)
	}
	// Resource IDs map should flow through unchanged.
	if got, want := mint.calledWith.resourceIDs["admin:read"], []string{"resource-1"}; !stringSliceEqual(got, want) {
		t.Errorf("resourceIDs: want %v, got %v", want, got)
	}
}

func TestRenewAppToken_RefreshOnlyScopeFallsBackToUserMe(t *testing.T) {
	// When the original token only had `token:refresh`, the new access
	// token falls back to `user:me` so the user isn't left scope-less.
	key := testKey(t)
	token := signRefresh(t, key, validClaims(""))
	mint := &cannedMint{}

	if _, err := renewAppToken(
		token,
		"app-id",
		oauth_lib.AuthConfig{},
		staticKey(&key.PublicKey),
		mint.fn(),
		noResources,
		time.Now(),
	); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mint.calledWith.scopes) != 1 || mint.calledWith.scopes[0] != "user:me" {
		t.Errorf("want [user:me] fallback, got %v", mint.calledWith.scopes)
	}
}

func TestRenewAppToken_MintErrorIsPassedThrough(t *testing.T) {
	key := testKey(t)
	token := signRefresh(t, key, validClaims(""))
	mint := &cannedMint{returnErr: errors.New("signing broken")}

	_, err := renewAppToken(
		token,
		"app-id",
		oauth_lib.AuthConfig{},
		staticKey(&key.PublicKey),
		mint.fn(),
		noResources,
		time.Now(),
	)
	assertRenewError(t, err, 500, "signing broken")
}

// ---------- tiny helpers ----------

func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
