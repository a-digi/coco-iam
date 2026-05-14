package oauthserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/a-digi/coco-iam/src/oauthserver/entity"
)

// ------- fakes -----------------------------------------------------

type fakeRegistry struct {
	client *entity.Client
	err    error
}

func (f *fakeRegistry) FindByClientID(_ context.Context, _, _ string) (*entity.Client, error) {
	return f.client, f.err
}
func (f *fakeRegistry) VerifySecret(_ context.Context, _ *entity.Client, _ string) error { return nil }

type fakeCodes struct {
	mintErr  error
	minted   string
	mintCall *CodeMintInput
}

func (f *fakeCodes) Mint(_ context.Context, in CodeMintInput, _ time.Duration) (string, error) {
	if f.mintErr != nil {
		return "", f.mintErr
	}
	cp := in
	f.mintCall = &cp
	return f.minted, nil
}
func (f *fakeCodes) ConsumeOnce(_ context.Context, _ string) (*entity.AuthorizationCode, error) {
	return nil, nil
}
func (f *fakeCodes) DeleteExpired(_ context.Context, _ time.Time) (int, error) { return 0, nil }

type fakeConsents struct {
	stored *entity.Consent
	loaded bool
	loadErr error
	recordCalled bool
	recordedScopes []string
}

func (f *fakeConsents) Load(_ context.Context, _, _, _ string) (*entity.Consent, error) {
	f.loaded = true
	if f.loadErr != nil {
		return nil, f.loadErr
	}
	if f.stored == nil {
		return nil, entity.ErrConsentNotFound
	}
	return f.stored, nil
}
func (f *fakeConsents) Record(_ context.Context, _, _, _ string, scopes []string) error {
	f.recordCalled = true
	cp := append([]string(nil), scopes...)
	f.recordedScopes = cp
	return nil
}
func (f *fakeConsents) Revoke(_ context.Context, _, _, _ string) error { return nil }

type fakeAuth struct {
	user, org string
	err       error
}

func (f *fakeAuth) CurrentUser(_ context.Context, _ RequestInfo) (string, string, error) {
	return f.user, f.org, f.err
}

func newAuthorizeHandler(reg *fakeRegistry, codes *fakeCodes, consents *fakeConsents, auth *fakeAuth) *AuthorizeHandler {
	return &AuthorizeHandler{
		ApplicationIDFromRequest: func(_ *http.Request) (string, string, error) {
			return "app-1", "org-1", nil
		},
		Clients:  reg,
		Codes:    codes,
		Consents: consents,
		Auth:     auth,
		LoginRedirectURL: func(_ *http.Request, returnTo string) string {
			return "https://login.example/sign-in?return_to=" + url.QueryEscape(returnTo)
		},
	}
}

func activeClient(uris []string, scopes []string, requireConsent bool) *entity.Client {
	return &entity.Client{
		ID:             "client-row-1",
		ApplicationID:  "app-1",
		ClientID:       "cid-1",
		Type:           entity.ClientTypeConfidential,
		DisplayName:    "Test Client",
		RedirectURIs:   uris,
		AllowedScopes:  scopes,
		RequireConsent: requireConsent,
		IsActive:       true,
	}
}

func authorizeURL(extra map[string]string) *http.Request {
	q := url.Values{}
	q.Set("client_id", "cid-1")
	q.Set("redirect_uri", "https://app.example/cb")
	q.Set("response_type", "code")
	q.Set("scope", "openid profile email")
	q.Set("state", "state-xyz")
	q.Set("code_challenge", "chal-xyz")
	q.Set("code_challenge_method", "S256")
	q.Set("nonce", "nonce-xyz")
	for k, v := range extra {
		if v == "" {
			q.Del(k)
		} else {
			q.Set(k, v)
		}
	}
	return httptest.NewRequest(http.MethodGet, "/oauth/authorize?"+q.Encode(), nil)
}

// ------- ParseAuthorizeRequest -------------------------------------

func TestParseAuthorizeRequest(t *testing.T) {
	q := url.Values{}
	q.Set("client_id", "  cid  ")
	q.Set("redirect_uri", "https://x/cb")
	q.Set("response_type", "code")
	q.Set("scope", "openid")
	got := ParseAuthorizeRequest(q)
	if got.ClientID != "cid" {
		t.Errorf("client_id should be trimmed, got %q", got.ClientID)
	}
}

// ------- ValidateAuthorizeRequest ---------------------------------

func TestValidate_HappyPath(t *testing.T) {
	reg := &fakeRegistry{client: activeClient([]string{"https://app.example/cb"}, []string{"openid", "profile"}, true)}
	req := ParseAuthorizeRequest(authorizeURL(nil).URL.Query())
	d, err := ValidateAuthorizeRequest(context.Background(), "app-1", req, reg)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !reflect.DeepEqual(d.GrantedScopes, []string{"openid", "profile"}) {
		t.Errorf("scope filter: %v", d.GrantedScopes)
	}
	if !d.NeedsConsent {
		t.Error("RequireConsent client should need consent")
	}
}

