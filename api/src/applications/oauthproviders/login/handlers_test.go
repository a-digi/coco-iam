package login

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/a-digi/coco-iam/src/applications/oauthproviders/adapters"
	"github.com/a-digi/coco-iam/src/applications/oauthproviders/authstate"
	"github.com/a-digi/coco-iam/src/applications/oauthproviders/entity"
	"github.com/a-digi/coco-server/server/request"

	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

// ------- fakes ------------------------------------------------------

type fakeSlugs struct {
	appID, orgID string
	err          error
}

func (f *fakeSlugs) ResolveSlugs(_, _, _ string) (string, string, error) {
	return f.appID, f.orgID, f.err
}

type fakeProviders struct {
	cfg *entity.ProviderConfig
	err error
}

func (f *fakeProviders) FindByProvider(_ string, _ entity.Provider) (*entity.ProviderConfig, error) {
	return f.cfg, f.err
}

type fakeResolver struct {
	authorizeURL string
	access       string
	idToken      string
	exchangeErr  error
	identity     entity.Identity
	identityErr  error
}

func (f *fakeResolver) AuthorizeURL(_ entity.ProviderConfig, state, chal, redirect string) (string, error) {
	return f.authorizeURL + "?state=" + state + "&chal=" + chal + "&rd=" + redirect, nil
}

func (f *fakeResolver) ExchangeCode(_ context.Context, _ entity.ProviderConfig, _, _, _ string) (string, string, error) {
	return f.access, f.idToken, f.exchangeErr
}

func (f *fakeResolver) FetchIdentity(_ context.Context, _ entity.ProviderConfig, _, _ string) (entity.Identity, error) {
	return f.identity, f.identityErr
}

func resolverFactoryOf(r adapters.IdentityResolver) ResolverFactory {
	return func(_ entity.Provider) adapters.IdentityResolver { return r }
}

type fakeTokens struct {
	access, refresh string
	err             error
	calls           int
}

func (f *fakeTokens) IssueLoginTokens(_ context.Context, _, _ string, _ []string, _ map[string][]string) (string, string, error) {
	f.calls++
	return f.access, f.refresh, f.err
}

type recordingDispatcher struct {
	lastReturnURL string
	lastAccess    string
}

func (d *recordingDispatcher) Redirect(returnURL, access, refresh string) (string, error) {
	d.lastReturnURL = returnURL
	d.lastAccess = access
	return returnURL + "#access_token=" + access + "&refresh_token=" + refresh, nil
}

// stubDI is a minimal DI for request.NewContext — the handlers
// don't reach into it.
type stubDI struct{}

func (stubDI) GetDatabaseManager() interface{} { return nil }
func (stubDI) GetLogger() interface{}          { return nil }

// Because request.NewContext expects the concrete di.Context
// type, wrap via a trivial interface-satisfying stub.
type diCtx struct{}

func (diCtx) GetDatabaseManager() interface{} { return nil }
func (diCtx) GetLogger() interface{}          { return nil }

// openStateStore makes a scratch Store over an in-memory DB.
func openStateStore(t *testing.T) *authstate.Store {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(`
		CREATE TABLE oauth_auth_requests (
		    state TEXT PRIMARY KEY, application_id TEXT, provider TEXT,
		    code_verifier TEXT, return_url TEXT, created_at DATETIME
		);
	`); err != nil {
		t.Fatalf("schema: %v", err)
	}
	return authstate.New(db, nil, 0, nil)
}

func serve(t *testing.T, method, path string, h interface {
	ServeHTTP(reqCtx request.RequestContext)
}) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	reqCtx := request.NewContext(rec, r, nil)
	h.ServeHTTP(reqCtx)
	return rec
}

func activeCfg() entity.ProviderConfig {
	return entity.ProviderConfig{
		ID:                "p-1",
		ApplicationID:     "app-1",
		Provider:          entity.ProviderGoogle,
		ClientID:          "cid",
		ClientSecret:      "sec",
		AllowLogin:        true,
		AllowRegistration: true,
		IsActive:          true,
	}
}

// ------- AuthorizeHandler -----------------------------------------

