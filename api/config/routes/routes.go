package routes

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/a-digi/coco-iam/config"
	"github.com/a-digi/coco-iam/config/di"
	"github.com/a-digi/coco-iam/config/resource"
	activation_admin "github.com/a-digi/coco-iam/src/activation/admin"
	admin_acl "github.com/a-digi/coco-iam/src/admin/acl"
	dashboard_failedtasks "github.com/a-digi/coco-iam/src/admin/dashboard/failedtasks"
	dashboard_queue "github.com/a-digi/coco-iam/src/admin/dashboard/queue"
	dashboard_recentusers "github.com/a-digi/coco-iam/src/admin/dashboard/recentusers"
	dashboard_registrations "github.com/a-digi/coco-iam/src/admin/dashboard/registrations"
	dashboard_stats "github.com/a-digi/coco-iam/src/admin/dashboard/stats"
	dashboard_toporgs "github.com/a-digi/coco-iam/src/admin/dashboard/toporgs"
	mail_admin "github.com/a-digi/coco-iam/src/admin/mail"
	mail_accounts_admin "github.com/a-digi/coco-iam/src/admin/mail/accounts"
	mail_settings_admin "github.com/a-digi/coco-iam/src/admin/mail/settings"
	mail_templates_admin "github.com/a-digi/coco-iam/src/admin/mail/templates"
	admin_mfa "github.com/a-digi/coco-iam/src/admin/mfa/handler"
	admin_security_archives "github.com/a-digi/coco-iam/src/admin/security/archives/handler"
	admin_security_attacks "github.com/a-digi/coco-iam/src/admin/security/attacks/handler"
	admin_security "github.com/a-digi/coco-iam/src/admin/security/handler"
	admin_security_loginlog "github.com/a-digi/coco-iam/src/admin/security/loginlog/handler"
	admin_security_scans "github.com/a-digi/coco-iam/src/admin/security/scans/handler"
	admin_users "github.com/a-digi/coco-iam/src/admin/users"
	admin_login "github.com/a-digi/coco-iam/src/admin/users/authentication"
	"github.com/a-digi/coco-iam/src/admin/users/me"
	admin_avatar "github.com/a-digi/coco-iam/src/admin/users/profile/avatar"
	workspace_stats "github.com/a-digi/coco-iam/src/admin/workspaces/stats"
	applications_admin "github.com/a-digi/coco-iam/src/applications/admin"
	analytics_handler "github.com/a-digi/coco-iam/src/applications/analytics/handler"
	apicred_admin "github.com/a-digi/coco-iam/src/applications/apicredentials/admin"
	apicred_public "github.com/a-digi/coco-iam/src/applications/apicredentials/public"
	app_authn "github.com/a-digi/coco-iam/src/applications/authentication"
	app_keys "github.com/a-digi/coco-iam/src/applications/keys"
	app_keys_handler "github.com/a-digi/coco-iam/src/applications/keys/handler"
	app_loginlog "github.com/a-digi/coco-iam/src/applications/loginlog/handler"
	app_loginpage "github.com/a-digi/coco-iam/src/applications/loginpage"
	app_loginpage_handler "github.com/a-digi/coco-iam/src/applications/loginpage/handler"
	app_media_handler "github.com/a-digi/coco-iam/src/applications/media/handler"
	oauthproviders_admin "github.com/a-digi/coco-iam/src/applications/oauthproviders/admin"
	oauth_authstate "github.com/a-digi/coco-iam/src/applications/oauthproviders/authstate"
	oauth_login "github.com/a-digi/coco-iam/src/applications/oauthproviders/login"
	oauth_repo "github.com/a-digi/coco-iam/src/applications/oauthproviders/repository"
	oauthserverwiring "github.com/a-digi/coco-iam/src/applications/oauthserverwiring"
	oauthserver_admin "github.com/a-digi/coco-iam/src/applications/oauthserverwiring/admin"
	oauthserver_consents "github.com/a-digi/coco-iam/src/applications/oauthserverwiring/consents"
	profilefields "github.com/a-digi/coco-iam/src/applications/profilefields"
	publicapi_handler "github.com/a-digi/coco-iam/src/applications/publicapi/handler"
	app_recoverypage_handler "github.com/a-digi/coco-iam/src/applications/recoverypage/handler"
	regfields_admin "github.com/a-digi/coco-iam/src/applications/registrationfields/admin"
	regfields_public "github.com/a-digi/coco-iam/src/applications/registrationfields/public"
	regfields_submit "github.com/a-digi/coco-iam/src/applications/registrationfields/submit"
	app_slugmedia "github.com/a-digi/coco-iam/src/applications/slugmedia"
	userprofile "github.com/a-digi/coco-iam/src/applications/userprofile"
	renew_handler "github.com/a-digi/coco-iam/src/auth/oauth/renew"
	password_handler "github.com/a-digi/coco-iam/src/auth/password/handler"
	recovery_handler "github.com/a-digi/coco-iam/src/auth/recovery/handler"
	general_admin "github.com/a-digi/coco-iam/src/general/admin"
	oauthserver "github.com/a-digi/coco-iam/src/oauthserver"
	oauth_dbregistry "github.com/a-digi/coco-iam/src/oauthserver/dbregistry"
	oauth_sqlstore "github.com/a-digi/coco-iam/src/oauthserver/sqlstore"
	profile_dbregistry_main "github.com/a-digi/coco-iam/src/organizations/profile/dbregistry"
	profile_handler "github.com/a-digi/coco-iam/src/organizations/profile/handler"
	organization_users_admin "github.com/a-digi/coco-iam/src/organizations/users/admin"
	users_dbregistry "github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-iam/src/orgrouter"
	"github.com/a-digi/coco-iam/src/security/geoip"
	geoip_handler "github.com/a-digi/coco-iam/src/security/geoip/handler"
	"github.com/a-digi/coco-iam/src/security/ipguard"
	ipsearch_handler "github.com/a-digi/coco-iam/src/security/ipsearch/handler"
	loginbans_handler "github.com/a-digi/coco-iam/src/security/loginbans/handler"
	swagger_handler "github.com/a-digi/coco-iam/src/swagger"
	userrules_handler "github.com/a-digi/coco-iam/src/userrules/handler"
	lift_routes "github.com/a-digi/coco-lift/routes"
	security "github.com/a-digi/coco-lift/security"
	oauth_lib "github.com/a-digi/coco-oauth/oauth"
	observe_agg "github.com/a-digi/coco-observe/aggregator"
	queue_admin "github.com/a-digi/coco-queue/admin"
	serverdi "github.com/a-digi/coco-server/server/di"
	"github.com/a-digi/coco-server/server/fileserver"
	app_media "github.com/a-digi/coco-server/server/media"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
	"github.com/a-digi/coco-server/server/routing"
)

type RootHandler struct{}

// @Summary     Health check
// @Description Returns a plain-text confirmation that the server is running.
// @Tags        system
// @Produce     plain
// @Success     200 "Server is running!"
// @Router      / [get]
func (h *RootHandler) ServeHTTP(reqCtx request.RequestContext) {
	reqCtx.GetWriter().Write([]byte("Server is running!"))
}

// bagGetter is the narrow slice of the DI context that carries
// application-registered services (loginpage.Service, keys.Service,
// …). ctx satisfies this via its underlying ContextBag type; we
// type-assert at Init time so we can hand concrete services to
// handlers that prefer dependency injection over per-request
// service resolution.
type bagGetter interface {
	Get(key string) (interface{}, bool)
}

// profileMeDeps bundles the DI-resolved services the /profile/me
// family of handlers shares. Resolved once in Init so every builder
// (GET, PATCH, file upload/delete/serve) gets the same concrete
// services without repeating the DI dance per builder.
type profileMeDeps struct {
	slugs      userprofile.SlugResolver
	keys       userprofile.KeyLoader
	users      userprofile.UserOrgReader
	profiles   userprofile.ProfileReader
	fields     userprofile.FieldConfigReader
	writer     userprofile.ProfileWriter
	fileRepo   userprofile.FileRepo
	store      userprofile.FileStore
	scanner    userprofile.Scanner
	publicBase string
}