func TestValidate_MissingClientID(t *testing.T) {
	reg := &fakeRegistry{}
	req := ParseAuthorizeRequest(authorizeURL(map[string]string{"client_id": ""}).URL.Query())
	_, err := ValidateAuthorizeRequest(context.Background(), "app-1", req, reg)
	var oe *entity.OAuthError
	if !errors.As(err, &oe) || oe.Code != entity.ErrCodeInvalidRequest {
		t.Errorf("want invalid_request, got %v", err)
	}
}

func TestValidate_UnknownClient(t *testing.T) {
	reg := &fakeRegistry{err: entity.ErrClientNotFound}
	req := ParseAuthorizeRequest(authorizeURL(nil).URL.Query())
	_, err := ValidateAuthorizeRequest(context.Background(), "app-1", req, reg)
	var oe *entity.OAuthError
	if !errors.As(err, &oe) || oe.Code != entity.ErrCodeInvalidClient {
		t.Errorf("want invalid_client, got %v", err)
	}
}

func TestValidate_RedirectURIMismatch(t *testing.T) {
	reg := &fakeRegistry{client: activeClient([]string{"https://app.example/cb"}, []string{"openid"}, true)}
	req := ParseAuthorizeRequest(authorizeURL(map[string]string{"redirect_uri": "https://attacker/cb"}).URL.Query())
	_, err := ValidateAuthorizeRequest(context.Background(), "app-1", req, reg)
	var oe *entity.OAuthError
	if !errors.As(err, &oe) || oe.Code != entity.ErrCodeInvalidRequest {
		t.Errorf("want invalid_request on uri mismatch, got %v", err)
	}
}

func TestValidate_UnsupportedResponseType(t *testing.T) {
	reg := &fakeRegistry{client: activeClient([]string{"https://app.example/cb"}, []string{"openid"}, true)}
	req := ParseAuthorizeRequest(authorizeURL(map[string]string{"response_type": "token"}).URL.Query())
	_, err := ValidateAuthorizeRequest(context.Background(), "app-1", req, reg)
	var oe *entity.OAuthError
	if !errors.As(err, &oe) || oe.Code != entity.ErrCodeUnsupportedResponseType {
		t.Errorf("want unsupported_response_type, got %v", err)
	}
}

func TestValidate_PKCERequired(t *testing.T) {
	reg := &fakeRegistry{client: activeClient([]string{"https://app.example/cb"}, []string{"openid"}, true)}
	req := ParseAuthorizeRequest(authorizeURL(map[string]string{"code_challenge": ""}).URL.Query())
	_, err := ValidateAuthorizeRequest(context.Background(), "app-1", req, reg)
	var oe *entity.OAuthError
	if !errors.As(err, &oe) || oe.Code != entity.ErrCodeInvalidRequest {
		t.Errorf("want invalid_request when PKCE missing, got %v", err)
	}
}

func TestValidate_RejectsPlainPKCE(t *testing.T) {
	reg := &fakeRegistry{client: activeClient([]string{"https://app.example/cb"}, []string{"openid"}, true)}
	req := ParseAuthorizeRequest(authorizeURL(map[string]string{"code_challenge_method": "plain"}).URL.Query())
	_, err := ValidateAuthorizeRequest(context.Background(), "app-1", req, reg)
	var oe *entity.OAuthError
	if !errors.As(err, &oe) || oe.Code != entity.ErrCodeInvalidRequest {
		t.Errorf("want invalid_request for plain PKCE, got %v", err)
	}
}

func TestValidate_AllowedScopeFilter(t *testing.T) {
	// Client allows only openid; user requested profile + email.
	// Expected: granted=openid, no error.
	reg := &fakeRegistry{client: activeClient([]string{"https://app.example/cb"}, []string{"openid"}, true)}
	req := ParseAuthorizeRequest(authorizeURL(map[string]string{"scope": "openid email"}).URL.Query())
	d, err := ValidateAuthorizeRequest(context.Background(), "app-1", req, reg)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !reflect.DeepEqual(d.GrantedScopes, []string{"openid"}) {
		t.Errorf("granted: %v", d.GrantedScopes)
	}
}

func TestValidate_AllScopesDisallowed(t *testing.T) {
	reg := &fakeRegistry{client: activeClient([]string{"https://app.example/cb"}, []string{"openid"}, true)}
	req := ParseAuthorizeRequest(authorizeURL(map[string]string{"scope": "email profile"}).URL.Query())
	_, err := ValidateAuthorizeRequest(context.Background(), "app-1", req, reg)
	var oe *entity.OAuthError
	if !errors.As(err, &oe) || oe.Code != entity.ErrCodeInvalidScope {
		t.Errorf("want invalid_scope, got %v", err)
	}
}

