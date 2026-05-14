package oauthserver_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/a-digi/coco-iam/src/oauthserver"
	"github.com/a-digi/coco-iam/src/oauthserver/entity"
	"github.com/a-digi/coco-iam/src/oauthserver/pkce"
	"github.com/a-digi/coco-iam/src/oauthserver/sqlstore"

	_ "github.com/mattn/go-sqlite3"
)

// End-to-end black-box test of the OAuth server library.
// Imports only the public API (oauthserver_test package) and
// drives the full authorize → token → userinfo → refresh →
// revoke sequence against the real SQL stores backed by an
// in-memory SQLite. Token signing + verification are stubbed
// because the production signer depends on per-app RSA keys.
//
// Pin: this test is the single biggest signal that the public
// surface of the library composes correctly. If a future
// refactor breaks any internal contract, this test catches it
// before any wiring layer ever sees the change.

// ---------- DI wiring against in-memory SQLite ---------------------

func setupE2E(t *testing.T) (
	*oauthserver.AuthorizeHandler,
	*oauthserver.TokenHandler,
	*oauthserver.UserinfoHandler,
	*oauthserver.RevokeHandler,
	*oauthserver.IntrospectHandler,
	*oauthserver.DiscoveryHandler,
	*sqlstore.ClientRepo,
) {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(`
		CREATE TABLE application_oauth_clients (
		    id                   TEXT PRIMARY KEY,
		    application_id       TEXT NOT NULL,
		    client_id            TEXT NOT NULL,
		    client_secret_hash   TEXT,
		    client_type          TEXT NOT NULL DEFAULT 'confidential',
		    display_name         TEXT NOT NULL DEFAULT '',
		    redirect_uris        TEXT NOT NULL DEFAULT '[]',
		    allowed_scopes       TEXT NOT NULL DEFAULT '[]',
		    require_consent      INTEGER NOT NULL DEFAULT 1,
		    access_token_ttl     INTEGER NOT NULL DEFAULT 3600,
		    refresh_token_ttl    INTEGER NOT NULL DEFAULT 1209600,
		    is_active            INTEGER NOT NULL DEFAULT 1,
		    created_at           DATETIME DEFAULT CURRENT_TIMESTAMP,
		    updated_at           DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE UNIQUE INDEX clients_app_clientid ON application_oauth_clients (application_id, client_id);

		CREATE TABLE oauth_authorization_codes (
		    code                   TEXT PRIMARY KEY,
		    client_row_id          TEXT NOT NULL,
		    application_id         TEXT NOT NULL,
		    user_id                TEXT NOT NULL,
		    redirect_uri           TEXT NOT NULL,
		    scopes                 TEXT NOT NULL DEFAULT '[]',
		    code_challenge         TEXT NOT NULL DEFAULT '',
		    code_challenge_method  TEXT NOT NULL DEFAULT 'S256',
		    nonce                  TEXT NOT NULL DEFAULT '',
		    created_at             DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE oauth_refresh_tokens (
		    id              TEXT PRIMARY KEY,
		    token_hash      TEXT NOT NULL UNIQUE,
		    client_row_id   TEXT NOT NULL,
		    application_id  TEXT NOT NULL,
		    user_id         TEXT NOT NULL,
		    scopes          TEXT NOT NULL DEFAULT '[]',
		    issued_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		    expires_at      DATETIME NOT NULL,
		    revoked_at      DATETIME,
		    replaced_by_id  TEXT
		);
		CREATE TABLE oauth_user_consents (
		    id              TEXT PRIMARY KEY,
		    user_id         TEXT NOT NULL,
		    client_row_id   TEXT NOT NULL,
		    granted_scopes  TEXT NOT NULL DEFAULT '[]',
		    granted_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		    revoked_at      DATETIME
		);
		CREATE UNIQUE INDEX consents_user_client ON oauth_user_consents (user_id, client_row_id);
	`); err != nil {
		t.Fatalf("schema: %v", err)
	}

	clients := sqlstore.NewClientRepo(db, plainHasher{})
	codes := sqlstore.NewCodeRepo(db)
	refresh := sqlstore.NewRefreshRepo(db)
	consents := sqlstore.NewConsentRepo(func(_ string) (*sql.DB, error) { return db, nil })

	signer := &stubSigner{}
	verifier := &stubVerifier{store: signer}
	auth := &stubAuth{user: "user-1", org: "org-1"}
	claimsReader := &stubClaims{claims: map[string]any{
		"email":          "alice@example.com",
		"email_verified": true,
		"name":           "Alice",
		"given_name":     "Alice",
		"family_name":    "Liddell",
	}}

	appResolver := func(_ *http.Request) (string, string, error) {
		return "app-1", "org-1", nil
	}
	issuerFn := func(_ *http.Request, _ string) string {
		return "https://iam.example/a/acme/prod/web"
	}

	authorize := &oauthserver.AuthorizeHandler{
		ApplicationIDFromRequest: appResolver,
		Clients: clients, Codes: codes, Consents: consents, Auth: auth,
		LoginRedirectURL: func(_ *http.Request, returnTo string) string {
			return "https://login.example/sign-in?return_to=" + url.QueryEscape(returnTo)
		},
	}
	token := &oauthserver.TokenHandler{
		ApplicationIDFromRequest: appResolver,
		IssuerFromRequest:        issuerFn,
		Clients:                  clients,
		Codes:                    codes,
		Refresh:                  refresh,
		Claims:                   claimsReader,
		Signer:                   signer,
	}
	userinfo := &oauthserver.UserinfoHandler{
		ApplicationIDFromRequest: appResolver,
		Verifier:                 verifier,
		Claims:                   claimsReader,
	}
	revoke := &oauthserver.RevokeHandler{
		ApplicationIDFromRequest: appResolver,
		Clients:                  clients,
		Refresh:                  refresh,
	}
	introspect := &oauthserver.IntrospectHandler{
		ApplicationIDFromRequest: appResolver,
		Clients:                  clients,
		Refresh:                  refresh,
		Verifier:                 verifier,
	}
	discovery := &oauthserver.DiscoveryHandler{
		IssuerFromRequest:   func(_ *http.Request) string { return "https://iam.example" },
		BasePathFromRequest: func(_ *http.Request) string { return "/a/acme/prod/web" },
		ScopesSupported:     []string{"openid", "profile", "email", "offline_access"},
	}
	return authorize, token, userinfo, revoke, introspect, discovery, clients
}