// resolveProfileMeDeps walks the DI bag once, returning a fully-
// populated profileMeDeps. When a required service is missing we
// log + return ready=false so the builders can still produce
// (broken) handlers rather than panicking during Init.
func resolveProfileMeDeps(ctx serverdi.Context) profileMeDeps {
	bag, ok := ctx.(bagGetter)
	if !ok {
		if log := ctx.GetLogger(); log != nil {
			log.Error("routes: /profile/me — DI context is not a bagGetter; endpoints will 500")
		}
		return profileMeDeps{}
	}
	loginRaw, _ := bag.Get(app_loginpage.ContextBagKeyService)
	keysRaw, _ := bag.Get(app_keys.ContextBagKeyService)
	regRaw, _ := bag.Get(profile_dbregistry_main.ContextBagKey)
	loginSvc, _ := loginRaw.(*app_loginpage.Service)
	keysSvc, _ := keysRaw.(*app_keys.Service)
	profileReg, _ := regRaw.(*profile_dbregistry_main.OrgDBRegistry)
	orgUsersRegRaw, _ := bag.Get(users_dbregistry.ContextBagKey)
	orgUsersReg, _ := orgUsersRegRaw.(*users_dbregistry.OrgUserDBRegistry)
	pb := osGetenv("COCO_IAM_PUBLIC_BASE_URL")
	if pb == "" {
		pb = "http://localhost:2026"
	}
	return profileMeDeps{
		slugs:      userprofile.NewLoginpageSlugResolver(loginSvc),
		keys:       userprofile.NewKeysServiceKeyLoader(keysSvc),
		users:      userprofile.NewOrgRegistryUserOrgReader(orgUsersReg),
		profiles:   userprofile.NewOrgRegistryProfileReader(profileReg),
		fields:     userprofile.NewOrgFieldConfigReader(profileReg),
		writer:     userprofile.NewOrgProfileWriter(profileReg),
		fileRepo:   userprofile.NewOrgFileRepo(profileReg),
		store:      userprofile.NewPerOrgUserFileStore("./data/db"),
		scanner:    userprofile.NewMediaScanner(),
		publicBase: pb,
	}
}

// buildOAuthLoginHandlers constructs the AuthorizeHandler and
// CallbackHandler for the external-IdP login flow. Shares one
// state store + slug resolver + provider loader across both.
// Returns a pair so the caller can wire both into the handlerMap
// with a single resolve pass.
func buildOAuthLoginHandlers(ctx serverdi.Context) (routing.HandlerInterface, routing.HandlerInterface) {
	bag, ok := ctx.(bagGetter)
	if !ok {
		return &oauth_login.AuthorizeHandler{}, &oauth_login.CallbackHandler{}
	}
	loginRaw, _ := bag.Get(app_loginpage.ContextBagKeyService)
	keysRaw, _ := bag.Get(app_keys.ContextBagKeyService)
	usersRegRaw, _ := bag.Get(users_dbregistry.ContextBagKey)
	loginSvc, _ := loginRaw.(*app_loginpage.Service)
	keysSvc, _ := keysRaw.(*app_keys.Service)
	usersReg, _ := usersRegRaw.(*users_dbregistry.OrgUserDBRegistry)
	manager := ctx.GetDatabaseManager()
	var mainDB *sql.DB
	if manager != nil && manager.Connector != nil {
		mainDB = manager.Connector.DB
	}

	oauthRegRaw2, _ := bag.Get(oauth_dbregistry.ContextBagKey)
	oauthReg2, _ := oauthRegRaw2.(*oauth_dbregistry.OrgOAuthDBRegistry)

	slugs := loginpageSlugAdapter{svc: loginSvc}
	// Providers now live in the per-org DB. Build a resolver that
	// scans per-org DBs to find the one that owns the application.
	providers := oauth_repo.NewWithResolver(func(appID string) (*sql.DB, error) {
		if usersReg == nil {
			return nil, sql.ErrConnDone
		}
		orgDB, _, err := orgrouter.OrgDBForApp(usersReg, appID)
		if err != nil {
			return nil, err
		}
		return orgDB, nil
	})
	// Auth-state rows live in the per-org oauth.db. Build a resolver
	// that maps org ID to oauth.db via the registry.
	loginOAuthResolver := func(orgID string) (*sql.DB, error) {
		if oauthReg2 == nil {
			return nil, sql.ErrConnDone
		}
		mgr, err := oauthReg2.For(orgID)
		if err != nil || mgr == nil || mgr.Connector == nil {
			return nil, sql.ErrConnDone
		}
		return mgr.Connector.DB, nil
	}
	oauthKnownOrgIDsLogin := func() []string {
		if oauthReg2 == nil {
			return nil
		}
		return oauthReg2.KnownOrgIDs()
	}
	stateBase := oauth_authstate.New(mainDB, loginOAuthResolver, 0, nil)
	state := stateBase.
		WithAppResolver(func(appID string) (string, error) {
			if usersReg == nil {
				return "", sql.ErrConnDone
			}
			_, orgID, err := orgrouter.OrgDBForApp(usersReg, appID)
			return orgID, err
		}).
		WithKnownOrgIDs(oauthKnownOrgIDsLogin)
	linker := oauth_login.NewSQLLinker(usersReg)

	cfgBytes, _ := config.ReadConfigFile("config.json")
	authCfg, _ := oauth_lib.LoadAuthConfigFromBytes(cfgBytes)
	tokens := &oauth_login.AppTokenIssuer{Keys: keysSvc, Cfg: authCfg}

	// Public base URL: where the IdP should post the user back.
	// Defaults to localhost:<port> from config.json for dev.
	publicBase := osGetenv("COCO_IAM_PUBLIC_BASE_URL")
	if publicBase == "" {
		publicBase = "http://localhost:2026"
	}

	authorize := &oauth_login.AuthorizeHandler{
		Slugs:         slugs,
		Providers:     providers,
		State:         state,
		Resolvers:     oauth_login.ResolverFactoryDefault,
		LoginSettings: loginSvc,
		RedirectURI:   publicBase,
	}
	callback := &oauth_login.CallbackHandler{
		Slugs:       slugs,
		Providers:   providers,
		State:       state,
		Resolvers:   oauth_login.ResolverFactoryDefault,
		Linker:      linker,
		Tokens:      tokens,
		Dispatcher:  oauth_login.FragmentDispatcher{},
		RedirectURI: publicBase,
	}
	return authorize, callback
}

// loginpageSlugAdapter wraps loginpage.Service.Store.FindBySlugs
// as the SlugResolver the OAuth handlers expect. One line of
// plumbing — no business logic.
type loginpageSlugAdapter struct {
	svc *app_loginpage.Service
}

func (a loginpageSlugAdapter) ResolveSlugs(org, ws, app string) (string, string, error) {
	if a.svc == nil {
		return "", "", sql.ErrConnDone
	}
	info, err := a.svc.Store.FindBySlugs(org, ws, app)
	if err != nil {
		return "", "", err
	}
	return info.ID, info.OrganizationID, nil
}

// osGetenv is re-declared here to keep the import list stable
// when this file is routinely touched; it just wraps os.Getenv.
var osGetenv = os.Getenv

// oauthServerEndpoints bundles the live OAuth provider handlers
// that get registered into the slug-routed YAML. Built once in
// Init from the DI bag + config.json. Per-request state in the
// TokenHandler (lastMintedRefreshID) means we wrap it in a
// per-request cloning helper that the routing framework calls.
type oauthServerEndpoints struct {
	authorize  *oauthserver.AuthorizeHandler
	tokenProto *oauthserver.TokenHandler
	userinfo   *oauthserver.UserinfoHandler
	revoke     *oauthserver.RevokeHandler
	introspect *oauthserver.IntrospectHandler
	discovery  *oauthserver.DiscoveryHandler
	codes      oauthserver.CodeStore
	clients    oauthserver.ClientRegistry
}

