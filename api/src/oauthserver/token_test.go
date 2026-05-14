package oauthserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/a-digi/coco-iam/src/oauthserver/entity"
	"github.com/a-digi/coco-iam/src/oauthserver/pkce"
)

// ------- fakes ----------------------------------------------------

type fakeRefresh struct {
	mintRaw    string
	mintRecord *entity.RefreshToken
	mintErr    error
	findRec    *entity.RefreshToken
	findErr    error
	rotateCalls int
	revokeCalls  int
	revokeFamilyCalls int
}

func (f *fakeRefresh) Mint(_ context.Context, clientRowID, applicationID, userID string, scopes []string, _ time.Duration) (string, *entity.RefreshToken, error) {
	if f.mintErr != nil {
		return "", nil, f.mintErr
	}
	if f.mintRecord == nil {
		f.mintRecord = &entity.RefreshToken{
			ID:            "new-refresh-id",
			ClientRowID:   clientRowID,
			ApplicationID: applicationID,
			UserID:        userID,
			Scopes:        scopes,
		}
	}
	return f.mintRaw, f.mintRecord, nil
}
func (f *fakeRefresh) FindUnconsumed(_ context.Context, _ string) (*entity.RefreshToken, error) {
	return f.findRec, f.findErr
}
func (f *fakeRefresh) Rotate(_ context.Context, _, _ string) error { f.rotateCalls++; return nil }
func (f *fakeRefresh) Revoke(_ context.Context, _ string) error    { f.revokeCalls++; return nil }
func (f *fakeRefresh) RevokeFamily(_ context.Context, _ string) error {
	f.revokeFamilyCalls++
	return nil
}

type fakeClaims struct{ extra map[string]any }

func (f *fakeClaims) LoadClaims(_ context.Context, _, _ string, _ []string) (map[string]any, error) {
	return f.extra, nil
}

type fakeSigner struct{ access, idToken string }

func (f *fakeSigner) SignAccessToken(_ context.Context, _ string, _ map[string]any) (string, error) {
	return f.access, nil
}
func (f *fakeSigner) SignIDToken(_ context.Context, _ string, _ map[string]any) (string, error) {
	return f.idToken, nil
}

type fakeCodes2 struct {
	rec    *entity.AuthorizationCode
	err    error
	consumed int
}

func (f *fakeCodes2) Mint(_ context.Context, _ CodeMintInput, _ time.Duration) (string, error) {
	return "code-not-used", nil
}
func (f *fakeCodes2) ConsumeOnce(_ context.Context, _ string) (*entity.AuthorizationCode, error) {
	f.consumed++
	if f.err != nil {
		return nil, f.err
	}
	return f.rec, nil
}
func (f *fakeCodes2) DeleteExpired(_ context.Context, _ time.Time) (int, error) { return 0, nil }

type fakeRegistry2 struct{ client *entity.Client; err error }

func (f *fakeRegistry2) FindByClientID(_ context.Context, _, _ string) (*entity.Client, error) {
	return f.client, f.err
}
func (f *fakeRegistry2) VerifySecret(_ context.Context, _ *entity.Client, _ string) error { return nil }

func newTokenHandler(reg *fakeRegistry2, codes *fakeCodes2, refresh *fakeRefresh, claims *fakeClaims, signer *fakeSigner) *TokenHandler {
	return &TokenHandler{
		ApplicationIDFromRequest: func(_ *http.Request) (string, string, error) {
			return "app-1", "org-1", nil
		},
		IssuerFromRequest: func(_ *http.Request, _ string) string {
			return "https://iam.example/a/acme/prod/web"
		},
		Clients: reg,
		Codes:   codes,
		Refresh: refresh,
		Claims:  claims,
		Signer:  signer,
	}
}

func sampleClient() *entity.Client {
	return &entity.Client{
		ID:              "client-row-1",
		ApplicationID:   "app-1",
		ClientID:        "cid-1",
		Type:            entity.ClientTypeConfidential,
		IsActive:        true,
		AccessTokenTTL:  3600,
		RefreshTokenTTL: 1209600,
		AllowedScopes:   []string{"openid", "profile", "email", "offline_access"},
	}
}

// PKCE pair we reuse across tests.
const sampleVerifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
var sampleChallenge = pkce.DeriveChallenge(sampleVerifier)

func tokenForm(extra map[string]string) *http.Request {
	form := url.Values{}
	form.Set("client_id", "cid-1")
	form.Set("client_secret", "secret")
	form.Set("grant_type", "authorization_code")
	form.Set("code", "the-code")
	form.Set("code_verifier", sampleVerifier)
	form.Set("redirect_uri", "https://app.example/cb")
	for k, v := range extra {
		if v == "" {
			form.Del(k)
		} else {
			form.Set(k, v)
		}
	}
	r := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r
}

