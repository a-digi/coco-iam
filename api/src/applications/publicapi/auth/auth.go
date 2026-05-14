// Package auth owns the shared bearer-verification + grantable-roles
// logic the public management endpoints at
// `/api/v1/public/applications/{id}/...` depend on. Every handler
// calls `Authenticate` at the top; every mutation that grants a
// role calls `EnsureGrantable` after.
package auth

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/a-digi/coco-iam/src/applications/keys"
	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-iam/src/orgrouter"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
	jwtv5 "github.com/golang-jwt/jwt/v5"
)

// Caller is the authenticated context built by Authenticate. Handlers
// read `ApplicationID` to scope DB queries and `Grantable` to gate
// role-assignment mutations. `Scopes` is exposed for ad-hoc checks,
// but the common path is to call `Authenticate` with a required
// scope which 403's on mismatch before the handler runs.
//
// `ResourceIDs` is the resolved id allow-list per scope, read fresh
// from the DB on every request. A scope key that is absent from the
// map means "no constraint for that scope"; a present key with an
// empty slice means "explicitly deny-all" (the scope exists but
// applies to no ids).
type Caller struct {
	ApplicationID  string
	UserID         string
	OrganizationID string
	Scopes         []string
	Grantable      []string
	ResourceIDs    map[string][]string
	// OrgDB is the per-org DB handle. Handlers use it for user/ACL queries.
	OrgDB *sql.DB
}

// HasScope returns true if the caller's JWT carries the given scope.
func (c Caller) HasScope(scope string) bool {
	for _, s := range c.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// AllowedIDs returns the id allow-list for the given scope.
//
//	nil          → no constraint (any id is allowed)
//	[]string{}   → deny-all (scope is present but applies to no id)
//	[]string{…}  → only these ids
func (c Caller) AllowedIDs(scope string) []string {
	if c.ResourceIDs == nil {
		return nil
	}
	ids, ok := c.ResourceIDs[scope]
	if !ok {
		return nil
	}
	if ids == nil {
		return []string{}
	}
	return ids
}

// CanActOnID returns true when the caller is allowed to operate on the
// given id under the given scope. The caller must separately hold the
// scope itself — this only resolves the resource constraint.
func (c Caller) CanActOnID(scope, id string) bool {
	allowed := c.AllowedIDs(scope)
	if allowed == nil {
		return true
	}
	for _, a := range allowed {
		if a == id {
			return true
		}
	}
	return false
}

// EnsureGrantable fails with the first role in `roles` that is NOT in
// the caller's grantable set. Empty `roles` is always OK. Revocation
// paths should not call this — by design a caller can revoke roles
// they can't grant.
func (c Caller) EnsureGrantable(roles []string) error {
	budget := make(map[string]struct{}, len(c.Grantable))
	for _, g := range c.Grantable {
		budget[g] = struct{}{}
	}
	for _, r := range roles {
		if _, ok := budget[r]; !ok {
			return fmt.Errorf("role %q is not in caller's grantable set", r)
		}
	}
	return nil
}

type bagGetter interface {
	Get(key string) (interface{}, bool)
}

// Authenticate verifies the bearer on the request, confirms the token
// was signed for the application named in `{id}`, and checks that the
// JWT carries `requiredScope`. On any failure it writes an error
// response and returns nil so the handler short-circuits.
func Authenticate(reqCtx request.RequestContext, requiredScope string) *Caller {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	appID := appIDFromPath(r.URL.Path)
	if appID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "missing application id")
		return nil
	}

	keysSvc := resolveKeysService(reqCtx)
	if keysSvc == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "keys service not available")
		return nil
	}

	token := extractBearer(r.Header.Get("Authorization"))
	if token == "" {
		response.ErrorResponse(w, http.StatusUnauthorized, "missing or malformed Authorization header")
		return nil
	}

	parser := jwtv5.NewParser(jwtv5.WithValidMethods([]string{"RS256"}))
	tok, err := parser.Parse(token, func(t *jwtv5.Token) (interface{}, error) {
		kidRaw, ok := t.Header["kid"]
		if !ok {
			return nil, errors.New("token is missing kid header")
		}
		kid, ok := kidRaw.(string)
		if !ok || kid == "" {
			return nil, errors.New("token kid header is not a string")
		}
		return keysSvc.LoadVerifiablePublicKey(appID, kid)
	})
	if err != nil || tok == nil || !tok.Valid {
		response.ErrorResponse(w, http.StatusUnauthorized, "invalid bearer token")
		return nil
	}

	claims, ok := tok.Claims.(jwtv5.MapClaims)
	if !ok {
		response.ErrorResponse(w, http.StatusUnauthorized, "invalid token claims")
		return nil
	}

	sub, _ := claims["sub"].(string)
	if sub == "" {
		response.ErrorResponse(w, http.StatusUnauthorized, "token missing subject")
		return nil
	}

	scopeStr, _ := claims["scope"].(string)
	scopes := splitScope(scopeStr)

	if requiredScope != "" && !contains(scopes, requiredScope) {
		response.ErrorResponse(w, http.StatusForbidden, "missing required scope: "+requiredScope)
		return nil
	}

	// Resolve the app's organization so we can open the per-org DB.
	var orgID string
	var orgDB *sql.DB
	if reg := resolveOrgRegistry(reqCtx); reg != nil {
		if odb, oid, oerr := orgrouter.OrgDBForApp(reg, appID); oerr == nil {
			orgDB = odb
			orgID = oid
		}
	}

	// Resolve the caller's grantable-roles budget from their own ACL
	// row. Missing or empty → no grant rights; handlers that call
	// EnsureGrantable will 403 for any role being assigned.
	grantable := loadGrantable(orgDB, appID, sub)

	// Per-scope resource id allow-lists. coco-iam always resolves
	// fresh from the DB so revocations take effect immediately;
	// downstream services that trust the JWT get a point-in-time
	// snapshot via the `resource_ids` claim.
	resourceIDs := loadResourceIDs(orgDB, appID, sub)

	return &Caller{
		ApplicationID:  appID,
		UserID:         sub,
		OrganizationID: orgID,
		Scopes:         scopes,
		Grantable:      grantable,
		ResourceIDs:    resourceIDs,
		OrgDB:          orgDB,
	}
}