// buildOAuthServerEndpoints constructs the handler set the
// OAuth provider exposes at /a/{slugs}/oauth/... and the
// matching .well-known endpoint. Returns a fully-wired struct
// ready to register in the route map.
func buildOAuthServerEndpoints(ctx serverdi.Context) *oauthServerEndpoints {
	bag, ok := ctx.(bagGetter)
	if !ok {
		return nil
	}
	loginRaw, _ := bag.Get(app_loginpage.ContextBagKeyService)
	keysRaw, _ := bag.Get(app_keys.ContextBagKeyService)
	usersRegRaw, _ := bag.Get(users_dbregistry.ContextBagKey)
	oauthRegRaw, _ := bag.Get(oauth_dbregistry.ContextBagKey)
	loginSvc, _ := loginRaw.(*app_loginpage.Service)
	keysSvc, _ := keysRaw.(*app_keys.Service)
	usersReg, _ := usersRegRaw.(*users_dbregistry.OrgUserDBRegistry)
	oauthReg, _ := oauthRegRaw.(*oauth_dbregistry.OrgOAuthDBRegistry)
	manager := ctx.GetDatabaseManager()
	var mainDB *sql.DB
	if manager != nil && manager.Connector != nil {
		mainDB = manager.Connector.DB
	}
	if loginSvc == nil || keysSvc == nil || usersReg == nil || mainDB == nil {
		return nil
	}

	cfgBytes, _ := config.ReadConfigFile("config.json")
	authCfg, _ := oauth_lib.LoadAuthConfigFromBytes(cfgBytes)

	publicBase := osGetenv("COCO_IAM_PUBLIC_BASE_URL")
	if publicBase == "" {
		publicBase = "http://localhost:2026"
	}

	hasher := oauthserverwiring.NewBcryptHasher(0)
	// application_oauth_clients now lives in the per-org users DB.
	// oauth_authorization_codes and oauth_refresh_tokens now live in
	// the per-org oauth.db (OrgOAuthDBRegistry). Codes and refresh
	// tokens use the oauth registry resolver; clients use the users
	// registry resolver (scanning per-org DBs via OrgDBForApp).
	appOrgResolver := func(appID string) (*sql.DB, error) {
		if usersReg == nil {
			return nil, sql.ErrConnDone
		}
		orgDB, _, err := orgrouter.OrgDBForApp(usersReg, appID)
		if err != nil {
			return nil, err
		}
		return orgDB, nil
	}
	appOrgIDResolver := func(appID string) (string, error) {
		if usersReg == nil {
			return "", sql.ErrConnDone
		}
		_, orgID, err := orgrouter.OrgDBForApp(usersReg, appID)
		return orgID, err
	}

	// oauthOrgResolver opens the per-org oauth.db for a given org ID.
	// Falls back to the users DB if the oauth registry is not wired.
	oauthOrgResolver := func(orgID string) (*sql.DB, error) {
		if oauthReg != nil {
			mgr, err := oauthReg.For(orgID)
			if err != nil || mgr == nil || mgr.Connector == nil {
				return nil, sql.ErrConnDone
			}
			return mgr.Connector.DB, nil
		}
		// Fallback: use the users DB (legacy behaviour when registry
		// not yet provisioned).
		if usersReg == nil {
			return nil, sql.ErrConnDone
		}
		mgr, err := usersReg.For(orgID)
		if err != nil || mgr == nil || mgr.Connector == nil {
			return nil, sql.ErrConnDone
		}
		return mgr.Connector.DB, nil
	}

	oauthKnownOrgIDs := func() []string {
		if oauthReg == nil {
			return nil
		}
		return oauthReg.KnownOrgIDs()
	}

	clients := oauth_sqlstore.NewClientRepoWithResolver(appOrgResolver, hasher)
	codes := oauth_sqlstore.NewCodeRepoWithOrgResolver(oauthOrgResolver, oauthKnownOrgIDs, appOrgIDResolver)
	refresh := oauth_sqlstore.NewRefreshRepoWithOrgResolver(oauthOrgResolver, oauthKnownOrgIDs, appOrgIDResolver)
	consents := oauth_sqlstore.NewConsentRepo(func(orgID string) (*sql.DB, error) {
		mgr, err := usersReg.For(orgID)
		if err != nil {
			return nil, err
		}
		if mgr == nil || mgr.Connector == nil || mgr.Connector.DB == nil {
			return nil, sql.ErrConnDone
		}
		return mgr.Connector.DB, nil
	})

	signer := oauthserverwiring.NewKeysSigner(keysSvc)
	verifier := oauthserverwiring.NewKeysVerifier(keysSvc)
	claimsReader := &oauthserverwiring.UsersDBClaimsReader{
		Registry: usersReg,
		Resolver: func(orgID string) (*sql.DB, error) {
			mgr, err := usersReg.For(orgID)
			if err != nil {
				return nil, err
			}
			if mgr == nil || mgr.Connector == nil || mgr.Connector.DB == nil {
				return nil, sql.ErrConnDone
			}
			return mgr.Connector.DB, nil
		},
	}

	sessionStore, _ := oauthserverwiring.NewSessionStore(authCfg.HS256Secret)
	auth := &oauthserverwiring.SessionAuthenticator{Store: sessionStore}

	routing := oauthserverwiring.NewSlugRouting(loginSvc, publicBase, "")
	appResolver := routing.ApplicationIDFromRequest
	issuerFn := routing.IssuerFromRequest
	loginRedirect := routing.LoginRedirectURL

	authorize := &oauthserver.AuthorizeHandler{
		ApplicationIDFromRequest: appResolver,
		Clients:                  clients,
		Codes:                    codes,
		Consents:                 consents,
		Auth:                     auth,
		LoginRedirectURL:         loginRedirect,
		ScopeEnricher: func(_ context.Context, appID, userID string, granted []string) []string {
			orgDB, err := appOrgResolver(appID)
			if err != nil {
				return granted
			}
			extra := app_authn.LoadAllUserScopes(orgDB, appID, userID)
			if len(extra) == 0 {
				return granted
			}
			existing := make(map[string]struct{}, len(granted))
			for _, s := range granted {
				existing[s] = struct{}{}
			}
			for _, r := range extra {
				if _, ok := existing[r]; !ok {
					granted = append(granted, r)
				}
			}
			return granted
		},
	}
	tokenProto := &oauthserver.TokenHandler{
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
		IssuerFromRequest:   func(r *http.Request) string { return publicBase },
		BasePathFromRequest: routing.BasePathFromRequest,
		ScopesSupported:     []string{"openid", "profile", "email", "offline_access"},
	}
	return &oauthServerEndpoints{
		authorize:  authorize,
		tokenProto: tokenProto,
		userinfo:   userinfo,
		revoke:     revoke,
		introspect: introspect,
		discovery:  discovery,
		codes:      codes,
		clients:    clients,
	}
}

// oauthShim wraps a stdlib http.Handler-style ServeHTTP into
// the request.RequestContext-style ServeHTTP the framework
// expects. The OAuth library handlers are framework-agnostic
// (they take *http.Request + http.ResponseWriter); this is the
// 5-line bridge.
type oauthShim struct {
	fn func(w http.ResponseWriter, r *http.Request)
}

func (s oauthShim) ServeHTTP(reqCtx request.RequestContext) {
	s.fn(reqCtx.GetWriter(), reqCtx.GetRequest())
}

// tokenShim ensures every /token request gets a fresh
// TokenHandler instance — the library's lastMintedRefreshID
// field is per-request state.
type tokenShim struct {
	proto *oauthserver.TokenHandler
}

func (s tokenShim) ServeHTTP(reqCtx request.RequestContext) {
	if s.proto == nil {
		http.Error(reqCtx.GetWriter(), `{"error":"server_error","error_description":"oauth server not configured"}`, http.StatusInternalServerError)
		return
	}
	clone := *s.proto
	clone.ServeHTTP(reqCtx.GetWriter(), reqCtx.GetRequest())
}

// buildConsentHandlers wires the user-facing connected-apps
// list + revoke endpoints. Reuses the profileMe deps for slug
// resolution + bearer auth so the surface follows the same
// auth model as /profile/me.
func buildConsentHandlers(ctx serverdi.Context, deps profileMeDeps) (routing.HandlerInterface, routing.HandlerInterface) {
	bag, ok := ctx.(bagGetter)
	if !ok {
		return &oauthserver_consents.ListHandler{}, &oauthserver_consents.RevokeHandler{}
	}
	usersRegRaw, _ := bag.Get(users_dbregistry.ContextBagKey)
	usersReg, _ := usersRegRaw.(*users_dbregistry.OrgUserDBRegistry)
	manager := ctx.GetDatabaseManager()
	var mainDB *sql.DB
	if manager != nil && manager.Connector != nil {
		mainDB = manager.Connector.DB
	}
	usersDBResolver := func(orgID string) (*sql.DB, error) {
		if usersReg == nil {
			return nil, sql.ErrConnDone
		}
		mgr, err := usersReg.For(orgID)
		if err != nil {
			return nil, err
		}
		if mgr == nil || mgr.Connector == nil || mgr.Connector.DB == nil {
			return nil, sql.ErrConnDone
		}
		return mgr.Connector.DB, nil
	}
	consentDeps := oauthserver_consents.Deps{
		Slugs:   deps.slugs,
		Keys:    deps.keys,
		Users:   deps.users,
		UsersDB: usersDBResolver,
		MainDB:  mainDB,
	}
	return &oauthserver_consents.ListHandler{Deps: consentDeps},
		&oauthserver_consents.RevokeHandler{Deps: consentDeps}
}