func TestValidate_InactiveClient(t *testing.T) {
	c := activeClient([]string{"https://app.example/cb"}, []string{"openid"}, true)
	c.IsActive = false
	reg := &fakeRegistry{client: c}
	req := ParseAuthorizeRequest(authorizeURL(nil).URL.Query())
	_, err := ValidateAuthorizeRequest(context.Background(), "app-1", req, reg)
	var oe *entity.OAuthError
	if !errors.As(err, &oe) || oe.Code != entity.ErrCodeInvalidClient {
		t.Errorf("want invalid_client when inactive, got %v", err)
	}
}

// ------- ResolveConsent -------------------------------------------

func TestResolveConsent_NoConsentNeededShortCircuits(t *testing.T) {
	cs := &fakeConsents{}
	d := &AuthorizeDecision{NeedsConsent: false}
	if err := ResolveConsent(context.Background(), "org", "user", d, cs); err != nil {
		t.Fatalf("ResolveConsent: %v", err)
	}
	if cs.loaded {
		t.Error("should not query consent store when NeedsConsent=false")
	}
}

func TestResolveConsent_CachedSubsetSkipsScreen(t *testing.T) {
	cs := &fakeConsents{stored: &entity.Consent{
		ClientRowID: "client-row-1",
		GrantedScopes: []string{"openid", "profile", "email"},
	}}
	d := &AuthorizeDecision{
		Client:        &entity.Client{ID: "client-row-1"},
		GrantedScopes: []string{"openid", "profile"},
		NeedsConsent:  true,
	}
	if err := ResolveConsent(context.Background(), "org", "user", d, cs); err != nil {
		t.Fatalf("ResolveConsent: %v", err)
	}
	if d.NeedsConsent {
		t.Error("cached consent that's a superset of requested should skip the screen")
	}
}

func TestResolveConsent_CachedDoesNotCoverNewScope(t *testing.T) {
	cs := &fakeConsents{stored: &entity.Consent{
		ClientRowID: "client-row-1",
		GrantedScopes: []string{"openid"},
	}}
	d := &AuthorizeDecision{
		Client:        &entity.Client{ID: "client-row-1"},
		GrantedScopes: []string{"openid", "email"},
		NeedsConsent:  true,
	}
	if err := ResolveConsent(context.Background(), "org", "user", d, cs); err != nil {
		t.Fatalf("ResolveConsent: %v", err)
	}
	if !d.NeedsConsent {
		t.Error("widening request must force re-consent")
	}
}

// ------- AuthorizeHandler smoke -----------------------------------