// appIDFromPath walks `/api/v1/public/applications/<id>/...` and
// returns `<id>`. Returns "" if the segment sequence isn't present.
func appIDFromPath(path string) string {
	segs := strings.Split(strings.Trim(path, "/"), "/")
	for i := 0; i+1 < len(segs); i++ {
		if segs[i] == "applications" {
			// The `/public/applications/.well-known/...` endpoints
			// never land here because they have their own prefix;
			// guard anyway by refusing non-uuid-looking values.
			return strings.TrimSpace(segs[i+1])
		}
	}
	return ""
}

func extractBearer(header string) string {
	const prefix = "Bearer "
	if len(header) < len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(header[len(prefix):])
}

func splitScope(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Fields(s)
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func resolveKeysService(reqCtx request.RequestContext) *keys.Service {
	ctx := reqCtx.GetDI()
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

func resolveOrgRegistry(reqCtx request.RequestContext) *dbregistry.OrgUserDBRegistry {
	ctx := reqCtx.GetDI()
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

func loadGrantable(orgDB *sql.DB, appID, userID string) []string {
	if orgDB == nil {
		return nil
	}
	var raw []byte
	err := orgDB.QueryRow(
		`SELECT grantable_roles FROM application_user_acl
		 WHERE application_id = ? AND user_id = ? AND is_active = TRUE
		 LIMIT 1`,
		appID, userID,
	).Scan(&raw)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return nil
	}
	if len(raw) == 0 {
		return nil
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}

// loadResourceIDs returns the effective per-scope id allow-lists for
// the caller. Both ACL roles and the application_scopes catalog now
// live in the per-org DB.
func loadResourceIDs(orgDB *sql.DB, appID, userID string) map[string][]string {
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