// stdShim adapts a stdlib http.Handler to the framework's routing.HandlerInterface.
// It injects the same CORS headers every other handler gets via response.BuildHeaders,
// because the aggregator's writeJSON/jsonError helpers do not add them.
type stdShim struct{ h http.Handler }

func (s stdShim) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, PATCH")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, Accept")
	response.SetBaselineSecurityHeaders(w)
	s.h.ServeHTTP(w, reqCtx.GetRequest())
}

const observeAgentsBasePath = "/api/v1/admin/observe/agents"

// validateAggregatorURL requires an absolute http(s) URL with a host.
// Duplicated (not imported) from plugins/coco-observe/agent's own
// identical validator — that package belongs to a separate Go
// module this one doesn't otherwise depend on, and this is the
// established convention in this codebase for small validation
// helpers rather than adding a cross-module dependency for one
// function. See plan/todo/security/header-and-cache-poisoning.md.
func validateAggregatorURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("must be a valid URL: %w", err)
	}
	switch u.Scheme {
	case "http", "https":
	default:
		return fmt.Errorf("must use http or https, got %q", u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("must include a host")
	}
	return nil
}

// buildObserveHandlers initialises the coco-observe aggregator and returns
// four framework-compatible handler shims. DataDir defaults to
// ./data/observe and may be overridden by OBSERVE_DATA_DIR.
// The base agent binaries are embedded into this binary at build time;
// per-agent binaries are generated and stored on agent creation.
func buildObserveHandlers() (push, query, agents, download routing.HandlerInterface) {
	dataDir := osGetenv("OBSERVE_DATA_DIR")
	if dataDir == "" {
		dataDir = "./data/observe"
	}
	aggregatorURL := osGetenv("OBSERVE_AGGREGATOR_URL")
	if aggregatorURL == "" {
		publicBase := osGetenv("COCO_IAM_PUBLIC_BASE_URL")
		if publicBase == "" {
			publicBase = "http://localhost:2026"
		}
		aggregatorURL = publicBase + "/api/v1/admin/observe/push"
	} else if err := validateAggregatorURL(aggregatorURL); err != nil {
		// Only the explicitly-set env var needs validating here — the
		// derived default is always well-formed (built from a fixed
		// prefix + a scheme/host already checked at boot, see
		// validatePublicBaseURL in main.go). This only disables the
		// observe agent-download feature, not core auth, so it
		// degrades the same way an aggregator construction failure
		// already does below, rather than crashing the whole server.
		// See plan/todo/security/header-and-cache-poisoning.md.
		broken := stdShim{h: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, `{"error":"invalid OBSERVE_AGGREGATOR_URL: `+err.Error()+`"}`, http.StatusServiceUnavailable)
		})}
		return broken, broken, broken, broken
	}
	agg, err := observe_agg.New(observe_agg.Config{
		DataDir:       dataDir,
		AggregatorURL: aggregatorURL,
	})
	if err != nil {
		broken := stdShim{h: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, `{"error":"observe aggregator unavailable"}`, http.StatusServiceUnavailable)
		})}
		return broken, broken, broken, broken
	}
	return stdShim{agg.PushHandler()},
		stdShim{agg.QueryHandler()},
		stdShim{agg.AgentsHandler(observeAgentsBasePath, observe_agg.EmbeddedAgentAmd64, observe_agg.EmbeddedAgentArm64)},
		stdShim{agg.DownloadHandler()}
}

// safeOAuthShim returns a handler that 500s if endpoints is nil
// (the OAuth server didn't construct), otherwise wraps the
// pick'd handler in oauthShim.
func safeOAuthShim(endpoints *oauthServerEndpoints, pick func(*oauthServerEndpoints) interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}) routing.HandlerInterface {
	if endpoints == nil {
		return oauthShim{fn: func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, `{"error":"server_error","error_description":"oauth server not configured"}`, http.StatusInternalServerError)
		}}
	}
	h := pick(endpoints)
	return oauthShim{fn: h.ServeHTTP}
}

// buildProfileMeHandler pulls the three services `/profile/me`
// needs out of the DI bag and constructs the handler with
// production adapters. Kept inline in Init so the static
// handlerMap declaration stays compile-time simple.
func buildProfileMeHandler(deps profileMeDeps) routing.HandlerInterface {
	return &userprofile.GetMeHandler{
		Slugs:      deps.slugs,
		Keys:       deps.keys,
		Users:      deps.users,
		Profiles:   deps.profiles,
		PublicBase: deps.publicBase,
	}
}

// buildMyLoginLogHandler wires GET /profile/me/login-log — reuses
// the exact same slugs/keys/users collaborators /profile/me itself
// resolves, since both are the same self-service RS256-bearer auth
// flow. See plan/self-service-login-log/plan.md.
func buildMyLoginLogHandler(deps profileMeDeps) routing.HandlerInterface {
	return &app_loginlog.MyLoginLogHandler{
		Slugs: deps.slugs,
		Keys:  deps.keys,
		Users: deps.users,
	}
}

// buildProfileMePatchHandler wires the PATCH /profile/me handler.
// Shares slug resolver + auth + profile reader with the GET variant;
// adds the ProfileWriter for the merge-result apply step.
func buildProfileMePatchHandler(deps profileMeDeps) routing.HandlerInterface {
	return &userprofile.PatchMeHandler{
		Slugs:      deps.slugs,
		Keys:       deps.keys,
		Users:      deps.users,
		Profiles:   deps.profiles,
		Writer:     deps.writer,
		PublicBase: deps.publicBase,
	}
}

// buildProfileMeFileUploadHandler assembles POST .../profile/me/files/{fieldName}.
// Pulls in the scanner + store + file repo on top of the shared
// auth collaborators.
func buildProfileMeFileUploadHandler(deps profileMeDeps) routing.HandlerInterface {
	return &userprofile.FileUploadHandler{
		Slugs:    deps.slugs,
		Keys:     deps.keys,
		Users:    deps.users,
		Fields:   deps.fields,
		Scanner:  deps.scanner,
		Store:    deps.store,
		Files:    deps.fileRepo,
		Writer:   deps.writer,
		Profiles: deps.profiles,
	}
}

// buildProfileMeFileDeleteHandler wires DELETE on a user's file-type field.
func buildProfileMeFileDeleteHandler(deps profileMeDeps) routing.HandlerInterface {
	return &userprofile.FileDeleteHandler{
		Slugs:  deps.slugs,
		Keys:   deps.keys,
		Users:  deps.users,
		Fields: deps.fields,
		Store:  deps.store,
		Files:  deps.fileRepo,
		Writer: deps.writer,
	}
}

// buildProfileMeFileServeHandler wires GET bytes on a user's file-type field.
func buildProfileMeFileServeHandler(deps profileMeDeps) routing.HandlerInterface {
	return &userprofile.FileServeHandler{
		Slugs: deps.slugs,
		Keys:  deps.keys,
		Users: deps.users,
		Store: deps.store,
		Files: deps.fileRepo,
	}
}

// buildProfileFieldsPutHandler wires PUT /a/{org}/{ws}/{app}/profile/fields.
// Resolves all file-handling deps plus the PUT-specific ports (FullFieldLoader,
// ProfileReader, ProfileSaver) from the DI bag.
func buildProfileFieldsPutHandler(ctx serverdi.Context, deps profileMeDeps) routing.HandlerInterface {
	bag, ok := ctx.(bagGetter)
	if !ok {
		return &profilefields.PutProfileFieldsHandler{}
	}
	loginRaw, _ := bag.Get(app_loginpage.ContextBagKeyService)
	keysRaw, _ := bag.Get(app_keys.ContextBagKeyService)
	regRaw, _ := bag.Get(profile_dbregistry_main.ContextBagKey)
	orgUsersRegRaw, _ := bag.Get(users_dbregistry.ContextBagKey)
	loginSvc, _ := loginRaw.(*app_loginpage.Service)
	keysSvc, _ := keysRaw.(*app_keys.Service)
	profileReg, _ := regRaw.(*profile_dbregistry_main.OrgDBRegistry)
	orgUsersReg, _ := orgUsersRegRaw.(*users_dbregistry.OrgUserDBRegistry)
	return &profilefields.PutProfileFieldsHandler{
		Slugs:      profilefields.NewLoginpageSlugResolver(loginSvc),
		Keys:       profilefields.NewKeysServiceKeyLoader(keysSvc),
		Users:      profilefields.NewOrgRegistryUserOrgReader(orgUsersReg),
		FullFields: profilefields.NewOrgFullFieldLoader(profileReg),
		Reader:     profilefields.NewOrgRegistryProfileReader(profileReg),
		Saver:      profilefields.NewOrgProfileSaver(profileReg),
		Scanner:    deps.scanner,
		Store:      deps.store,
		Files:      deps.fileRepo,
		PublicBase: deps.publicBase,
	}
}