// ------- code grant -----------------------------------------------

func TestToken_AuthorizationCode_HappyPath(t *testing.T) {
	codes := &fakeCodes2{rec: &entity.AuthorizationCode{
		ClientRowID:         "client-row-1",
		ApplicationID:       "app-1",
		UserID:              "user-1",
		RedirectURI:         "https://app.example/cb",
		Scopes:              []string{"openid", "profile"},
		CodeChallenge:       sampleChallenge,
		CodeChallengeMethod: "S256",
		Nonce:               "nonce-xyz",
	}}
	signer := &fakeSigner{access: "ACCESS-JWT", idToken: "ID-JWT"}
	h := newTokenHandler(&fakeRegistry2{client: sampleClient()}, codes,
		&fakeRefresh{}, &fakeClaims{extra: map[string]any{"email": "a@b"}}, signer)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, tokenForm(nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var out TokenResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.AccessToken != "ACCESS-JWT" {
		t.Errorf("access token: %q", out.AccessToken)
	}
	if out.IDToken != "ID-JWT" {
		t.Errorf("id token expected because openid scope present: %q", out.IDToken)
	}
	if out.RefreshToken != "" {
		t.Errorf("no offline_access — refresh should not be issued: %q", out.RefreshToken)
	}
	if codes.consumed != 1 {
		t.Errorf("code should be consumed once: %d", codes.consumed)
	}
}

func TestToken_AuthorizationCode_OfflineAccessIssuesRefresh(t *testing.T) {
	codes := &fakeCodes2{rec: &entity.AuthorizationCode{
		ClientRowID: "client-row-1", ApplicationID: "app-1", UserID: "user-1",
		RedirectURI: "https://app.example/cb",
		Scopes: []string{"openid", "offline_access"},
		CodeChallenge: sampleChallenge, CodeChallengeMethod: "S256",
	}}
	rt := &fakeRefresh{mintRaw: "REFRESH"}
	h := newTokenHandler(&fakeRegistry2{client: sampleClient()}, codes, rt,
		&fakeClaims{extra: map[string]any{}}, &fakeSigner{access: "AT", idToken: "IDT"})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, tokenForm(nil))
	var out TokenResponse
	json.Unmarshal(rec.Body.Bytes(), &out)
	if out.RefreshToken != "REFRESH" {
		t.Errorf("offline_access scope should mint refresh, got %q", out.RefreshToken)
	}
}

func TestToken_AuthorizationCode_RejectedRedirectURIMismatch(t *testing.T) {
	codes := &fakeCodes2{rec: &entity.AuthorizationCode{
		ClientRowID: "client-row-1", ApplicationID: "app-1", UserID: "u",
		RedirectURI: "https://app.example/cb",
		Scopes: []string{"openid"},
		CodeChallenge: sampleChallenge, CodeChallengeMethod: "S256",
	}}
	h := newTokenHandler(&fakeRegistry2{client: sampleClient()}, codes,
		&fakeRefresh{}, &fakeClaims{}, &fakeSigner{access: "x"})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, tokenForm(map[string]string{"redirect_uri": "https://attacker/cb"}))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rec.Code)
	}
}

func TestToken_AuthorizationCode_PKCEMismatch(t *testing.T) {
	codes := &fakeCodes2{rec: &entity.AuthorizationCode{
		ClientRowID: "client-row-1", ApplicationID: "app-1", UserID: "u",
		RedirectURI: "https://app.example/cb",
		Scopes: []string{"openid"},
		CodeChallenge: sampleChallenge, CodeChallengeMethod: "S256",
	}}
	h := newTokenHandler(&fakeRegistry2{client: sampleClient()}, codes,
		&fakeRefresh{}, &fakeClaims{}, &fakeSigner{access: "x"})
	// wrong verifier
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, tokenForm(map[string]string{"code_verifier": "wrong-verifier-but-43chars-long-for-rfc-7636-rules"}))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body["error"] != "invalid_grant" {
		t.Errorf("want invalid_grant, got %v", body["error"])
	}
}

func TestToken_AuthorizationCode_CrossClientCodeRejected(t *testing.T) {
	codes := &fakeCodes2{rec: &entity.AuthorizationCode{
		ClientRowID: "OTHER-CLIENT", ApplicationID: "app-1", UserID: "u",
		RedirectURI: "https://app.example/cb",
		Scopes: []string{"openid"},
		CodeChallenge: sampleChallenge, CodeChallengeMethod: "S256",
	}}
	h := newTokenHandler(&fakeRegistry2{client: sampleClient()}, codes,
		&fakeRefresh{}, &fakeClaims{}, &fakeSigner{access: "x"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, tokenForm(nil))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400 (invalid_grant), got %d", rec.Code)
	}
}

