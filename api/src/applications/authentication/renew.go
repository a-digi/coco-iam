package authentication

import (
	"crypto/rsa"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/a-digi/coco-iam/config"
	"github.com/a-digi/coco-iam/src/auth/oauth"
	"github.com/a-digi/coco-iam/src/orgrouter"
	oauth_lib "github.com/a-digi/coco-oauth/oauth"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// AppRenewHandler serves POST /api/v1/applications/renew?org=<o>&ws=<w>&app=<client_id>.
// URL shape matches the authenticate endpoint on purpose — callers
// only need to know the public slug triple, never the application
// UUID.
//
// Takes `{ refresh_token }`, verifies the signature against the
// application's public key (same algorithm the login path signed it
// with — RS256), and mints a fresh access + refresh pair signed with
// the application's private key again.
//
// The admin /oauth/renew endpoint is untouched; it still uses the
// shared HS256 secret and is admin-only.
type AppRenewHandler struct{}

type renewBody struct {
	RefreshToken string `json:"refresh_token"`
}

// renewGrace is the same 15-minute window the admin renew endpoint
// accepts past `exp` — lets a target that saw a 401 immediately after
// expiry still renew without forcing a full re-login.
const renewGrace = 15 * time.Minute

func (h *AppRenewHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	ctx := reqCtx.GetDI()

	// Same slug triple the authenticate endpoint requires. Missing any
	// of the three collapses to a 400 — these are URL-level parse
	// errors, not authentication failures, so distinguishing is safe.
	orgSlug := strings.TrimSpace(r.URL.Query().Get("org"))
	wsSlug := strings.TrimSpace(r.URL.Query().Get("ws"))
	clientID := strings.TrimSpace(r.URL.Query().Get("app"))
	if orgSlug == "" || wsSlug == "" || clientID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "missing org, ws, or app query parameter")
		return
	}

	svc := resolveLoginPageService(ctx)
	if svc == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "loginpage service not available")
		return
	}
	info, err := svc.Store.FindBySlugs(orgSlug, wsSlug, clientID)
	if err != nil {
		// Obfuscate — don't tell the caller whether it was the slug
		// triple or the refresh token itself that was wrong.
		response.ErrorResponse(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}
	appID := info.ID

	manager := ctx.GetDatabaseManager()
	if manager == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "database manager not available")
		return
	}

	var body renewBody
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	defer r.Body.Close()
	if body.RefreshToken == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "refresh_token is required")
		return
	}

	keysSvc := resolveKeysService(ctx)
	if keysSvc == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "keys service not available")
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

	// Resolve the per-org DB for ACL and resource-id lookups.
	orgReg := resolveOrgUserRegistry(ctx)
	var orgDBForRenew *sql.DB
	if orgReg != nil && info.OrganizationID != "" {
		if odb, oerr := orgrouter.ForOrg(orgReg, info.OrganizationID); oerr == nil {
			orgDBForRenew = odb
		}
	}
	mainConnDB := manager.Connector.DB

	// Close over the concrete services so the core decision layer
	// (renewAppToken) sees only function-typed callbacks. This is
	// what makes the core trivially unit-testable.
	loadKey := func(kid string) (*rsa.PublicKey, error) {
		return keysSvc.LoadVerifiablePublicKey(appID, kid)
	}
	mint := func(appID string, cfg oauth_lib.AuthConfig, sub string, scopes []string, resourceIDs map[string][]string) (oauth.LoginTokenResponse, error) {
		return oauth.IssueAppLoginTokens(keysSvc, appID, cfg, sub, scopes, resourceIDs)
	}
	loadResources := func(appID, userID string) map[string][]string {
		return loadResourceIDsFromDB(mainConnDB, orgDBForRenew, appID, userID)
	}

	fresh, err := renewAppToken(body.RefreshToken, appID, cfg, loadKey, mint, loadResources, time.Now())
	if err != nil {
		var rerr *RenewError
		if errors.As(err, &rerr) {
			response.ErrorResponse(w, rerr.Status, rerr.Message)
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, "token renew failed")
		return
	}
	response.SuccessResponse(w, http.StatusOK, fresh)
}

func splitScope(scope string) []string {
	if scope == "" {
		return nil
	}
	return strings.Fields(scope)
}

func containsScope(scope, want string) bool {
	for _, s := range splitScope(scope) {
		if s == want {
			return true
		}
	}
	return false
}

func stripRefreshScope(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == oauth.RefreshScope {
			continue
		}
		out = append(out, s)
	}
	return out
}

// loadResourceIDsFromDB mirrors the login-side resolver: for each
// scope the caller holds via application_user_acl.roles, include its
// resource_ids row from application_scopes. Both now live in the
// per-org DB. Returning nil on any failure is safe — the public
// management API re-reads from the catalog on every request.
func loadResourceIDsFromDB(mainDB *sql.DB, orgDB *sql.DB, appID, userID string) map[string][]string {
	if orgDB == nil {
		return nil
	}
	var rolesRaw []byte
	if err := orgDB.QueryRow(
		`SELECT roles FROM application_user_acl
		 WHERE application_id = ? AND user_id = ? AND is_active = TRUE
		 LIMIT 1`,
		appID, userID,
	).Scan(&rolesRaw); err != nil {
		return nil
	}
	var roles []string
	if err := json.Unmarshal(rolesRaw, &roles); err != nil || len(roles) == 0 {
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
		if err := json.Unmarshal(raw, &ids); err != nil || len(ids) == 0 {
			continue
		}
		out[scope] = ids
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

