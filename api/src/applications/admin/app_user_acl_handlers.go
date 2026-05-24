package admin

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/a-digi/coco-lift/resource/uri"
	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-iam/src/orgrouter"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
	"github.com/google/uuid"
)

// CustomApplicationUserAclHandler routes all CRUD operations for
// application_user_acl to the per-org DB (data/db/organization/{orgID}/users.db).
//
// Dispatches by method:
//
//	POST              — create mapping (body must contain application_id + user_id)
//	GET (list)        — requires filter[@exact:application_id] or ?application_id=
//	GET (by id)       — requires ?application_id= query param for org routing
//	PATCH / PUT       — body must contain application_id for org routing
//	DELETE            — requires ?application_id= query param for org routing
func CustomApplicationUserAclHandler(reqCtx request.RequestContext) {
	r := reqCtx.GetRequest()
	switch r.Method {
	case http.MethodPost:
		appUserAclCreate(reqCtx)
	case http.MethodGet:
		appUserAclGet(reqCtx)
	case http.MethodPatch, http.MethodPut:
		appUserAclUpdate(reqCtx)
	case http.MethodDelete:
		appUserAclDelete(reqCtx)
	default:
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusMethodNotAllowed, "method not allowed")
	}
}

// aclBody is the shared shape for POST / PATCH / PUT bodies.
type aclBody struct {
	ApplicationID  string          `json:"application_id"`
	UserID         string          `json:"user_id"`
	Roles          []string        `json:"roles"`
	GrantableRoles []string        `json:"grantable_roles"`
	ResourceIDs    json.RawMessage `json:"resource_ids"`
	IsActive       *bool           `json:"is_active"`
}

type aclRow struct {
	ID             string   `json:"id"`
	ApplicationID  string   `json:"application_id"`
	UserID         string   `json:"user_id"`
	Roles          []string `json:"roles"`
	GrantableRoles []string `json:"grantable_roles"`
	ResourceIDs    string   `json:"resource_ids"`
	CreatedAt      string   `json:"created_at"`
	IsActive       bool     `json:"is_active"`
}

// --- POST --------------------------------------------------------------