// buildProfileFieldsHandler wires GET /a/{org}/{ws}/{app}/profile/fields.
// Resolves its own deps from the DI bag so it stays independent of
// profileMeDeps internals.
func buildProfileFieldsHandler(ctx serverdi.Context) routing.HandlerInterface {
	bag, ok := ctx.(bagGetter)
	if !ok {
		return &profilefields.GetProfileFieldsHandler{}
	}
	loginRaw, _ := bag.Get(app_loginpage.ContextBagKeyService)
	keysRaw, _ := bag.Get(app_keys.ContextBagKeyService)
	regRaw, _ := bag.Get(profile_dbregistry_main.ContextBagKey)
	orgUsersRegRaw, _ := bag.Get(users_dbregistry.ContextBagKey)
	loginSvc, _ := loginRaw.(*app_loginpage.Service)
	keysSvc, _ := keysRaw.(*app_keys.Service)
	profileReg, _ := regRaw.(*profile_dbregistry_main.OrgDBRegistry)
	orgUsersReg, _ := orgUsersRegRaw.(*users_dbregistry.OrgUserDBRegistry)
	return &profilefields.GetProfileFieldsHandler{
		Slugs:  profilefields.NewLoginpageSlugResolver(loginSvc),
		Keys:   profilefields.NewKeysServiceKeyLoader(keysSvc),
		Users:  profilefields.NewOrgRegistryUserOrgReader(orgUsersReg),
		Fields: profilefields.NewOrgFieldSchemaReader(profileReg),
	}
}

