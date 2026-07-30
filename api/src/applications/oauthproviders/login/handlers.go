package login

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/a-digi/coco-iam/src/applications/loginpage"
	"github.com/a-digi/coco-iam/src/applications/oauthproviders/adapters"
	"github.com/a-digi/coco-iam/src/applications/oauthproviders/authstate"
	"github.com/a-digi/coco-iam/src/applications/oauthproviders/entity"
	"github.com/a-digi/coco-iam/src/applications/oauthproviders/repository"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// SlugResolver turns a (orgSlug, wsSlug, appSlug) triple into the
// concrete (appID, orgID) pair. Production wiring closes over
// loginpage.Service.Store.FindBySlugs; tests substitute a fake.
type SlugResolver interface {
	ResolveSlugs(org, ws, app string) (appID, orgID string, err error)
}

// ProviderLoader abstracts the repository layer so the callback
// handler can look up the decrypted provider config without
// importing the concrete Repository in tests.
type ProviderLoader interface {
	FindByProvider(applicationID string, p entity.Provider) (*entity.ProviderConfig, error)
}

// ResolverFactory returns the adapter that speaks the IdP's
// wire protocol. Production returns adapters.For; tests inject
// a fake resolver so the handler can be exercised without a
// real IdP.
type ResolverFactory func(p entity.Provider) adapters.IdentityResolver

// CallbackDispatcher builds the final redirect URL the browser
// lands on after a successful OAuth login. Production
// implementation appends the issued access_token + refresh_token
// to the return_url fragment (the `#access_token=…` convention
// client apps already consume). Tests inject a deterministic
// dispatcher to pin the redirect shape.
type CallbackDispatcher interface {
	Redirect(returnURL, accessToken, refreshToken string) (string, error)
}

// LoginSettingsReader returns an application's login-page settings —
// used here only for Settings.RedirectURL, the admin-configured
// destination this application already trusts to receive tokens
// (the same value the password-login flow dispatches to). Reusing it
// as the return_url allowlist means there is no second, separate
// "trusted domain" concept to configure.
type LoginSettingsReader interface {
	LoadSettings(appID string) (loginpage.Settings, error)
}

// AuthorizeHandler mounts
//
//	GET /a/{orgSlug}/{wsSlug}/{appSlug}/auth/oauth/{provider}/authorize?return_url=…
type AuthorizeHandler struct {
	Slugs         SlugResolver
	Providers     ProviderLoader
	State         *authstate.Store
	Resolvers     ResolverFactory
	LoginSettings LoginSettingsReader
	RedirectURI   string // pre-computed callback URL base — see routes.go
}