// ---------- stubs --------------------------------------------------

// plainHasher round-trips plaintext for fast tests.
type plainHasher struct{}

func (plainHasher) Hash(plain string) (string, error) { return "plain:" + plain, nil }
func (plainHasher) Verify(hashed, plain string) error {
	if hashed == "plain:"+plain {
		return nil
	}
	return errPlainMismatch
}

var errPlainMismatch = stubError("plain mismatch")

type stubError string

func (e stubError) Error() string { return string(e) }

// stubSigner mints tokens of the form "issuer|sub|scope|jti" so
// the verifier can decode them back without a real JWT lib.
type stubSigner struct {
	issued []map[string]any
}

func (s *stubSigner) SignAccessToken(_ context.Context, _ string, claims map[string]any) (string, error) {
	s.issued = append(s.issued, claims)
	return encodeStubToken(claims), nil
}
func (s *stubSigner) SignIDToken(_ context.Context, _ string, claims map[string]any) (string, error) {
	return encodeStubToken(claims), nil
}

type stubVerifier struct {
	store *stubSigner
}

func (v *stubVerifier) VerifyAccessToken(_ context.Context, _, token string) (string, []string, error) {
	c, ok := decodeStubToken(token)
	if !ok {
		return "", nil, oauthserver.ErrAccessTokenInvalid
	}
	sub, _ := c["sub"].(string)
	scopeStr, _ := c["scope"].(string)
	if sub == "" {
		return "", nil, oauthserver.ErrAccessTokenInvalid
	}
	return sub, strings.Fields(scopeStr), nil
}

func encodeStubToken(claims map[string]any) string {
	b, _ := json.Marshal(claims)
	return "stub." + string(b)
}
func decodeStubToken(s string) (map[string]any, bool) {
	if !strings.HasPrefix(s, "stub.") {
		return nil, false
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(s[len("stub."):]), &out); err != nil {
		return nil, false
	}
	return out, true
}

type stubAuth struct{ user, org string }

func (s *stubAuth) CurrentUser(_ context.Context, _ oauthserver.RequestInfo) (string, string, error) {
	return s.user, s.org, nil
}

type stubClaims struct{ claims map[string]any }

func (s *stubClaims) LoadClaims(_ context.Context, _, _ string, _ []string) (map[string]any, error) {
	return s.claims, nil
}

// ---------- the test ----------------------------------------------