func Init(ctx serverdi.Context) {
	routing.GlobalRouteBuilder.AddContext(ctx)
	updatedYamlBytes, _ := lift_routes.LoadRoutesYAML(config.ConfigFS)
	profileMe := resolveProfileMeDeps(ctx)
	oauthAuthorize, oauthCallback := buildOAuthLoginHandlers(ctx)
	oauthSrv := buildOAuthServerEndpoints(ctx)
	consentList, consentRevoke := buildConsentHandlers(ctx, profileMe)
	oauthSrvAuthorize := safeOAuthShim(oauthSrv, func(s *oauthServerEndpoints) interface {
		ServeHTTP(w http.ResponseWriter, r *http.Request)
	} {
		return s.authorize
	})
	oauthSrvUserinfo := safeOAuthShim(oauthSrv, func(s *oauthServerEndpoints) interface {
		ServeHTTP(w http.ResponseWriter, r *http.Request)
	} {
		return s.userinfo
	})
	oauthSrvRevoke := safeOAuthShim(oauthSrv, func(s *oauthServerEndpoints) interface {
		ServeHTTP(w http.ResponseWriter, r *http.Request)
	} {
		return s.revoke
	})
	oauthSrvIntrospect := safeOAuthShim(oauthSrv, func(s *oauthServerEndpoints) interface {
		ServeHTTP(w http.ResponseWriter, r *http.Request)
	} {
		return s.introspect
	})
	oauthSrvDiscovery := safeOAuthShim(oauthSrv, func(s *oauthServerEndpoints) interface {
		ServeHTTP(w http.ResponseWriter, r *http.Request)
	} {
		return s.discovery
	})
	var oauthSrvToken routing.HandlerInterface = tokenShim{}
	if oauthSrv != nil {
		oauthSrvToken = tokenShim{proto: oauthSrv.tokenProto}
	}

	observePush, observeQuery, observeAgents, observeDownload := buildObserveHandlers()

	handlerMap := map[string]routing.HandlerInterface{
		"RootHandler":                        &RootHandler{},
		"AdminDashboardStatsHandler":         &dashboard_stats.AdminDashboardStatsHandler{},
		"AdminDashboardRegistrationsHandler": &dashboard_registrations.AdminDashboardRegistrationsHandler{},
		"AdminDashboardTopOrgsHandler":       &dashboard_toporgs.AdminDashboardTopOrgsHandler{},
		"AdminDashboardQueueHandler":         &dashboard_queue.AdminDashboardQueueHandler{},
		"AdminDashboardRecentUsersHandler":   &dashboard_recentusers.AdminDashboardRecentUsersHandler{},
		"AdminDashboardFailedTasksHandler":   &dashboard_failedtasks.AdminDashboardFailedTasksHandler{},
		"OrgProfileFieldsListHandler":        &profile_handler.OrgProfileFieldsListHandler{},
		"OrgProfileFieldsCreateHandler":      &profile_handler.OrgProfileFieldsCreateHandler{},
		"OrgProfileFieldsUpdateHandler":      &profile_handler.OrgProfileFieldsUpdateHandler{},
		"OrgProfileFieldsDeleteHandler":      &profile_handler.OrgProfileFieldsDeleteHandler{},
		"OrgProfileFieldsReorderHandler":     &profile_handler.OrgProfileFieldsReorderHandler{},
		"OrgUserProfileGetHandler":           &profile_handler.OrgUserProfileGetHandler{},
		"OrgUserProfileUpsertHandler":        &profile_handler.OrgUserProfileUpsertHandler{},
		"OrgUserResendActivationHandler":     &organization_users_admin.ResendOrgUserActivationHandler{},
		"AdminLoginHandler":                  &admin_login.DatabaseAuthenticationLogin{},
		"AclScopesHandler":                   &admin_acl.AclScopesHandler{},
		"TokenRenewHandler":                  &renew_handler.TokenRenewHandler{},
		"MeGroupsHandler":                    &me.MeGroupsHandler{},
		"MeAclHandler":                       &me.MeAclHandler{},
		// Admin-user self profile — see plan/admin-user-profile/plan.md.
		"MeHandler":      &me.MeHandler{},
		"MePatchHandler": &me.MePatchHandler{},
		// Admin me: password notification preferences.
		"MePasswordNotificationGetHandler": &me.MePasswordNotificationGetHandler{},
		"MePasswordNotificationPutHandler": &me.MePasswordNotificationPutHandler{},
		"AdminAvatarUploadHandler":         &admin_avatar.UploadHandler{},
		"AdminAvatarDeleteHandler":         &admin_avatar.DeleteHandler{},
		"AdminAvatarServeHandler":          &admin_avatar.PublicServeHandler{},
		// Admin self-service TOTP MFA — see plan/admin-mfa-totp/plan.md.
		"MfaStatusHandler":                  &admin_mfa.MfaStatusHandler{},
		"MfaEnrollHandler":                  &admin_mfa.MfaEnrollHandler{},
		"MfaConfirmHandler":                 &admin_mfa.MfaConfirmHandler{},
		"MfaDisableHandler":                 &admin_mfa.MfaDisableHandler{},
		"MfaRecoveryCodesRegenerateHandler": &admin_mfa.MfaRecoveryCodesRegenerateHandler{},
		"VerifyMfaHandler":                  &admin_mfa.VerifyMfaHandler{},
		// Admin IP ban/allowlist management — see plan/ip-abuse-protection/plan.md.
		"IPBanListHandler":                 &admin_security.IPBanListHandler{},
		"IPBanCreateHandler":               &admin_security.IPBanCreateHandler{},
		"IPBanDeleteHandler":               &admin_security.IPBanDeleteHandler{},
		"IPBanAccountsHandler":             &admin_security.IPBanAccountsHandler{},
		"IPAllowlistListHandler":           &admin_security.IPAllowlistListHandler{},
		"IPAllowlistCreateHandler":         &admin_security.IPAllowlistCreateHandler{},
		"IPAllowlistDeleteHandler":         &admin_security.IPAllowlistDeleteHandler{},
		"SecurityStatusHandler":            &admin_security.SecurityStatusHandler{},
		"AttackListHandler":                &admin_security_attacks.AttackListHandler{},
		"AttackDetailHandler":              &admin_security_attacks.AttackDetailHandler{},
		"AttackFetchGeoIPHandler":          &admin_security_attacks.FetchGeoIPHandler{},
		"ArchiveListHandler":               &admin_security_archives.ArchiveListHandler{},
		"ArchiveDetailHandler":             &admin_security_archives.ArchiveDetailHandler{},
		"ArchiveAttacksListHandler":        &admin_security_archives.ArchiveAttacksListHandler{},
		"ArchiveAttackDetailHandler":       &admin_security_archives.ArchiveAttackDetailHandler{},
		"ScanListHandler":                  &admin_security_scans.ScanListHandler{},
		"ScanDetailHandler":                &admin_security_scans.ScanDetailHandler{},
		"AdminLoginListHandler":            &admin_security_loginlog.AdminLoginListHandler{},
		"AdminLoginArchiveListHandler":     &admin_security_loginlog.AdminLoginArchiveListHandler{},
		"AdminLoginArchiveAttemptsHandler": &admin_security_loginlog.AdminLoginArchiveAttemptsHandler{},
		// Failed-login ban-rule settings — see plan/login-ban-rules/plan.md.
		"LoginBansGetSettingsHandler": &loginbans_handler.GetSettingsHandler{},
		"LoginBansPutSettingsHandler": &loginbans_handler.PutSettingsHandler{},
		// Admin GeoIP settings + process control — see
		// plan/geoip-enrichment/plan.md.
		"GeoIPGetSettingsHandler":              &geoip_handler.GetSettingsHandler{},
		"GeoIPPutSettingsHandler":              &geoip_handler.PutSettingsHandler{},
		"GeoIPStatusHandler":                   &geoip_handler.StatusHandler{},
		"GeoIPStartHandler":                    &geoip_handler.StartHandler{},
		"GeoIPStopHandler":                     &geoip_handler.StopHandler{},
		"GeoIPSyncHandler":                     &geoip_handler.SyncHandler{},
		"IPSearchHandler":                      &ipsearch_handler.SearchHandler{},
		"AdminQueueStatsHandler":               &queue_admin.AdminQueueStatsHandler{},
		"AdminQueueRetryHandler":               &queue_admin.AdminQueueRetryHandler{},
		"AdminQueueCreateHandler":              &queue_admin.AdminQueueCreateHandler{},
		"AdminQueueTaskPayloadHandler":         &queue_admin.AdminQueueTaskPayloadHandler{},
		"AdminQueueTasksListHandler":           &queue_admin.AdminQueueTasksListHandler{},
		"AdminQueueTaskGetHandler":             &queue_admin.AdminQueueTaskGetHandler{},
		"WorkspaceStatsHandler":                &workspace_stats.WorkspaceStatsHandler{},
		"ApplicationScopesExportHandler":       &applications_admin.ApplicationScopesExportHandler{},
		"ApplicationScopesImportHandler":       &applications_admin.ApplicationScopesImportHandler{},
		"AdminMailTestHandler":                 &mail_admin.AdminMailTestHandler{},
		"AdminMailListHandler":                 &mail_admin.AdminMailListHandler{},
		"AdminMailDetailHandler":               &mail_admin.AdminMailDetailHandler{},
		"AdminMailRetryHandler":                &mail_admin.AdminMailRetryHandler{},
		"AdminMailStatusHandler":               &mail_admin.AdminMailStatusHandler{},
		"AdminMailTemplatesListHandler":        &mail_templates_admin.AdminMailTemplatesListHandler{},
		"AdminMailTemplatesGetHandler":         &mail_templates_admin.AdminMailTemplatesGetHandler{},
		"AdminMailTemplatesCreateHandler":      &mail_templates_admin.AdminMailTemplatesCreateHandler{},
		"AdminMailTemplatesUpdateHandler":      &mail_templates_admin.AdminMailTemplatesUpdateHandler{},
		"AdminMailTemplatesDeleteHandler":      &mail_templates_admin.AdminMailTemplatesDeleteHandler{},
		"AdminMailTemplatesStartersHandler":    &mail_templates_admin.AdminMailTemplatesStartersHandler{},
		"AdminMailSettingsGetHandler":          &mail_settings_admin.AdminMailSettingsGetHandler{},
		"AdminMailSettingsUpdateHandler":       &mail_settings_admin.AdminMailSettingsUpdateHandler{},
		"AdminMailSettingsEventsHandler":       &mail_settings_admin.AdminMailSettingsEventsHandler{},
		"AdminMailSettingsTestHandler":         &mail_settings_admin.AdminMailSettingsTestHandler{},
		"AdminMailAccountsListHandler":         &mail_accounts_admin.AdminMailAccountsListHandler{},
		"AdminMailAccountsGetHandler":          &mail_accounts_admin.AdminMailAccountsGetHandler{},
		"AdminMailAccountsCreateHandler":       &mail_accounts_admin.AdminMailAccountsCreateHandler{},
		"AdminMailAccountsUpdateHandler":       &mail_accounts_admin.AdminMailAccountsUpdateHandler{},
		"AdminMailAccountsDeleteHandler":       &mail_accounts_admin.AdminMailAccountsDeleteHandler{},
		"AdminMailAccountsActivateHandler":     &mail_accounts_admin.AdminMailAccountsActivateHandler{},
		"AdminMailAccountsTestHandler":         &mail_accounts_admin.AdminMailAccountsTestHandler{},
		"ActivationVerifyHandler":              &activation_admin.VerifyHandler{},
		"ActivationActivateHandler":            &activation_admin.ActivateHandler{},
		"AdminActivationVerifyHandler":         &activation_admin.AdminPortalVerifyHandler{},
		"AdminActivationActivateHandler":       &activation_admin.AdminPortalActivateHandler{},
		"ActivationChangePasswordHandler":      &activation_admin.ChangePasswordHandler{},
		"AccountPasswordVerifyHandler":         &password_handler.VerifyHandler{},
		"AccountPasswordChangeHandler":         &password_handler.ChangeHandler{},
		"AdminUserRulesGetHandler":             &userrules_handler.AdminUserRulesGetHandler{},
		"AdminUserRulesUpdateHandler":          &userrules_handler.AdminUserRulesUpdateHandler{},
		"OrgUserRulesGetHandler":               &userrules_handler.OrgUserRulesGetHandler{},
		"OrgUserRulesUpdateHandler":            &userrules_handler.OrgUserRulesUpdateHandler{},
		"AccountUserRulesHandler":              &userrules_handler.AccountUserRulesHandler{},
		"RecoveryRequestHandler":               &recovery_handler.RequestHandler{},
		"RecoveryVerifyHandler":                &recovery_handler.VerifyHandler{},
		"RecoveryResetHandler":                 &recovery_handler.ResetHandler{},
		"AppLoginSettingsGetHandler":           &app_loginpage_handler.GetSettingsHandler{},
		"AppLoginSettingsUpdateHandler":        &app_loginpage_handler.UpdateSettingsHandler{},
		"AppSlugsHandler":                      &app_loginpage_handler.AppSlugsHandler{},
		"AppAnalyticsUsersCountHandler":        &analytics_handler.UsersCountHandler{},
		"AppAnalyticsGroupsCountHandler":       &analytics_handler.GroupsCountHandler{},
		"AppAnalyticsScopesCountHandler":       &analytics_handler.ScopesCountHandler{},
		"AppAnalyticsRecentGrantsHandler":      &analytics_handler.RecentGrantsHandler{},
		"AppAnalyticsPendingRecoveriesHandler": &analytics_handler.PendingRecoveriesHandler{},
		"AppKeysListHandler":                   &app_keys_handler.ListKeysHandler{},
		"AppKeysRegenerateHandler":             &app_keys_handler.RegenerateKeysHandler{},
		"AppKeysActivatePendingHandler":        &app_keys_handler.ActivatePendingHandler{},
		"AppKeysDiscardPendingHandler":         &app_keys_handler.DiscardPendingHandler{},
		"AppKeysDeactivateHandler":             &app_keys_handler.DeactivateKeyHandler{},
		"AppKeysPublicJWKSHandler":             &app_keys_handler.PublicJWKSHandler{},
		"AppLoginTemplateListAssetsHandler":    &app_loginpage_handler.ListAssetsHandler{},
		"AppLoginTemplateUploadAssetHandler":   &app_loginpage_handler.UploadAssetHandler{},
		"AppLoginTemplateDeleteAssetHandler":   &app_loginpage_handler.DeleteAssetHandler{},
		"AppLoginTemplatePublicGetHandler":     &app_loginpage_handler.PublicGetConfigHandler{},
		"AppLoginTemplatePublicAssetHandler":   &app_loginpage_handler.PublicServeAssetHandler{},
		"AppLoginAuthenticateHandler": func() *app_authn.AppLoginHandler {
			h := &app_authn.AppLoginHandler{
				Codes: func() oauthserver.CodeStore {
					if oauthSrv != nil {
						return oauthSrv.codes
					}
					return nil
				}(),
				Clients: func() oauthserver.ClientRegistry {
					if oauthSrv != nil {
						return oauthSrv.clients
					}
					return nil
				}(),
			}
			if bag, ok := ctx.(bagGetter); ok {
				if raw, ok := bag.Get(users_dbregistry.ContextBagKey); ok {
					h.OrgRegistry, _ = raw.(*users_dbregistry.OrgUserDBRegistry)
				}
			}
			return h
		}(),
		"AppLoginMethodsHandler": &app_authn.AppLoginMethodsHandler{},
		"AppRenewHandler":        &app_authn.AppRenewHandler{},
		// Slug-routed machine-auth API (/a/{org}/{ws}/{app}/...) — see
		// plan/application-api-credentials/plan.md.
		"AppApiGetPublicKeysHandler": &apicred_public.GetPublicKeysHandler{},
		"AppApiGetPrivateKeyHandler": &apicred_public.GetPrivateKeyHandler{},
		// Admin session surface for issuing + revoking the credentials
		// above.
		"AppApiCredentialsListHandler":   &apicred_admin.ListHandler{},
		"AppApiCredentialsCreateHandler": &apicred_admin.CreateHandler{},
		"AppApiCredentialsRevokeHandler": &apicred_admin.RevokeHandler{},
		// Per-application end-user login-attempt audit log. See
		// plan/login-audit-log/plan.md Step 8.
		"AppLoginLogListHandler":            &app_loginlog.AppLoginLogListHandler{},
		"AppLoginLogArchiveListHandler":     &app_loginlog.AppLoginLogArchiveListHandler{},
		"AppLoginLogArchiveAttemptsHandler": &app_loginlog.AppLoginLogArchiveAttemptsHandler{},
		// Workspace-application OAuth providers — admin CRUD.
		// See plan/workspace-app-oauth/plan.md.
		"AppOAuthProvidersListHandler":   &oauthproviders_admin.ListHandler{},
		"AppOAuthProvidersCreateHandler": &oauthproviders_admin.CreateHandler{},
		"AppOAuthProvidersUpdateHandler": &oauthproviders_admin.UpdateHandler{},
		"AppOAuthProvidersDeleteHandler": &oauthproviders_admin.DeleteHandler{},
		// External IdP authorize + callback (public endpoints).
		"AppOAuthAuthorizeHandler": oauthAuthorize,
		"AppOAuthCallbackHandler":  oauthCallback,
		// OAuth PROVIDER runtime endpoints — coco-iam acting as
		// OIDC IdP for third-party clients. See
		// plan/oauth-provider/plan.md.
		"AppOAuthSrvAuthorizeHandler":  oauthSrvAuthorize,
		"AppOAuthSrvTokenHandler":      oauthSrvToken,
		"AppOAuthSrvUserinfoHandler":   oauthSrvUserinfo,
		"AppOAuthSrvRevokeHandler":     oauthSrvRevoke,
		"AppOAuthSrvIntrospectHandler": oauthSrvIntrospect,
		"AppOAuthSrvDiscoveryHandler":  oauthSrvDiscovery,
		// User-facing "Connected apps" — list + revoke
		// per-(user, client) consents. Same RS256 bearer auth
		// as /profile/me.
		"AppOAuthSrvConsentsListHandler":   consentList,
		"AppOAuthSrvConsentsRevokeHandler": consentRevoke,
		// OAuth PROVIDER — admin CRUD for third-party clients
		// that use coco-iam as their OIDC IdP. See
		// plan/oauth-provider/plan.md.
		"AppOAuthClientsListHandler":   &oauthserver_admin.ListHandler{},
		"AppOAuthClientsCreateHandler": &oauthserver_admin.CreateHandler{},
		"AppOAuthClientsUpdateHandler": &oauthserver_admin.UpdateHandler{},
		"AppOAuthClientsRotateHandler": &oauthserver_admin.RotateHandler{},
		"AppOAuthClientsDeleteHandler": &oauthserver_admin.DeleteHandler{},
		// Registration schema — public endpoint + admin management.
		// See plan/application-registration-fields/plan.md.
		"AppApiRegistrationFieldsHandler": &regfields_public.RegistrationFieldsHandler{},
		// Registration submission — See plan/application-registration-submit/plan.md.
		"AppApiRegisterHandler":      &regfields_submit.RegisterHandler{},
		"AppApiRegisterCheckHandler": &regfields_submit.CheckHandler{},
		// User-facing profile: /a/{o}/{w}/{a}/profile/me — see
		// plan/app-user-profile-me/plan.md + the SOLID / test
		// refactor at plan/app-user-profile-me-testability/plan.md.
		// Built via a helper so the handler receives its deps
		// through interface fields instead of resolving them
		// per-request from the DI bag.
		"AppApiProfileMeHandler":              buildProfileMeHandler(profileMe),
		"AppMyLoginLogHandler":                buildMyLoginLogHandler(profileMe),
		"AppApiProfileMePatchHandler":         buildProfileMePatchHandler(profileMe),
		"AppApiProfileMeFileUploadHandler":    buildProfileMeFileUploadHandler(profileMe),
		"AppApiProfileMeFileDeleteHandler":    buildProfileMeFileDeleteHandler(profileMe),
		"AppApiProfileMeFileServeHandler":     buildProfileMeFileServeHandler(profileMe),
		"AppApiProfileFieldsHandler":          buildProfileFieldsHandler(ctx),
		"AppApiProfileFieldsPutHandler":       buildProfileFieldsPutHandler(ctx, profileMe),
		"AppRegistrationFieldsListHandler":    &regfields_admin.ListHandler{},
		"AppRegistrationFieldsReplaceHandler": &regfields_admin.ReplaceHandler{},
		"AppRecoveryPublicRequestHandler":     &app_recoverypage_handler.PublicRequestHandler{},
		"AppRecoveryPublicResetHandler":       &app_recoverypage_handler.PublicResetHandler{},
		// Public management API — see plan/application-public-api/plan.md
		"PublicApiUsersListHandler":          &publicapi_handler.UsersListHandler{},
		"PublicApiUsersGetHandler":           &publicapi_handler.UsersGetHandler{},
		"PublicApiUsersCreateHandler":        &publicapi_handler.UsersCreateHandler{},
		"PublicApiUsersPatchHandler":         &publicapi_handler.UsersPatchHandler{},
		"PublicApiUsersPasswordHandler":      &publicapi_handler.UsersPasswordHandler{},
		"PublicApiUsersDeleteHandler":        &publicapi_handler.UsersDeleteHandler{},
		"PublicApiUserAclGetHandler":         &publicapi_handler.UserAclGetHandler{},
		"PublicApiUserAclPutHandler":         &publicapi_handler.UserAclPutHandler{},
		"PublicApiUserAclDeleteHandler":      &publicapi_handler.UserAclDeleteHandler{},
		"PublicApiGroupsListHandler":         &publicapi_handler.GroupsListHandler{},
		"PublicApiGroupsGetHandler":          &publicapi_handler.GroupsGetHandler{},
		"PublicApiGroupsCreateHandler":       &publicapi_handler.GroupsCreateHandler{},
		"PublicApiGroupsPatchHandler":        &publicapi_handler.GroupsPatchHandler{},
		"PublicApiGroupsDeleteHandler":       &publicapi_handler.GroupsDeleteHandler{},
		"PublicApiGroupMembersListHandler":   &publicapi_handler.GroupMembersListHandler{},
		"PublicApiGroupMembersAddHandler":    &publicapi_handler.GroupMembersAddHandler{},
		"PublicApiGroupMembersRemoveHandler": &publicapi_handler.GroupMembersRemoveHandler{},
		"PublicApiGroupAclGetHandler":        &publicapi_handler.GroupAclGetHandler{},
		"PublicApiGroupAclPutHandler":        &publicapi_handler.GroupAclPutHandler{},
		"PublicApiGroupAclDeleteHandler":     &publicapi_handler.GroupAclDeleteHandler{},
		"PublicApiScopesListHandler":         &publicapi_handler.ScopesListHandler{},
		"PublicApiScopesGetHandler":          &publicapi_handler.ScopesGetHandler{},
		"PublicApiScopesCreateHandler":       &publicapi_handler.ScopesCreateHandler{},
		"PublicApiScopesPatchHandler":        &publicapi_handler.ScopesPatchHandler{},
		"PublicApiScopesDeleteHandler":       &publicapi_handler.ScopesDeleteHandler{},
		// Org user me: password notification preferences.
		"PasswordNotificationGetHandler":    &publicapi_handler.PasswordNotificationGetHandler{},
		"PasswordNotificationPutHandler":    &publicapi_handler.PasswordNotificationPutHandler{},
		"MediaListHandler":                  &app_media_handler.ListHandler{},
		"MediaCreateFolderHandler":          &app_media_handler.CreateFolderHandler{},
		"MediaDeleteFolderHandler":          &app_media_handler.DeleteFolderHandler{},
		"MediaUploadFileHandler":            &app_media_handler.UploadFileHandler{},
		"MediaDeleteFileHandler":            &app_media_handler.DeleteFileHandler{},
		"MediaFileServer":                   mediaFileServer(ctx),
		"AdminUserSendActivationHandler":    &admin_users.AdminUserSendActivationHandler{},
		"AdminUserResetPasswordHandler":     &admin_users.AdminUserResetPasswordHandler{},
		"ActivationResendAdminHandler":      &activation_admin.ResendAdminHandler{},
		"ActivationResendUserHandler":       &activation_admin.ResendUserHandler{},
		"AdminGeneralSettingsGetHandler":    &general_admin.AdminGeneralSettingsGetHandler{},
		"AdminGeneralSettingsUpdateHandler": &general_admin.AdminGeneralSettingsUpdateHandler{},
		"PublicGeneralSettingsHandler":      &general_admin.PublicGeneralSettingsHandler{},
		"OrgGeneralSettingsGetHandler":      &general_admin.OrgGeneralSettingsGetHandler{},
		"OrgGeneralSettingsUpdateHandler":   &general_admin.OrgGeneralSettingsUpdateHandler{},
		// coco-observe — system observability (push public/HMAC, query+agents admin-scoped).
		"ObservePushHandler":          observePush,
		"ObserveQueryHandler":         observeQuery,
		"ObserveAgentsHandler":        observeAgents,
		"ObserveAgentDownloadHandler": observeDownload,
		"ApiResourceHandler":          resource.GetApiResourceHandler(),
		"SwaggerSpecHandler":          &swagger_handler.SpecHandler{},
		"SwaggerRawSpecHandler":       &swagger_handler.RawSpecHandler{},
	}

	authCfgBytes, err := config.ReadConfigFile("config.json")
	if err != nil {
		panic(err)
	}
	scopeLayer := security.NewScopeSecurityLayer(handlerMap, authCfgBytes, updatedYamlBytes)

	// IPGuardSecurityLayer wraps scopeLayer — RouteBuilder.ServeHTTP
	// calls Authorize for every matched route, public or authenticated,
	// so this sees 100% of traffic including the public login endpoint.
	// See plan/ip-abuse-protection/plan.md sections 1 and 4.
	ipGuardCfg, err := ipguard.LoadConfig(authCfgBytes)
	if err != nil {
		panic(err)
	}
	bag, ok := ctx.(*di.ContextBag)
	if !ok {
		panic("routes.Init: DI context has unexpected type")
	}
	if bag.IPAttacksHandle == nil {
		panic("routes.Init: ip-attacks db handle not available")
	}

	// geoip enrichment — see plan/geoip-enrichment/plan.md. geoipCfg's
	// static (config.json) values are merged with whatever's been
	// saved via the admin settings UI (main DB, geoip_settings) — the
	// DB values win when present, config.json is the fallback for a
	// fresh install nobody has configured yet.
	geoipCfg, err := geoip.LoadConfig(authCfgBytes)
	if err != nil {
		panic(err)
	}
	if manager := bag.GetDatabaseManager(); manager != nil && manager.Connector != nil && manager.Connector.DB != nil {
		if settings, err := geoip.NewSettingsQueryRepo(manager.Connector.DB).LoadSettings(); err != nil {
			ctx.GetLogger().Warning("geoip: failed to load settings from the main database: %v (using config.json defaults only)", err)
		} else {
			geoipCfg = geoipCfg.WithSettings(settings)
		}
	}
	// Constructed unconditionally, regardless of geoipCfg.Enabled: geo
	// and the Watcher are always safe to have running — geoip.db
	// simply won't exist (or will be empty) until an admin actually
	// starts the updater, and SQLLookup/Watcher already handle that
	// gracefully (a miss, not an error). This also means enabling
	// geoip later via the admin UI + Start button takes effect
	// immediately, without needing to restart this admin server too —
	// the already-ticking Watcher picks up geoip.db on its very next
	// tick once the updater actually creates it.
	geo := geoip.NewSQLLookup(nil)
	geoWatcher := geoip.NewWatcher(geoipCfg.DBPath, geo, geoipCfg.CheckInterval(), ctx.GetLogger())
	bag.GeoIP = geo
	bag.GeoIPWatcher = geoWatcher
	bag.GeoIPManager = geoip.NewManager(geoipCfg.UpdaterBinaryPath, geoipCfg.PIDFile, geoipCfg.DBPath, ctx.GetLogger())

	ipGuard, err := ipguard.New(ipGuardCfg, scopeLayer, ctx, bag.IPAttacksHandle, bag.IPAttacksLog, geo)
	if err != nil {
		panic(err)
	}
	bag.IPGuard = ipGuard

	routing.GlobalRouteBuilder.SetSecurityLayer(ipGuard)

	// A request matching no route at all never reaches Authorize (see
	// its own doc comment), so this is the only seam that sees
	// path-probing against paths coco-iam doesn't route at all
	// (/wp-admin, /.env, etc.) — see plan/port-scan-detection/plan.md
	// Phase A. NotFoundHook is additive/nil-safe in the vendored
	// RouteBuilder, so this is the only place that needs to know about it.
	routing.GlobalRouteBuilder.NotFoundHook = func(r *http.Request) {
		ip := ipguard.ClientIP(r, ipGuardCfg.TrustProxyIPHeaders)
		ipGuard.RecordRecon(ip, r)
	}

	routing.GlobalRouteBuilder.AddRoute(
		routing.Routes{
			YamlContent: updatedYamlBytes,
			HandlerMap:  handlerMap,
		},
	)
}

