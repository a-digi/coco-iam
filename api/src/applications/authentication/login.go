// Package authentication serves the public POST
// /api/v1/applications/authenticate?org=<org>&ws=<ws>&app=<app> endpoint.
// All three params are the admin-chosen slug identifiers
// (organization.organization_id, workspace.workspace_id,
// applications.client_id) — never UUIDs.
//
// Invariant: this handler queries only the `users` table (joined to
// application_user_acl to gate access by application). It never
// touches `admin_users`. Admin accounts authenticate exclusively
// through /api/v1/admin/oauth/authenticate. Keep these two
// populations disjoint.
package authentication

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/a-digi/coco-iam/config"
	"github.com/a-digi/coco-iam/src/applications/keys"
	loginlog_dbregistry "github.com/a-digi/coco-iam/src/applications/loginlog/dbregistry"
	loginlog_persistent "github.com/a-digi/coco-iam/src/applications/loginlog/repository/persistent"
	loginlog_query "github.com/a-digi/coco-iam/src/applications/loginlog/repository/query"
	"github.com/a-digi/coco-iam/src/applications/loginpage"
	oauthsession "github.com/a-digi/coco-iam/src/applications/oauthserverwiring"
	auth_db "github.com/a-digi/coco-iam/src/auth/database"
	auth_query "github.com/a-digi/coco-iam/src/auth/database/repository/query"
	"github.com/a-digi/coco-iam/src/auth/oauth"
	"github.com/a-digi/coco-iam/src/oauthserver"
	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	orgpwexpiry "github.com/a-digi/coco-iam/src/organizations/users/passwordexpiry"
	"github.com/a-digi/coco-iam/src/orgrouter"
	"github.com/a-digi/coco-iam/src/security/dbhandle"
	"github.com/a-digi/coco-iam/src/security/ipguard"
	"github.com/a-digi/coco-iam/src/security/loginbans"
	oauth_lib "github.com/a-digi/coco-oauth/oauth"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// loadUserAclRoles returns the caller's direct-ACL role set for the
// given application. Group-ACL inheritance is not expanded into the
// JWT today — the existing admin authorisation code treats them as
// separate paths and keeping that split makes the token smaller.
// Returns an empty slice on any error; login itself is not blocked
// by an ACL lookup failure.
func loadUserAclRoles(db *sql.DB, appID, userID string) []string {
	var raw []byte
	err := db.QueryRow(
		`SELECT roles FROM application_user_acl
		 WHERE application_id = ? AND user_id = ? AND is_active = TRUE
		 LIMIT 1`,
		appID, userID,
	).Scan(&raw)
	if err != nil || len(raw) == 0 {
		return nil
	}
	var roles []string
	if err := json.Unmarshal(raw, &roles); err != nil {
		return nil
	}
	return roles
}