func appUserAclCreate(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	_, reg, ok := resolveAppAclDBs(reqCtx, w)
	if !ok {
		return
	}

	var body aclBody
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if body.ApplicationID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "application_id is required")
		return
	}
	if body.UserID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "user_id is required")
		return
	}

	orgDB, orgID, err := appOrgDB(reg, body.ApplicationID)
	if err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := validateUserInOrg(reg, body.UserID, orgID); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateRoles(orgDB, body.ApplicationID, body.Roles); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	if body.Roles == nil {
		body.Roles = []string{}
	}
	if body.GrantableRoles == nil {
		body.GrantableRoles = []string{}
	}
	rolesJSON, _ := json.Marshal(body.Roles)
	grantableJSON, _ := json.Marshal(body.GrantableRoles)
	resourceIDs := "{}"
	if len(body.ResourceIDs) > 0 {
		resourceIDs = string(body.ResourceIDs)
	}
	isActive := true
	if body.IsActive != nil {
		isActive = *body.IsActive
	}

	id := uuid.New().String()
	if _, err := orgDB.Exec(
		`INSERT INTO application_user_acl
		    (id, application_id, user_id, roles, grantable_roles, resource_ids, is_active)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, body.ApplicationID, body.UserID,
		string(rolesJSON), string(grantableJSON), resourceIDs, isActive,
	); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to create ACL: "+err.Error())
		return
	}

	row, err := fetchAclRow(orgDB, id)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusCreated, row)
}

// --- GET ---------------------------------------------------------------

func appUserAclGet(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	_, reg, ok := resolveAppAclDBs(reqCtx, w)
	if !ok {
		return
	}

	_, aclID := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if aclID == "" {
		aclID = reqCtx.GetURI().GetPathVariable("id")
	}

	appID := extractAppIDParam(r)
	if appID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "application_id filter is required")
		return
	}

	orgDB, _, err := appOrgDB(reg, appID)
	if err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	if aclID != "" {
		row, err := fetchAclRow(orgDB, aclID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				response.ErrorResponse(w, http.StatusNotFound, "ACL row not found")
				return
			}
			response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		response.SuccessResponse(w, http.StatusOK, row)
		return
	}

	rows, err := orgDB.Query(
		`SELECT id, application_id, user_id, roles, grantable_roles, resource_ids, created_at, is_active
		 FROM application_user_acl
		 WHERE application_id = ?
		 ORDER BY created_at DESC`,
		appID,
	)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to query ACLs: "+err.Error())
		return
	}
	defer rows.Close()

	out := []aclRow{}
	for rows.Next() {
		row, err := scanAclRow(rows)
		if err != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		out = append(out, row)
	}
	response.SuccessResponse(w, http.StatusOK, out)
}

// --- PATCH / PUT -------------------------------------------------------

func appUserAclUpdate(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	_, reg, ok := resolveAppAclDBs(reqCtx, w)
	if !ok {
		return
	}

	_, aclID := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if aclID == "" {
		aclID = reqCtx.GetURI().GetPathVariable("id")
	}
	if aclID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "ACL id missing from path")
		return
	}

	var body aclBody
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if body.ApplicationID == "" {
		body.ApplicationID = extractAppIDParam(r)
	}

	var orgDB *sql.DB
	var err error
	if body.ApplicationID != "" {
		orgDB, _, err = appOrgDB(reg, body.ApplicationID)
		if err != nil {
			response.ErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
	} else {
		orgDB, body.ApplicationID, err = findAclRowOrg(reg, aclID)
		if err != nil {
			response.ErrorResponse(w, http.StatusNotFound, "ACL row not found")
			return
		}
	}

	existing, err := fetchAclRow(orgDB, aclID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.ErrorResponse(w, http.StatusNotFound, "ACL row not found")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	newRoles := existing.Roles
	newGrantable := existing.GrantableRoles
	newResourceIDs := existing.ResourceIDs
	newIsActive := existing.IsActive

	if body.Roles != nil {
		if err := validateRoles(orgDB, body.ApplicationID, body.Roles); err != nil {
			response.ErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		newRoles = body.Roles
	}
	if body.GrantableRoles != nil {
		newGrantable = body.GrantableRoles
	}
	if len(body.ResourceIDs) > 0 {
		newResourceIDs = string(body.ResourceIDs)
	}
	if body.IsActive != nil {
		newIsActive = *body.IsActive
	}

	rolesJSON, _ := json.Marshal(newRoles)
	grantableJSON, _ := json.Marshal(newGrantable)

	if _, err := orgDB.Exec(
		`UPDATE application_user_acl
		    SET roles = ?, grantable_roles = ?, resource_ids = ?, is_active = ?
		  WHERE id = ?`,
		string(rolesJSON), string(grantableJSON), newResourceIDs, newIsActive, aclID,
	); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to update ACL: "+err.Error())
		return
	}

	row, err := fetchAclRow(orgDB, aclID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, row)
}

// --- DELETE ------------------------------------------------------------

func appUserAclDelete(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	_, reg, ok := resolveAppAclDBs(reqCtx, w)
	if !ok {
		return
	}

	_, aclID := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if aclID == "" {
		aclID = reqCtx.GetURI().GetPathVariable("id")
	}
	if aclID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "ACL id missing from path")
		return
	}

	appID := extractAppIDParam(r)
	if appID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "application_id query param is required")
		return
	}

	orgDB, _, err := appOrgDB(reg, appID)
	if err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	existing, err := fetchAclRow(orgDB, aclID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.ErrorResponse(w, http.StatusNotFound, "ACL row not found")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	if _, err := orgDB.Exec(`DELETE FROM application_user_acl WHERE id = ?`, aclID); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to delete ACL: "+err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, existing)
}

// --- internal helpers --------------------------------------------------

// resolveAppAclDBs extracts the OrgUserDBRegistry from DI context.
// The first return value is always nil and kept only for call-site compatibility
// during the transition; callers should use _ for it.
func resolveAppAclDBs(reqCtx request.RequestContext, w http.ResponseWriter) (*sql.DB, *dbregistry.OrgUserDBRegistry, bool) {
	ctx := reqCtx.GetDI()
	bag, ok := ctx.(interface{ Get(string) (interface{}, bool) })
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "DI context not keyed")
		return nil, nil, false
	}
	raw, ok := bag.Get(dbregistry.ContextBagKey)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "org user db registry not available")
		return nil, nil, false
	}
	reg, ok := raw.(*dbregistry.OrgUserDBRegistry)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "org user db registry type mismatch")
		return nil, nil, false
	}
	return nil, reg, true
}

// appOrgDB derives the organization ID from the per-org application_org_index and
// opens the per-org users DB.
func appOrgDB(reg *dbregistry.OrgUserDBRegistry, appID string) (*sql.DB, string, error) {
	return orgrouter.OrgDBForApp(reg, appID)
}

// wsOrgDB scans all known per-org DBs to find the one that owns wsID.
// Used when no routing index is available.
func wsOrgDB(reg *dbregistry.OrgUserDBRegistry, wsID string) (*sql.DB, string, error) {
	for _, orgID := range reg.KnownOrgIDs() {
		odb, err := orgrouter.ForOrg(reg, orgID)
		if err != nil {
			continue
		}
		var found string
		if err := odb.QueryRow(
			`SELECT id FROM workspace WHERE id = ? LIMIT 1`, wsID,
		).Scan(&found); err == nil {
			return odb, orgID, nil
		}
	}
	return nil, "", fmt.Errorf("workspace %q: org not found", wsID)
}

// findAclRowOrg scans all known per-org DBs to locate an application_user_acl
// row by its UUID. Used as a last resort when the caller did not supply
// application_id in the PATCH body or as a query parameter.
// Returns the matching org DB, the row's application_id, and nil on success.
func findAclRowOrg(reg *dbregistry.OrgUserDBRegistry, aclID string) (*sql.DB, string, error) {
	for _, orgID := range reg.KnownOrgIDs() {
		odb, err := orgrouter.ForOrg(reg, orgID)
		if err != nil {
			continue
		}
		var appID string
		if err := odb.QueryRow(
			`SELECT application_id FROM application_user_acl WHERE id = ? LIMIT 1`, aclID,
		).Scan(&appID); err == nil {
			return odb, appID, nil
		}
	}
	return nil, "", fmt.Errorf("ACL row %q not found in any org", aclID)
}

// validateUserInOrg verifies the user exists in the expected organization
// by checking the per-org DB directly.
func validateUserInOrg(reg *dbregistry.OrgUserDBRegistry, userID, expectedOrgID string) error {
	orgDB, err := orgrouter.ForOrg(reg, expectedOrgID)
	if err != nil {
		return fmt.Errorf("user validation: open org db: %w", err)
	}
	var exists int
	if err := orgDB.QueryRow(
		`SELECT COUNT(1) FROM users WHERE id = ? LIMIT 1`, userID,
	).Scan(&exists); err != nil || exists == 0 {
		return fmt.Errorf("user %q does not belong to this application's organization", userID)
	}
	return nil
}

// validateRoles checks every role name exists as an active scope on the application.
func validateRoles(mainDB *sql.DB, appID string, roles []string) error {
	for _, scopeID := range roles {
		var exists int
		if err := mainDB.QueryRow(
			`SELECT COUNT(1) FROM application_scopes WHERE application_id = ? AND scope_id = ? AND is_active = TRUE`,
			appID, scopeID,
		).Scan(&exists); err != nil {
			return fmt.Errorf("failed to look up scope %q: %w", scopeID, err)
		}
		if exists == 0 {
			return fmt.Errorf("scope %q is not defined for application %q", scopeID, appID)
		}
	}
	return nil
}

// extractAppIDParam reads application_id from filter[@exact:application_id] or ?application_id=.
func extractAppIDParam(r *http.Request) string {
	q := r.URL.Query()
	if v := strings.TrimSpace(q.Get("filter[@exact:application_id]")); v != "" {
		return v
	}
	return strings.TrimSpace(q.Get("application_id"))
}

func fetchAclRow(db *sql.DB, id string) (aclRow, error) {
	row := db.QueryRow(
		`SELECT id, application_id, user_id, roles, grantable_roles, resource_ids, created_at, is_active
		 FROM application_user_acl WHERE id = ? LIMIT 1`,
		id,
	)
	type scanner interface {
		Scan(dest ...any) error
	}
	return scanAclRowFrom(row)
}

func scanAclRowFrom(row *sql.Row) (aclRow, error) {
	var out aclRow
	var rolesRaw, grantableRaw []byte
	var createdAt sql.NullString
	if err := row.Scan(
		&out.ID, &out.ApplicationID, &out.UserID,
		&rolesRaw, &grantableRaw, &out.ResourceIDs,
		&createdAt, &out.IsActive,
	); err != nil {
		return aclRow{}, err
	}
	if createdAt.Valid {
		out.CreatedAt = createdAt.String
	}
	_ = json.Unmarshal(rolesRaw, &out.Roles)
	_ = json.Unmarshal(grantableRaw, &out.GrantableRoles)
	if out.Roles == nil {
		out.Roles = []string{}
	}
	if out.GrantableRoles == nil {
		out.GrantableRoles = []string{}
	}
	return out, nil
}

func scanAclRow(rows *sql.Rows) (aclRow, error) {
	var out aclRow
	var rolesRaw, grantableRaw []byte
	var createdAt sql.NullString
	if err := rows.Scan(
		&out.ID, &out.ApplicationID, &out.UserID,
		&rolesRaw, &grantableRaw, &out.ResourceIDs,
		&createdAt, &out.IsActive,
	); err != nil {
		return aclRow{}, err
	}
	if createdAt.Valid {
		out.CreatedAt = createdAt.String
	}
	_ = json.Unmarshal(rolesRaw, &out.Roles)
	_ = json.Unmarshal(grantableRaw, &out.GrantableRoles)
	if out.Roles == nil {
		out.Roles = []string{}
	}
	if out.GrantableRoles == nil {
		out.GrantableRoles = []string{}
	}
	return out, nil
}