// mediaFileServer returns the HandlerInterface that powers the
// `/p/media/**` catch-all route. The media service registered in
// main.go implements fileserver.Resolver. We wrap it with a slug
// dispatcher so URLs of the form /p/media/<org>/<ws>/<app>/<filename>
// resolve via loginpage.Store.FindBySlugs to the owning app's UUID
// before the file-server takes over. Legacy UUID-first URLs (admin
// MediaBrowser) pass through unchanged.
func mediaFileServer(ctx serverdi.Context) routing.HandlerInterface {
	type bagGetter interface {
		Get(key string) (interface{}, bool)
	}
	bag, ok := ctx.(bagGetter)
	if !ok {
		return &fileserver.Handler{} // resolver nil → 500 on request
	}
	raw, ok := bag.Get(app_media.ContextBagKeyService)
	if !ok {
		return &fileserver.Handler{}
	}
	svc, ok := raw.(*app_media.Service)
	if !ok {
		return &fileserver.Handler{}
	}
	delegate := &fileserver.Handler{
		Resolver:    svc,
		CacheMaxAge: 600,
	}

	// Attempt to resolve the loginpage Store so slug-based requests can
	// be rewritten to the UUID form. If the service isn't registered
	// yet, fall back to the raw file-server (legacy behaviour).
	rawLP, lpOK := bag.Get(app_loginpage.ContextBagKeyService)
	if !lpOK {
		return delegate
	}
	lpSvc, lpCast := rawLP.(*app_loginpage.Service)
	if !lpCast || lpSvc == nil {
		return delegate
	}
	return &app_slugmedia.Handler{
		Store:    lpSvc.Store,
		Delegate: delegate,
	}
}