// loadUserAclResourceIDs returns the per-scope resource-id allow-lists
// that apply to this user. Both ACL roles and the application_scopes
// catalog now live in the per-org DB.
func loadUserAclResourceIDs(mainDB *sql.DB, orgDB *sql.DB, appID, userID string) map[string][]string {
	roles := loadUserAclRoles(orgDB, appID, userID)
	if len(roles) == 0 {
		return nil
	}
	placeholders := "?"
	for i := 1; i < len(roles); i++ {
		placeholders += ",?"
	}
	args := make([]interface{}, 0, len(roles)+1)
	args = append(args, appID)
	for _, r := range roles {
		args = append(args, r)
	}
	rows, err := orgDB.Query(
		`SELECT scope_id, resource_ids FROM application_scopes
		 WHERE application_id = ? AND is_active = TRUE
		   AND scope_id IN (`+placeholders+`)`,
		args...,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := map[string][]string{}
	for rows.Next() {
		var scope string
		var raw []byte
		if err := rows.Scan(&scope, &raw); err != nil {
			continue
		}
		if len(raw) == 0 {
			continue
		}
		var ids []string
		if err := json.Unmarshal(raw, &ids); err != nil {
			continue
		}
		if len(ids) == 0 {
			continue
		}
		out[scope] = ids
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

type AppLoginHandler struct {
	// Codes is required for the OAuth client dispatch path and for
	// inline authorization-code minting when return_to is set.
	Codes oauthserver.CodeStore
	// Clients is required for validating OAuth authorize requests
	// embedded in return_to. When nil the return_to path is skipped.
	Clients oauthserver.ClientRegistry
	// OrgRegistry resolves per-org user DBs for ACL and user lookups.
	OrgRegistry *dbregistry.OrgUserDBRegistry
}

type credsBody struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// @Summary     Authenticate application user
// @Tags        app-auth
// @Accept      json
// @Produce     json
// @Router      /applications/authenticate [post]
func (h *AppLoginHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	ctx := reqCtx.GetDI()

	orgSlug := strings.TrimSpace(r.URL.Query().Get("org"))
	wsSlug := strings.TrimSpace(r.URL.Query().Get("ws"))
	clientID := strings.TrimSpace(r.URL.Query().Get("app"))
	if orgSlug == "" || wsSlug == "" || clientID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "missing org, ws, or app query parameter")
		return
	}

	manager := ctx.GetDatabaseManager()
	if manager == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "database manager not available")
		return
	}

	var creds credsBody
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&creds); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	defer r.Body.Close()
	creds.Username = strings.TrimSpace(creds.Username)
	if creds.Username == "" || creds.Password == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "username and password are required")
		return
	}

	svc := resolveLoginPageService(ctx)
	if svc == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "loginpage service not available")
		return
	}

	info, err := svc.Store.FindBySlugs(orgSlug, wsSlug, clientID)
	if err != nil {
		response.ErrorResponse(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	// Block the login flow unless the app's redirect target is fully
	// configured. Mirrors the FE's "not configured" banner — we don't
	// want to mint tokens for a callback we can't actually dispatch.
	loginSettings, err := svc.LoadSettings(info.ID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to load login settings")
		return
	}
	if !loginSettings.IsConfigured() {
		response.ErrorResponse(w, http.StatusConflict, "login not configured for this application")
		return
	}

	// Resolve the per-org DB for this application's org.
	orgRegistry := h.OrgRegistry
	if orgRegistry == nil {
		orgRegistry = resolveOrgUserRegistry(ctx)
	}
	if orgRegistry == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "org user db registry not available")
		return
	}
	orgDB, err := orgrouter.ForOrg(orgRegistry, info.OrganizationID)
	if err != nil {
		response.ErrorResponse(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	// Password login can be disabled at the application level. When
	// the admin has flipped allow_password_login off, the endpoint
	// rejects every attempt — OAuth is the only way in. Surfaced as
	// 401 (not 403/404) so the response is indistinguishable from a
	// bad password, preventing enumeration of which apps allow
	// password login via timing. Applications now live in per-org DB.
	if !passwordLoginAllowed(orgDB, info.ID) {
		recordLoginAttempt(reqCtx, info.ID, "", creds.Username, false, "inactive_user")
		response.ErrorResponse(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	userID, err := svc.Store.FindUserForLogin(orgDB, info.ID, creds.Username)
	if err != nil {
		recordLoginAttempt(reqCtx, info.ID, "", creds.Username, false, "invalid_credentials")
		response.ErrorResponse(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	pwrepo := auth_query.NewPasswordQueryRepositoryFromDB(orgDB)
	authenticator := auth_db.NewPasswordAuthenticator(pwrepo)
	ok, err := authenticator.Verify(userID, creds.Password)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "authentication error")
		return
	}
	if !ok {
		recordLoginAttempt(reqCtx, info.ID, userID, creds.Username, false, "invalid_credentials")
		response.ErrorResponse(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	cfgBytes, err := config.ReadConfigFile("config.json")
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "auth config error")
		return
	}
	cfg, err := oauth_lib.LoadAuthConfigFromBytes(cfgBytes)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "auth config error")
		return
	}

	var mustChange bool
	_ = orgDB.QueryRow(
		`SELECT must_change_password FROM users WHERE id = ?`, userID,
	).Scan(&mustChange)

	if !mustChange {
		if checker := resolveOrgExpiryChecker(reqCtx.GetDI()); checker != nil {
			if expired, err := checker.IsExpired(info.OrganizationID, userID); err == nil && expired {
				mustChange = true
			}
		}
	}

	scopes := []string{"user:me"}
	if mustChange {
		scopes = []string{"system:pwd_reset_required"}
	} else {
		scopes = append(scopes, LoadAllUserScopes(orgDB, info.ID, userID)...)
	}

	// Set the OAuth session cookie regardless of which dispatch path
	// follows — it lets /oauth/authorize skip the login screen on a
	// subsequent browser-initiated OAuth flow.
	if cfg.HS256Secret != "" && info.OrganizationID != "" && !mustChange {
		if store, err := oauthsession.NewSessionStore(cfg.HS256Secret); err == nil {
			if cookie, err := store.Issue(userID, info.OrganizationID); err == nil {
				http.SetCookie(w, cookie)
			}
		}
	}

	// return_to path: when the login was triggered by an OAuth authorize
	// request (the authorize handler bounced the user here), process the
	// authorize request inline so no session-cookie round-trip is needed.
	// This is safe because the user just proved their identity above.
	returnTo := strings.TrimSpace(r.URL.Query().Get("return_to"))
	if returnTo != "" && !mustChange && h.Codes != nil && h.Clients != nil {
		if finalURL, ok := h.handleReturnTo(r, orgDB, info.ID, userID, returnTo); ok {
			recordLoginAttempt(reqCtx, info.ID, userID, creds.Username, true, "")
			response.SuccessResponse(w, http.StatusOK, struct {
				RedirectURL string `json:"redirect_url"`
			}{RedirectURL: finalURL})
			return
		}
	}

	// OAuth client path: issue an authorization code and let the
	// browser carry it to the registered redirect URI. The client
	// app exchanges the code at /oauth/token for a proper
	// { access_token, token_type, expires_in, refresh_token }
	// response — fully RFC 6749 / OAuth 2.1 compatible. No tokens
	// ever appear in the browser URL or history.
	if loginSettings.OAuthClientID != nil && h.Codes != nil {
		uris, active, lookupErr := svc.Store.FindOAuthClientForDispatch(info.ID, *loginSettings.OAuthClientID)
		if lookupErr != nil || !active || len(uris) == 0 {
			response.ErrorResponse(w, http.StatusInternalServerError, "oauth client dispatch unavailable")
			return
		}
		redirectURI := uris[0]
		code, mintErr := h.Codes.Mint(r.Context(), oauthserver.CodeMintInput{
			ClientRowID:   *loginSettings.OAuthClientID,
			ApplicationID: info.ID,
			UserID:        userID,
			RedirectURI:   redirectURI,
			Scopes:        scopes,
			// No PKCE challenge — the token endpoint skips PKCE
			// verification when CodeChallenge is empty (server-
			// initiated flow, no prior client handshake).
		}, 0)
		if mintErr != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, "failed to issue authorization code")
			return
		}
		// Return redirect_url and code separately so the browser-side
		// login page assembles the final URL itself. This keeps the
		// /a/…/oauth/authorize backend route completely out of the
		// browser navigation path — the SPA never needs to handle it.
		recordLoginAttempt(reqCtx, info.ID, userID, creds.Username, true, "")
		response.SuccessResponse(w, http.StatusOK, struct {
			RedirectURL string `json:"redirect_url"`
			Code        string `json:"code"`
		}{
			RedirectURL: redirectURI,
			Code:        code,
		})
		return
	}

	// Manual dispatch path: server-to-server call with tokens.
	// Tokens never reach the browser — only the redirect URL does.
	keysSvc := resolveKeysService(ctx)
	if keysSvc == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "keys service not available")
		return
	}
	resourceIDs := loadUserAclResourceIDs(manager.Connector.DB, orgDB, info.ID, userID)
	tokenResp, err := oauth.IssueAppLoginTokens(keysSvc, info.ID, cfg, userID, scopes, resourceIDs)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "token signing failed: "+err.Error())
		return
	}
	// Login itself succeeded here — a token was issued against
	// verified credentials. The downstream redirect dispatch below is
	// a separate infrastructure concern (the app's own callback
	// endpoint being reachable), not part of the login outcome being
	// audited. See plan/login-audit-log/plan.md Step 7: "success once
	// a token or code is actually issued."
	recordLoginAttempt(reqCtx, info.ID, userID, creds.Username, true, "")
	dispatched, err := dispatchRedirect(loginSettings, tokenResp.AccessToken, tokenResp.RefreshToken)
	if err != nil {
		response.ErrorResponse(w, http.StatusBadGateway, "login dispatch failed: "+err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, struct {
		RedirectURL string `json:"redirect_url"`
	}{
		RedirectURL: dispatched.RedirectURL,
	})
}