func TestE2E_FullAuthorizationCodeFlow(t *testing.T) {
	authorize, token, userinfo, revoke, _, discovery, clients := setupE2E(t)

	// Register a client.
	if _, err := clients.Insert(context.Background(), "row-1", sqlstore.InsertInput{
		ApplicationID:   "app-1",
		ClientID:        "reporter",
		ClientSecret:    "top-secret",
		Type:            entity.ClientTypeConfidential,
		DisplayName:     "Reporter",
		RedirectURIs:    []string{"https://reporter.example/cb"},
		AllowedScopes:   []string{"openid", "profile", "email", "offline_access"},
		RequireConsent:  true,
		AccessTokenTTL:  3600,
		RefreshTokenTTL: 1209600,
	}); err != nil {
		t.Fatalf("client register: %v", err)
	}

	// Discovery sanity check.
	rec := httptest.NewRecorder()
	discovery.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/.well-known/openid-configuration", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("discovery: %d", rec.Code)
	}

	// Step 1: authorize. User is logged in (via stub auth).
	// First call lands on the consent screen (no cached consent).
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	challenge := pkce.DeriveChallenge(verifier)
	authzURL := "/oauth/authorize?" + url.Values{
		"client_id":             {"reporter"},
		"redirect_uri":          {"https://reporter.example/cb"},
		"response_type":         {"code"},
		"scope":                 {"openid profile email offline_access"},
		"state":                 {"state-xyz"},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"nonce":                 {"nonce-xyz"},
	}.Encode()

	rec = httptest.NewRecorder()
	authorize.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, authzURL, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("authorize step 1 (consent screen): want 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	// Step 2: user approves → POST same URL with approve=yes.
	postURL := authzURL + "&approve=yes"
	rec = httptest.NewRecorder()
	authorize.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, postURL, nil))
	if rec.Code != http.StatusFound {
		t.Fatalf("authorize step 2 (approve): want 302, got %d body=%s", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	parsedLoc, _ := url.Parse(loc)
	code := parsedLoc.Query().Get("code")
	if code == "" {
		t.Fatalf("no code in redirect: %s", loc)
	}
	if parsedLoc.Query().Get("state") != "state-xyz" {
		t.Errorf("state echo: %s", loc)
	}

	// Step 3: token endpoint — exchange code.
	tokenForm := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {"reporter"},
		"client_secret": {"top-secret"},
		"code":          {code},
		"code_verifier": {verifier},
		"redirect_uri":  {"https://reporter.example/cb"},
	}.Encode()
	r := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(tokenForm))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	token.ServeHTTP(rec, r)
	if rec.Code != http.StatusOK {
		t.Fatalf("token: want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var tokens oauthserver.TokenResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &tokens); err != nil {
		t.Fatalf("token decode: %v", err)
	}
	if tokens.AccessToken == "" || tokens.IDToken == "" || tokens.RefreshToken == "" {
		t.Fatalf("token response missing values: %+v", tokens)
	}

	// Step 4: userinfo with the access token.
	r = httptest.NewRequest(http.MethodGet, "/oauth/userinfo", nil)
	r.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	rec = httptest.NewRecorder()
	userinfo.ServeHTTP(rec, r)
	if rec.Code != http.StatusOK {
		t.Fatalf("userinfo: want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var ui map[string]any
	json.Unmarshal(rec.Body.Bytes(), &ui)
	if ui["sub"] != "user-1" {
		t.Errorf("userinfo sub: %v", ui["sub"])
	}
	if ui["email"] != "alice@example.com" {
		t.Errorf("userinfo email missing: %v", ui)
	}

	// Step 5: refresh — rotate the refresh token.
	refreshForm := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {"reporter"},
		"client_secret": {"top-secret"},
		"refresh_token": {tokens.RefreshToken},
	}.Encode()
	r = httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(refreshForm))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	// Fresh handler instance per request — see TokenHandler docs.
	tokenHandler2 := *token
	(&tokenHandler2).ServeHTTP(rec, r)
	if rec.Code != http.StatusOK {
		t.Fatalf("refresh: want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var rotated oauthserver.TokenResponse
	json.Unmarshal(rec.Body.Bytes(), &rotated)
	if rotated.RefreshToken == "" || rotated.RefreshToken == tokens.RefreshToken {
		t.Errorf("refresh did not rotate: old=%q new=%q", tokens.RefreshToken, rotated.RefreshToken)
	}

	// Step 6: revoke the rotated refresh token.
	revokeForm := url.Values{
		"client_id":     {"reporter"},
		"client_secret": {"top-secret"},
		"token":         {rotated.RefreshToken},
	}.Encode()
	r = httptest.NewRequest(http.MethodPost, "/oauth/revoke", strings.NewReader(revokeForm))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	revoke.ServeHTTP(rec, r)
	if rec.Code != http.StatusOK {
		t.Fatalf("revoke: want 200, got %d", rec.Code)
	}

	// Step 7: pin that the revoked refresh token can't be used.
	r = httptest.NewRequest(http.MethodPost, "/oauth/token",
		strings.NewReader(url.Values{
			"grant_type":    {"refresh_token"},
			"client_id":     {"reporter"},
			"client_secret": {"top-secret"},
			"refresh_token": {rotated.RefreshToken},
		}.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	tokenHandler3 := *token
	(&tokenHandler3).ServeHTTP(rec, r)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("revoked refresh should be rejected with 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	var errBody map[string]any
	json.Unmarshal(rec.Body.Bytes(), &errBody)
	if errBody["error"] != "invalid_grant" {
		t.Errorf("want invalid_grant, got %v", errBody["error"])
	}

	// Step 8: pin that the original (rotated-out) refresh token
	// is now flagged as REPLAY.
	r = httptest.NewRequest(http.MethodPost, "/oauth/token",
		strings.NewReader(url.Values{
			"grant_type":    {"refresh_token"},
			"client_id":     {"reporter"},
			"client_secret": {"top-secret"},
			"refresh_token": {tokens.RefreshToken},
		}.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	tokenHandler4 := *token
	(&tokenHandler4).ServeHTTP(rec, r)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("replayed refresh should be rejected with 400, got %d body=%s", rec.Code, rec.Body.String())
	}

	// Step 9: pin that consent was cached — a second authorize
	// call with the same scopes skips the screen.
	rec = httptest.NewRecorder()
	authorize.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, authzURL+"&_=2", nil))
	if rec.Code != http.StatusFound {
		t.Fatalf("second authorize should skip consent (cached), got %d body=%s", rec.Code, rec.Body.String())
	}

}