func (h *AuthorizeHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	orgSlug, wsSlug, appSlug, providerStr, ok := parseAuthPath(r.URL.Path, "authorize")
	if !ok {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid oauth path")
		return
	}
	if !entity.IsAllowedProvider(providerStr) {
		response.ErrorResponse(w, http.StatusBadRequest, "unsupported provider")
		return
	}

	appID, _, err := h.Slugs.ResolveSlugs(orgSlug, wsSlug, appSlug)
	if err != nil {
		response.ErrorResponse(w, http.StatusNotFound, "application not found")
		return
	}

	// return_url must be validated against THIS application's
	// configured redirect target — resolved after appID, not before,
	// since there is no per-app trust boundary to check against until
	// we know which app we're in. A return_url pointing anywhere but
	// the app's own configured redirect_url origin is rejected — see
	// plan/security-return-url-allowlist/plan.md.
	settings, err := h.LoginSettings.LoadSettings(appID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	returnURL := strings.TrimSpace(r.URL.Query().Get("return_url"))
	if !settings.IsConfigured() || !isSafeReturnURL(returnURL, settings.RedirectURL) {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid return_url")
		return
	}

	cfg, err := h.Providers.FindByProvider(appID, entity.Provider(providerStr))
	if err != nil {
		if errors.Is(err, entity.ErrProviderNotFound) {
			response.ErrorResponse(w, http.StatusNotFound, "provider not configured")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !cfg.IsActive || !cfg.AllowLogin {
		response.ErrorResponse(w, http.StatusForbidden, "provider is not enabled")
		return
	}

	req, challenge, err := h.State.StartAuthRequest(authstate.StartInput{
		ApplicationID: appID,
		Provider:      providerStr,
		ReturnURL:     returnURL,
	})
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	resolver := h.Resolvers(entity.Provider(providerStr))
	if resolver == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "no adapter for provider")
		return
	}
	redirectURI := buildRedirectURI(h.RedirectURI, orgSlug, wsSlug, appSlug, providerStr)
	authorizeURL, err := resolver.AuthorizeURL(*cfg, req.State, challenge, redirectURI)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	http.Redirect(w, r, authorizeURL, http.StatusFound)
}

// CallbackHandler mounts
//
//	GET /a/{orgSlug}/{wsSlug}/{appSlug}/auth/oauth/{provider}/callback?code=…&state=…
type CallbackHandler struct {
	Slugs       SlugResolver
	Providers   ProviderLoader
	State       *authstate.Store
	Resolvers   ResolverFactory
	Linker      UserLinker
	Tokens      TokenIssuer
	Dispatcher  CallbackDispatcher
	RedirectURI string
}

func (h *CallbackHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	orgSlug, wsSlug, appSlug, providerStr, ok := parseAuthPath(r.URL.Path, "callback")
	if !ok {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid oauth path")
		return
	}
	if !entity.IsAllowedProvider(providerStr) {
		response.ErrorResponse(w, http.StatusBadRequest, "unsupported provider")
		return
	}
	q := r.URL.Query()
	if errParam := q.Get("error"); errParam != "" {
		response.ErrorResponse(w, http.StatusUnauthorized, "oauth provider returned error: "+errParam)
		return
	}
	code, state := q.Get("code"), q.Get("state")
	if code == "" || state == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "missing code or state")
		return
	}

	authReq, err := h.State.ConsumeAuthRequest(state)
	if err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "authorization request expired")
		return
	}
	if authReq.Provider != providerStr {
		response.ErrorResponse(w, http.StatusBadRequest, "state/provider mismatch")
		return
	}

	appID, orgID, err := h.Slugs.ResolveSlugs(orgSlug, wsSlug, appSlug)
	if err != nil {
		response.ErrorResponse(w, http.StatusNotFound, "application not found")
		return
	}
	if authReq.ApplicationID != appID {
		response.ErrorResponse(w, http.StatusBadRequest, "state/application mismatch")
		return
	}

	cfg, err := h.Providers.FindByProvider(appID, entity.Provider(providerStr))
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	resolver := h.Resolvers(entity.Provider(providerStr))
	if resolver == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "no adapter for provider")
		return
	}

	redirectURI := buildRedirectURI(h.RedirectURI, orgSlug, wsSlug, appSlug, providerStr)
	ctx := r.Context()
	accessAt, idToken, err := resolver.ExchangeCode(ctx, *cfg, code, authReq.CodeVerifier, redirectURI)
	if err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "token exchange failed")
		return
	}
	identity, err := resolver.FetchIdentity(ctx, *cfg, accessAt, idToken)
	if err != nil {
		response.ErrorResponse(w, http.StatusBadGateway, "userinfo fetch failed")
		return
	}
	// The adapter must stamp Provider, but belt-and-braces.
	identity.Provider = entity.Provider(providerStr)

	// The app-level registration flag would need a DB read; for
	// this first cut we trust the provider-level flag to veto
	// appropriately. Wiring the app-level flag is a follow-up.
	userID, _, err := ResolveLogin(identity, *cfg, AppSettings{
		ApplicationID:     appID,
		OrganizationID:    orgID,
		AllowRegistration: cfg.AllowRegistration,
	}, h.Linker)
	if err != nil {
		response.ErrorResponse(w, http.StatusForbidden, err.Error())
		return
	}

	access, refresh, err := h.Tokens.IssueLoginTokens(ctx, appID, userID, nil, nil)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "token issue failed")
		return
	}
	dispatchURL, err := h.Dispatcher.Redirect(authReq.ReturnURL, access, refresh)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	http.Redirect(w, r, dispatchURL, http.StatusFound)
}