// handleReturnTo parses an OAuth authorize URL from returnTo,
// validates the request against the registered clients, mints an
// authorization code, and returns the final redirect URI with
// code (and state) appended. The second return value is false
// when returnTo is not a recognisable authorize URL (caller
// falls through to the normal dispatch path).
func (h *AppLoginHandler) handleReturnTo(r *http.Request, orgDB *sql.DB, appID, userID, returnTo string) (string, bool) {
	parsed, err := url.Parse(returnTo)
	if err != nil {
		return "", false
	}
	q := parsed.Query()
	// Must look like an OAuth authorize request.
	if q.Get("response_type") != "code" || q.Get("client_id") == "" {
		return "", false
	}
	req := oauthserver.ParseAuthorizeRequest(q)
	decision, err := oauthserver.ValidateAuthorizeRequest(r.Context(), appID, req, h.Clients)
	if err != nil {
		return "", false
	}
	// Enrich granted scopes with all three ACL sources.
	extra := LoadAllUserScopes(orgDB, appID, userID)
	existing := make(map[string]struct{}, len(decision.GrantedScopes))
	for _, s := range decision.GrantedScopes {
		existing[s] = struct{}{}
	}
	for _, role := range extra {
		if _, ok := existing[role]; !ok {
			decision.GrantedScopes = append(decision.GrantedScopes, role)
		}
	}
	code, err := h.Codes.Mint(r.Context(), oauthserver.CodeMintInput{
		ClientRowID:         decision.Client.ID,
		ApplicationID:       appID,
		UserID:              userID,
		RedirectURI:         req.RedirectURI,
		Scopes:              decision.GrantedScopes,
		CodeChallenge:       req.CodeChallenge,
		CodeChallengeMethod: req.CodeChallengeMethod,
		Nonce:               req.Nonce,
	}, 0)
	if err != nil {
		return "", false
	}
	target, err := url.Parse(req.RedirectURI)
	if err != nil {
		return "", false
	}
	tq := target.Query()
	tq.Set("code", code)
	if req.State != "" {
		tq.Set("state", req.State)
	}
	target.RawQuery = tq.Encode()
	return target.String(), true
}