func TestAuthorize_HappyPathRedirectsToIdP(t *testing.T) {
	cfg := activeCfg()
	h := &AuthorizeHandler{
		Slugs:       &fakeSlugs{appID: "app-1", orgID: "org-1"},
		Providers:   &fakeProviders{cfg: &cfg},
		State:       openStateStore(t),
		Resolvers:   resolverFactoryOf(&fakeResolver{authorizeURL: "https://idp.example/auth"}),
		RedirectURI: "https://us.example",
	}
	rec := serve(t, http.MethodGet,
		"/a/acme/prod/web/auth/oauth/google/authorize?return_url=https%3A%2F%2Fapp%2Fcb",
		h)
	if rec.Code != http.StatusFound {
		t.Fatalf("want 302, got %d body=%s", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "https://idp.example/auth") {
		t.Errorf("wrong IdP URL: %s", loc)
	}
	if !strings.Contains(loc, "rd=https://us.example/a/acme/prod/web/auth/oauth/google/callback") {
		t.Errorf("redirect URI missing / wrong: %s", loc)
	}
}

func TestAuthorize_UnknownProviderReturns400(t *testing.T) {
	h := &AuthorizeHandler{
		Slugs:     &fakeSlugs{appID: "app-1", orgID: "org-1"},
		Providers: &fakeProviders{},
		State:     openStateStore(t),
		Resolvers: resolverFactoryOf(&fakeResolver{}),
	}
	rec := serve(t, http.MethodGet,
		"/a/acme/prod/web/auth/oauth/facebook/authorize?return_url=https%3A%2F%2Fapp",
		h)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

func TestAuthorize_InvalidReturnURLReturns400(t *testing.T) {
	cfg := activeCfg()
	h := &AuthorizeHandler{
		Slugs:     &fakeSlugs{appID: "app-1", orgID: "org-1"},
		Providers: &fakeProviders{cfg: &cfg},
		State:     openStateStore(t),
		Resolvers: resolverFactoryOf(&fakeResolver{}),
	}
	rec := serve(t, http.MethodGet,
		"/a/acme/prod/web/auth/oauth/google/authorize?return_url=javascript%3Aalert(1)",
		h)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400 on js scheme, got %d", rec.Code)
	}
}

func TestAuthorize_ProviderDisabledReturns403(t *testing.T) {
	cfg := activeCfg()
	cfg.AllowLogin = false
	h := &AuthorizeHandler{
		Slugs:     &fakeSlugs{appID: "app-1", orgID: "org-1"},
		Providers: &fakeProviders{cfg: &cfg},
		State:     openStateStore(t),
		Resolvers: resolverFactoryOf(&fakeResolver{}),
	}
	rec := serve(t, http.MethodGet,
		"/a/acme/prod/web/auth/oauth/google/authorize?return_url=https%3A%2F%2Fapp",
		h)
	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", rec.Code)
	}
}

func TestAuthorize_ProviderNotConfiguredReturns404(t *testing.T) {
	h := &AuthorizeHandler{
		Slugs:     &fakeSlugs{appID: "app-1"},
		Providers: &fakeProviders{err: entity.ErrProviderNotFound},
		State:     openStateStore(t),
		Resolvers: resolverFactoryOf(&fakeResolver{}),
	}
	rec := serve(t, http.MethodGet,
		"/a/acme/prod/web/auth/oauth/google/authorize?return_url=https%3A%2F%2Fapp",
		h)
	if rec.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rec.Code)
	}
}

// ------- CallbackHandler ------------------------------------------

func TestCallback_HappyPathMintsTokensAndRedirects(t *testing.T) {
	state := openStateStore(t)
	req, _, err := state.StartAuthRequest(authstate.StartInput{
		ApplicationID: "app-1", Provider: "google", ReturnURL: "https://app/cb",
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	cfg := activeCfg()
	lk := &fakeLinker{
		byIdentity: map[string]string{
			key(entity.ProviderGoogle, "sub-known"): "user-1",
		},
	}
	tokens := &fakeTokens{access: "AT", refresh: "RT"}
	disp := &recordingDispatcher{}

	h := &CallbackHandler{
		Slugs:     &fakeSlugs{appID: "app-1", orgID: "org-1"},
		Providers: &fakeProviders{cfg: &cfg},
		State:     state,
		Resolvers: resolverFactoryOf(&fakeResolver{
			access: "access-from-idp",
			identity: entity.Identity{
				Provider: entity.ProviderGoogle, Sub: "sub-known",
				Email: "alice@example.com", EmailVerified: true,
			},
		}),
		Linker:      lk,
		Tokens:      tokens,
		Dispatcher:  disp,
		RedirectURI: "https://us.example",
	}
	rec := serve(t, http.MethodGet,
		"/a/acme/prod/web/auth/oauth/google/callback?code=C&state="+req.State,
		h)
	if rec.Code != http.StatusFound {
		t.Fatalf("want 302, got %d body=%s", rec.Code, rec.Body.String())
	}
	if tokens.calls != 1 {
		t.Errorf("want 1 token call, got %d", tokens.calls)
	}
	if disp.lastAccess != "AT" {
		t.Errorf("dispatcher didn't see the access token: %q", disp.lastAccess)
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "https://app/cb#") || !strings.Contains(loc, "access_token=AT") {
		t.Errorf("redirect not built with fragment tokens: %s", loc)
	}
}

func TestCallback_MissingCodeOrStateReturns400(t *testing.T) {
	h := &CallbackHandler{
		Slugs: &fakeSlugs{}, Providers: &fakeProviders{},
		State: openStateStore(t), Resolvers: resolverFactoryOf(&fakeResolver{}),
		Tokens: &fakeTokens{}, Dispatcher: &recordingDispatcher{},
	}
	rec := serve(t, http.MethodGet,
		"/a/acme/prod/web/auth/oauth/google/callback", h)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

func TestCallback_StateMismatchReturns400(t *testing.T) {
	h := &CallbackHandler{
		Slugs: &fakeSlugs{}, Providers: &fakeProviders{},
		State: openStateStore(t), Resolvers: resolverFactoryOf(&fakeResolver{}),
		Tokens: &fakeTokens{}, Dispatcher: &recordingDispatcher{},
	}
	rec := serve(t, http.MethodGet,
		"/a/acme/prod/web/auth/oauth/google/callback?code=C&state=wrong",
		h)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400 on unknown state, got %d", rec.Code)
	}
}

func TestCallback_IdPReturnedErrorParamSurfacesAs401(t *testing.T) {
	h := &CallbackHandler{
		Slugs: &fakeSlugs{}, Providers: &fakeProviders{},
		State: openStateStore(t), Resolvers: resolverFactoryOf(&fakeResolver{}),
		Tokens: &fakeTokens{}, Dispatcher: &recordingDispatcher{},
	}
	rec := serve(t, http.MethodGet,
		"/a/acme/prod/web/auth/oauth/google/callback?error=access_denied&state=s",
		h)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rec.Code)
	}
}