// -- helpers -------------------------------------------------------

// parseAuthPath extracts (org, ws, app, provider) for a URL of
// the shape
//
//	/a/{org}/{ws}/{app}/auth/oauth/{provider}/<tail>
//
// tail is either "authorize" or "callback". Returns ok=false
// when the shape doesn't match so handlers can 400 cleanly.
func parseAuthPath(path, tail string) (org, ws, app, provider string, ok bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 8 {
		return "", "", "", "", false
	}
	if parts[0] != "a" || parts[4] != "auth" || parts[5] != "oauth" || parts[7] != tail {
		return "", "", "", "", false
	}
	return parts[1], parts[2], parts[3], parts[6], true
}

// buildRedirectURI assembles the callback URL our service
// expects from the IdP. Base is the scheme+host part
// configured at startup; we append the slug-routed path.
func buildRedirectURI(base, org, ws, app, provider string) string {
	return strings.TrimRight(base, "/") + fmt.Sprintf(
		"/a/%s/%s/%s/auth/oauth/%s/callback",
		org, ws, app, provider,
	)
}

// isSafeReturnURL refuses javascript:/data:/etc return URLs — only
// http(s) destinations are considered at all — AND requires
// returnURL's origin (scheme+host, case-insensitive) to exactly
// match allowedRedirectURL's origin, the application's own
// admin-configured token-dispatch destination. Path/query on
// returnURL are unconstrained — the security boundary is which
// domain receives the token fragment, not which page on that
// domain. See plan/security-return-url-allowlist/plan.md: this
// closes a token-exfiltration open redirect where returnURL could
// previously point at any http(s) host.
func isSafeReturnURL(s, allowedRedirectURL string) bool {
	if s == "" {
		return false
	}
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		if u.Host == "" {
			return false
		}
	default:
		return false
	}

	allowed, err := url.Parse(allowedRedirectURL)
	if err != nil || allowed.Host == "" {
		return false
	}
	return strings.EqualFold(u.Scheme, allowed.Scheme) && strings.EqualFold(u.Host, allowed.Host)
}

// FragmentDispatcher is the production CallbackDispatcher that
// appends tokens to the return_url fragment so the SPA can
// consume them client-side. The server-to-server dispatch the
// password flow uses is a follow-up; for now the SPA handles
// the tokens the same way it already does for the
// implicit-style social login buttons most frameworks render.
type FragmentDispatcher struct{}

func (FragmentDispatcher) Redirect(returnURL, access, refresh string) (string, error) {
	u, err := url.Parse(returnURL)
	if err != nil {
		return "", fmt.Errorf("return_url parse: %w", err)
	}
	frag := url.Values{}
	frag.Set("access_token", access)
	frag.Set("refresh_token", refresh)
	frag.Set("token_type", "Bearer")
	u.Fragment = frag.Encode()
	return u.String(), nil
}

// ResolverFactoryDefault returns the production resolver for
// the given provider via the adapters package. Handlers can
// pass this directly as their ResolverFactory.
func ResolverFactoryDefault(p entity.Provider) adapters.IdentityResolver {
	return adapters.For(p)
}

// Compile-time check: repository.Repository satisfies ProviderLoader.
var _ ProviderLoader = (*repository.Repository)(nil)

// Ensure context import stays live if a future reviewer strips
// unused imports — the callback uses ctx through resolver calls.
var _ = context.Background