type bagGetter interface {
	Get(key string) (interface{}, bool)
}

func resolveLoginPageService(ctx interface{}) *loginpage.Service {
	bag, ok := ctx.(bagGetter)
	if !ok {
		return nil
	}
	raw, ok := bag.Get(loginpage.ContextBagKeyService)
	if !ok {
		return nil
	}
	svc, _ := raw.(*loginpage.Service)
	return svc
}

func resolveKeysService(ctx interface{}) *keys.Service {
	bag, ok := ctx.(bagGetter)
	if !ok {
		return nil
	}
	raw, ok := bag.Get(keys.ContextBagKeyService)
	if !ok {
		return nil
	}
	svc, _ := raw.(*keys.Service)
	return svc
}

func resolveOrgUserRegistry(ctx interface{}) *dbregistry.OrgUserDBRegistry {
	bag, ok := ctx.(bagGetter)
	if !ok {
		return nil
	}
	raw, ok := bag.Get(dbregistry.ContextBagKey)
	if !ok {
		return nil
	}
	reg, _ := raw.(*dbregistry.OrgUserDBRegistry)
	return reg
}

func resolveOrgDB(reg *dbregistry.OrgUserDBRegistry, orgID string) (*sql.DB, error) {
	return orgrouter.ForOrg(reg, orgID)
}

func resolveAppLoginLogRegistry(ctx interface{}) *loginlog_dbregistry.Registry {
	bag, ok := ctx.(bagGetter)
	if !ok {
		return nil
	}
	raw, ok := bag.Get(loginlog_dbregistry.ContextBagKey)
	if !ok {
		return nil
	}
	reg, _ := raw.(*loginlog_dbregistry.Registry)
	return reg
}