func TestCallback_ExchangeFailureSurfacesAs400(t *testing.T) {
	state := openStateStore(t)
	req, _, _ := state.StartAuthRequest(authstate.StartInput{
		ApplicationID: "app-1", Provider: "google", ReturnURL: "https://app/cb",
	})
	cfg := activeCfg()
	h := &CallbackHandler{
		Slugs:     &fakeSlugs{appID: "app-1", orgID: "org-1"},
		Providers: &fakeProviders{cfg: &cfg},
		State:     state,
		Resolvers: resolverFactoryOf(&fakeResolver{exchangeErr: errors.New("boom")}),
		Linker:    &fakeLinker{},
		Tokens:    &fakeTokens{},
		Dispatcher: &recordingDispatcher{},
	}
	rec := serve(t, http.MethodGet,
		"/a/acme/prod/web/auth/oauth/google/callback?code=C&state="+req.State,
		h)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

func TestCallback_RegistrationClosedSurfacesAs403(t *testing.T) {
	state := openStateStore(t)
	req, _, _ := state.StartAuthRequest(authstate.StartInput{
		ApplicationID: "app-1", Provider: "google", ReturnURL: "https://app/cb",
	})
	cfg := activeCfg()
	cfg.AllowRegistration = false
	lk := &fakeLinker{} // no identity, email miss, registration closed
	h := &CallbackHandler{
		Slugs:     &fakeSlugs{appID: "app-1", orgID: "org-1"},
		Providers: &fakeProviders{cfg: &cfg},
		State:     state,
		Resolvers: resolverFactoryOf(&fakeResolver{
			identity: entity.Identity{
				Provider: entity.ProviderGoogle, Sub: "new",
				Email: "new@example.com", EmailVerified: true,
			},
		}),
		Linker:     lk,
		Tokens:     &fakeTokens{},
		Dispatcher: &recordingDispatcher{},
	}
	rec := serve(t, http.MethodGet,
		"/a/acme/prod/web/auth/oauth/google/callback?code=C&state="+req.State,
		h)
	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// ------- helper pure tests ----------------------------------------

func TestParseAuthPath(t *testing.T) {
	org, ws, app, p, ok := parseAuthPath("/a/acme/prod/web/auth/oauth/google/authorize", "authorize")
	if !ok || org != "acme" || ws != "prod" || app != "web" || p != "google" {
		t.Errorf("parse failed: %v %v %v %v %v", org, ws, app, p, ok)
	}
	if _, _, _, _, ok := parseAuthPath("/wrong/path", "callback"); ok {
		t.Error("should reject unrelated paths")
	}
}

func TestIsSafeReturnURL(t *testing.T) {
	cases := []struct {
		url  string
		want bool
	}{
		{"", false},
		{"https://app.example/cb", true},
		{"http://localhost:3000", true},
		{"javascript:alert(1)", false},
		{"data:text/html,1", false},
		{"//evil", false},
	}
	for _, c := range cases {
		if got := isSafeReturnURL(c.url); got != c.want {
			t.Errorf("%q: got %v want %v", c.url, got, c.want)
		}
	}
}

func TestFragmentDispatcher_AppendsFragment(t *testing.T) {
	out, err := FragmentDispatcher{}.Redirect("https://app/cb", "AT", "RT")
	if err != nil {
		t.Fatalf("Redirect: %v", err)
	}
	if !strings.Contains(out, "access_token=AT") || !strings.Contains(out, "refresh_token=RT") {
		t.Errorf("fragment missing tokens: %s", out)
	}
}