func TestToken_UnknownClient(t *testing.T) {
	codes := &fakeCodes2{}
	h := newTokenHandler(&fakeRegistry2{err: entity.ErrClientNotFound}, codes,
		&fakeRefresh{}, &fakeClaims{}, &fakeSigner{access: "x"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, tokenForm(nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401 invalid_client, got %d", rec.Code)
	}
}

func TestToken_UnsupportedGrantType(t *testing.T) {
	codes := &fakeCodes2{}
	h := newTokenHandler(&fakeRegistry2{client: sampleClient()}, codes,
		&fakeRefresh{}, &fakeClaims{}, &fakeSigner{access: "x"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, tokenForm(map[string]string{"grant_type": "password"}))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rec.Code)
	}
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body["error"] != "unsupported_grant_type" {
		t.Errorf("want unsupported_grant_type, got %v", body["error"])
	}
}

// ------- refresh grant --------------------------------------------

func TestToken_Refresh_HappyPathRotates(t *testing.T) {
	rt := &fakeRefresh{
		findRec: &entity.RefreshToken{
			ID: "old-id", ClientRowID: "client-row-1",
			ApplicationID: "app-1", UserID: "user-1",
			Scopes: []string{"openid", "offline_access"},
		},
		mintRaw: "NEW-REFRESH",
	}
	h := newTokenHandler(&fakeRegistry2{client: sampleClient()}, &fakeCodes2{}, rt,
		&fakeClaims{extra: map[string]any{}}, &fakeSigner{access: "AT", idToken: "IDT"})

	form := url.Values{}
	form.Set("client_id", "cid-1")
	form.Set("client_secret", "s")
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", "OLD-REFRESH")
	r := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var out TokenResponse
	json.Unmarshal(rec.Body.Bytes(), &out)
	if out.RefreshToken != "NEW-REFRESH" {
		t.Errorf("want rotated refresh, got %q", out.RefreshToken)
	}
	if rt.rotateCalls != 1 {
		t.Errorf("want 1 Rotate call, got %d", rt.rotateCalls)
	}
}

func TestToken_Refresh_ScopeWideningRejected(t *testing.T) {
	rt := &fakeRefresh{findRec: &entity.RefreshToken{
		ID: "old-id", ClientRowID: "client-row-1",
		Scopes: []string{"openid"}, // original
	}}
	h := newTokenHandler(&fakeRegistry2{client: sampleClient()}, &fakeCodes2{}, rt,
		&fakeClaims{}, &fakeSigner{access: "x"})

	form := url.Values{}
	form.Set("client_id", "cid-1")
	form.Set("client_secret", "s")
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", "OLD")
	form.Set("scope", "openid email") // widening
	r := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rec.Code)
	}
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body["error"] != "invalid_scope" {
		t.Errorf("want invalid_scope, got %v", body["error"])
	}
}

func TestToken_Refresh_ReplayDetected(t *testing.T) {
	rt := &fakeRefresh{findErr: entity.ErrReplayDetected}
	h := newTokenHandler(&fakeRegistry2{client: sampleClient()}, &fakeCodes2{}, rt,
		&fakeClaims{}, &fakeSigner{access: "x"})

	form := url.Values{}
	form.Set("client_id", "cid-1")
	form.Set("client_secret", "s")
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", "REPLAYED")
	r := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400 invalid_grant, got %d", rec.Code)
	}
}

func TestToken_Refresh_TokenForDifferentClientRejected(t *testing.T) {
	rt := &fakeRefresh{findRec: &entity.RefreshToken{
		ID: "x", ClientRowID: "OTHER-CLIENT", Scopes: []string{"openid"},
	}}
	h := newTokenHandler(&fakeRegistry2{client: sampleClient()}, &fakeCodes2{}, rt,
		&fakeClaims{}, &fakeSigner{access: "x"})

	form := url.Values{}
	form.Set("client_id", "cid-1")
	form.Set("client_secret", "s")
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", "OLD")
	r := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// ------- pure helpers ---------------------------------------------

func TestAccessTokenHash_DeterministicHalfSHA256(t *testing.T) {
	a := accessTokenHash("hello")
	b := accessTokenHash("hello")
	if a != b {
		t.Error("hash should be deterministic")
	}
	// SHA-256 → 32 bytes, half = 16 bytes → 22 chars
	// base64url no-pad.
	if len(a) != 22 {
		t.Errorf("unexpected length %d", len(a))
	}
}

func TestRandJTI_NotEmpty(t *testing.T) {
	if randJTI() == "" {
		t.Error("randJTI should return non-empty value")
	}
}

// Unused import guard in case errors becomes unused above.
var _ = errors.New