// resolveIPGuard duck-types against IPGuardSecurityLayer's own
// ClientIP method rather than importing config/di — same reasoning
// as every other resolver in this file: avoids depending on the
// concrete ContextBag type.
func resolveIPGuard(ctx interface{}) *ipguard.IPGuardSecurityLayer {
	bag, ok := ctx.(interface {
		GetIPGuard() *ipguard.IPGuardSecurityLayer
	})
	if !ok {
		return nil
	}
	return bag.GetIPGuard()
}

// recordLoginAttempt persists one application end-user login attempt
// into applicationID's own <slug>_login.db, best-effort — a write
// failure is logged and swallowed, never surfacing as a failure of
// the login itself. Silently does nothing if applicationID was never
// provisioned (e.g. an application created before this feature
// existed, or whose slug reservation failed) — nothing to write to,
// not worth logging on every single login attempt. IP/user-agent use
// the same ipguard.ClientIP resolution as every other rejection path
// in this codebase (see plan/attack-ip-attribution/plan.md), not a
// second IP-resolution path. See plan/login-audit-log/plan.md Step 7.
func recordLoginAttempt(reqCtx request.RequestContext, applicationID, applicationUserID, username string, success bool, failureReason string) {
	ctx := reqCtx.GetDI()
	registry := resolveAppLoginLogRegistry(ctx)
	if registry == nil {
		return
	}
	handle, err := registry.For(applicationID)
	if err != nil {
		return
	}

	r := reqCtx.GetRequest()
	var ip string
	if guard := resolveIPGuard(ctx); guard != nil {
		ip = guard.ClientIP(r)
	}

	repo := loginlog_persistent.NewApplicationLoginPersistentRepo(handle)
	if err := repo.RecordAttempt(applicationUserID, username, success, failureReason, ip, r.UserAgent()); err != nil {
		if log := reqCtx.GetDI().GetLogger(); log != nil {
			log.Warning("application login-log: failed to record attempt for %s: %v", applicationID, err)
		}
		return
	}

	if !success && ip != "" {
		checkApplicationFailureBan(reqCtx, handle, ip)
	}
}

// checkApplicationFailureBan counts this IP's recent failed
// application-login attempts and, if the configured (global, not
// per-application) threshold is crossed, bans it through the existing
// ipguard.Ban — reusing its allowlist bypass, firewall integration,
// and admin bans UI rather than building a second enforcement path.
// Best-effort, like recordLoginAttempt itself: an error here is
// swallowed, never surfacing as a failure of the login attempt being
// recorded. See plan/login-ban-rules/plan.md.
func checkApplicationFailureBan(reqCtx request.RequestContext, handle *dbhandle.Handle, ip string) {
	manager := reqCtx.GetDI().GetDatabaseManager()
	if manager == nil || manager.Connector == nil || manager.Connector.DB == nil {
		return
	}
	rules, err := loginbans.NewSettingsQueryRepo(manager.Connector.DB).LoadSettings()
	if err != nil || !rules.Application.Enabled {
		return
	}

	since := time.Now().UTC().Add(-time.Duration(rules.Application.WindowSeconds) * time.Second)
	count, err := loginlog_query.NewApplicationLoginQueryRepo(handle).CountRecentFailures(ip, since)
	if err != nil || count < rules.Application.Threshold {
		return
	}

	guard := resolveIPGuard(reqCtx.GetDI())
	if guard == nil {
		return
	}
	reason := fmt.Sprintf("%d failed application login attempts within %ds", count, rules.Application.WindowSeconds)
	if err := guard.Ban(ip, "application-login-failures", reason, time.Duration(rules.Application.BanSeconds)*time.Second, nil); err != nil {
		if log := reqCtx.GetDI().GetLogger(); log != nil {
			log.Warning("application login-log: failed to ban %s after %d failed logins: %v", ip, count, err)
		}
	}
}

func resolveOrgExpiryChecker(ctx interface{}) *orgpwexpiry.Checker {
	bag, ok := ctx.(interface {
		Get(string) (interface{}, bool)
	})
	if !ok {
		return nil
	}
	raw, ok := bag.Get("passwordexpiry.OrgChecker")
	if !ok {
		return nil
	}
	c, _ := raw.(*orgpwexpiry.Checker)
	return c
}