func TestAuthorizeHandler_UnauthenticatedRedirectsToLogin(t *testing.T) {
	reg := &fakeRegistry{client: activeClient([]string{"https://app.example/cb"}, []string{"openid"}, true)}
	auth := &fakeAuth{} // no user
	h := newAuthorizeHandler(reg, &fakeCodes{}, &fakeConsents{}, auth)

	req := authorizeURL(nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("want 302, got %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "https://login.example/sign-in?") {
		t.Errorf("wrong login URL: %s", loc)
	}
	if !strings.Contains(loc, url.QueryEscape("/oauth/authorize?")) {
		t.Errorf("login URL should carry return_to, got %s", loc)
	}
}

func TestAuthorizeHandler_AuthenticatedNoConsentRequiredIssuesCode(t *testing.T) {
	reg := &fakeRegistry{client: activeClient([]string{"https://app.example/cb"}, []string{"openid", "profile"}, false)}
	codes := &fakeCodes{minted: "the-code"}
	h := newAuthorizeHandler(reg, codes, &fakeConsents{}, &fakeAuth{user: "user-1", org: "org-1"})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, authorizeURL(nil))
	if rec.Code != http.StatusFound {
		t.Fatalf("want 302, got %d body=%s", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "https://app.example/cb?") {
		t.Errorf("wrong redirect target: %s", loc)
	}
	parsed, _ := url.Parse(loc)
	if parsed.Query().Get("code") != "the-code" {
		t.Errorf("missing code in redirect: %s", loc)
	}
	if parsed.Query().Get("state") != "state-xyz" {
		t.Errorf("missing state echo: %s", loc)
	}
	if codes.mintCall == nil || codes.mintCall.UserID != "user-1" {
		t.Errorf("Mint not called with right user: %+v", codes.mintCall)
	}
}

func TestAuthorizeHandler_AuthenticatedConsentRequiredRendersScreen(t *testing.T) {
	reg := &fakeRegistry{client: activeClient([]string{"https://app.example/cb"}, []string{"openid", "profile"}, true)}
	h := newAuthorizeHandler(reg, &fakeCodes{}, &fakeConsents{}, &fakeAuth{user: "user-1", org: "org-1"})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, authorizeURL(nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("consent screen want 200, got %d", rec.Code)
	}
	var payload struct {
		Consent ConsentRenderParams `json:"consent"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode consent: %v", err)
	}
	if payload.Consent.ClientID != "cid-1" {
		t.Errorf("consent client_id: %q", payload.Consent.ClientID)
	}
	if !reflect.DeepEqual(payload.Consent.RequestedScopes, []string{"openid", "profile"}) {
		t.Errorf("scopes on screen: %v", payload.Consent.RequestedScopes)
	}
}

func TestAuthorizeHandler_PostApproveMintsCodeAndRecordsConsent(t *testing.T) {
	reg := &fakeRegistry{client: activeClient([]string{"https://app.example/cb"}, []string{"openid", "profile"}, true)}
	codes := &fakeCodes{minted: "the-code"}
	consents := &fakeConsents{}
	h := newAuthorizeHandler(reg, codes, consents, &fakeAuth{user: "user-1", org: "org-1"})

	q := authorizeURL(nil).URL.Query()
	q.Set("approve", "yes")
	r := httptest.NewRequest(http.MethodPost, "/oauth/authorize?"+q.Encode(), nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)

	if rec.Code != http.StatusFound {
		t.Fatalf("want 302, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !consents.recordCalled {
		t.Error("Record(consent) should have been called")
	}
	if !reflect.DeepEqual(consents.recordedScopes, []string{"openid", "profile"}) {
		t.Errorf("recorded scopes: %v", consents.recordedScopes)
	}
}

func TestAuthorizeHandler_PostDenyRedirectsWithError(t *testing.T) {
	reg := &fakeRegistry{client: activeClient([]string{"https://app.example/cb"}, []string{"openid"}, true)}
	h := newAuthorizeHandler(reg, &fakeCodes{}, &fakeConsents{}, &fakeAuth{user: "user-1", org: "org-1"})

	q := authorizeURL(nil).URL.Query()
	// approve param absent → user denied
	r := httptest.NewRequest(http.MethodPost, "/oauth/authorize?"+q.Encode(), nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)

	if rec.Code != http.StatusFound {
		t.Fatalf("want 302 redirect, got %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "https://app.example/cb?") || !strings.Contains(loc, "error=access_denied") {
		t.Errorf("expected access_denied redirect, got %s", loc)
	}
}

func TestAuthorizeHandler_CrossOrgSessionRejected(t *testing.T) {
	reg := &fakeRegistry{client: activeClient([]string{"https://app.example/cb"}, []string{"openid"}, true)}
	// Session belongs to org-2 but the URL resolves to org-1.
	h := newAuthorizeHandler(reg, &fakeCodes{}, &fakeConsents{}, &fakeAuth{user: "user-1", org: "org-2"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, authorizeURL(nil))
	// Redirect with error since redirect_uri is validated.
	if rec.Code != http.StatusFound {
		t.Fatalf("want 302, got %d", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Location"), "error=access_denied") {
		t.Errorf("want access_denied in redirect: %s", rec.Header().Get("Location"))
	}
}

func TestAuthorizeHandler_BadRedirectURIIsServerRendered(t *testing.T) {
	// When the redirect_uri itself fails validation, we MUST
	// NOT redirect anywhere — return JSON error inline.
	reg := &fakeRegistry{client: activeClient([]string{"https://app.example/cb"}, []string{"openid"}, true)}
	h := newAuthorizeHandler(reg, &fakeCodes{}, &fakeConsents{}, &fakeAuth{user: "u", org: "org-1"})
	req := authorizeURL(map[string]string{"redirect_uri": "https://attacker/cb"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rec.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["error"] != "invalid_request" {
		t.Errorf("error code: %v", body["error"])
	}
}

func TestAuthorizeHandler_CachedConsentSkipsScreen(t *testing.T) {
	reg := &fakeRegistry{client: activeClient([]string{"https://app.example/cb"}, []string{"openid"}, true)}
	codes := &fakeCodes{minted: "the-code"}
	consents := &fakeConsents{stored: &entity.Consent{
		ClientRowID: "client-row-1",
		GrantedScopes: []string{"openid", "profile", "email"},
	}}
	h := newAuthorizeHandler(reg, codes, consents, &fakeAuth{user: "u", org: "org-1"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, authorizeURL(nil))
	if rec.Code != http.StatusFound {
		t.Fatalf("want 302 (skip consent), got %d body=%s", rec.Code, rec.Body.String())
	}
	if consents.recordCalled {
		t.Error("Record should not be called when consent was reused")
	}
}
