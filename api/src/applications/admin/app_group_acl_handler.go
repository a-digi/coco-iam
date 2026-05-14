package admin

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/a-digi/coco-lift/resource/uri"
	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-iam/src/orgrouter"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
	"github.com/google/uuid"
)

// CustomApplicationGroupAclHandler routes all CRUD for application_group_acl
// to the per-org DB (data/db/organization/{orgID}/users.db).
func CustomApplicationGroupAclHandler(reqCtx request.RequestContext) {
	r := reqCtx.GetRequest()
	switch r.Method {
	case http.MethodPost:
		appGroupAclCreate(reqCtx)
	case http.MethodGet:
		appGroupAclGet(reqCtx)
	case http.MethodPatch, http.MethodPut:
		appGroupAclUpdate(reqCtx)
	case http.MethodDelete:
		appGroupAclDelete(reqCtx)
	default:
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusMethodNotAllowed, "method not allowed")
	}
}

type groupAclBody struct {
	ApplicationID  string          `json:"application_id"`
	GroupID        string          `json:"group_id"`
	Roles          []string        `json:"roles"`
	GrantableRoles []string        `json:"grantable_roles"`
	ResourceIDs    json.RawMessage `json:"resource_ids"`
	IsActive       *bool           `json:"is_active"`
}

type groupAclAdminRow struct {
	ID             string   `json:"id"`
	ApplicationID  string   `json:"application_id"`
	GroupID        string   `json:"group_id"`
	Roles          []string `json:"roles"`
	GrantableRoles []string `json:"grantable_roles"`
	ResourceIDs    string   `json:"resource_ids"`
	CreatedAt      string   `json:"created_at"`
	IsActive       bool     `json:"is_active"`
}

// --- POST --------------------------------------------------------------

func appGroupAclCreate(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	_, reg, ok := resolveAppAclDBs(reqCtx, w)
	if !ok {
		return
	}

	var body groupAclBody
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if body.ApplicationID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "application_id is required")
		return
	}
	if body.GroupID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "group_id is required")
		return
	}

	orgDB, _, err := appOrgDB(reg, body.ApplicationID)
	if err != nil {
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
		`INSERT INTO application_group_acl
		    (id, application_id, group_id, roles, grantable_roles, resource_ids, is_active)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, body.ApplicationID, body.GroupID,
		string(rolesJSON), string(grantableJSON), resourceIDs, isActive,
	); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to create group ACL: "+err.Error())
		return
	}

	row, err := fetchGroupAclRow(orgDB, id)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusCreated, row)
}

// --- GET ---------------------------------------------------------------

func appGroupAclGet(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	_, reg, ok := resolveAppAclDBs(reqCtx, w)
	if !ok {
		return
	}

	_, aclID := uri.ExtractKeyAndValueFromURI(r.URL.Path)

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
		row, err := fetchGroupAclRow(orgDB, aclID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				response.ErrorResponse(w, http.StatusNotFound, "group ACL row not found")
				return
			}
			response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		response.SuccessResponse(w, http.StatusOK, row)
		return
	}

	rows, err := orgDB.Query(
		`SELECT id, application_id, group_id, roles, grantable_roles, resource_ids, created_at, is_active
		 FROM application_group_acl
		 WHERE application_id = ?
		 ORDER BY created_at DESC`,
		appID,
	)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to query group ACLs: "+err.Error())
		return
	}
	defer rows.Close()

	out := []groupAclAdminRow{}
	for rows.Next() {
		row, err := scanGroupAclRow(rows)
		if err != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		out = append(out, row)
	}
	response.SuccessResponse(w, http.StatusOK, out)
}

// --- PATCH / PUT -------------------------------------------------------

func appGroupAclUpdate(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	_, reg, ok := resolveAppAclDBs(reqCtx, w)
	if !ok {
		return
	}

	_, aclID := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if aclID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "ACL id missing from path")
		return
	}

	var body groupAclBody
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
		orgDB, body.ApplicationID, err = findGroupAclRowOrg(reg, aclID)
		if err != nil {
			response.ErrorResponse(w, http.StatusNotFound, "group ACL row not found")
			return
		}
	}

	existing, err := fetchGroupAclRow(orgDB, aclID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.ErrorResponse(w, http.StatusNotFound, "group ACL row not found")
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
		`UPDATE application_group_acl
		    SET roles = ?, grantable_roles = ?, resource_ids = ?, is_active = ?
		  WHERE id = ?`,
		string(rolesJSON), string(grantableJSON), newResourceIDs, newIsActive, aclID,
	); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to update group ACL: "+err.Error())
		return
	}

	row, err := fetchGroupAclRow(orgDB, aclID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, row)
}

// --- DELETE ------------------------------------------------------------

func appGroupAclDelete(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	_, reg, ok := resolveAppAclDBs(reqCtx, w)
	if !ok {
		return
	}

	_, aclID := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if aclID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "ACL id missing from path")
		return
	}

	appID := extractAppIDParam(r)

	var orgDB *sql.DB
	var err error
	if appID != "" {
		orgDB, _, err = appOrgDB(reg, appID)
		if err != nil {
			response.ErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
	} else {
		orgDB, appID, err = findGroupAclRowOrg(reg, aclID)
		if err != nil {
			response.ErrorResponse(w, http.StatusNotFound, "group ACL row not found")
			return
		}
	}

	existing, err := fetchGroupAclRow(orgDB, aclID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.ErrorResponse(w, http.StatusNotFound, "group ACL row not found")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = appID

	if _, err := orgDB.Exec(`DELETE FROM application_group_acl WHERE id = ?`, aclID); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to delete group ACL: "+err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, existing)
}

// --- internal helpers --------------------------------------------------

// findGroupAclRowOrg scans all known per-org DBs to locate an
// application_group_acl row by its UUID. Used as a last resort when
// the caller did not supply application_id.
func findGroupAclRowOrg(reg *dbregistry.OrgUserDBRegistry, aclID string) (*sql.DB, string, error) {
	for _, orgID := range reg.KnownOrgIDs() {
		odb, err := orgrouter.ForOrg(reg, orgID)
		if err != nil {
			continue
		}
		var appID string
		if err := odb.QueryRow(
			`SELECT application_id FROM application_group_acl WHERE id = ? LIMIT 1`, aclID,
		).Scan(&appID); err == nil {
			return odb, appID, nil
		}
	}
	return nil, "", fmt.Errorf("group ACL row %q not found in any org", aclID)
}

func fetchGroupAclRow(db *sql.DB, id string) (groupAclAdminRow, error) {
	row := db.QueryRow(
		`SELECT id, application_id, group_id, roles, grantable_roles, resource_ids, created_at, is_active
		 FROM application_group_acl WHERE id = ? LIMIT 1`,
		id,
	)
	return scanGroupAclRowFrom(row)
}

func scanGroupAclRowFrom(row *sql.Row) (groupAclAdminRow, error) {
	var out groupAclAdminRow
	var rolesRaw, grantableRaw []byte
	var createdAt sql.NullString
	if err := row.Scan(
		&out.ID, &out.ApplicationID, &out.GroupID,
		&rolesRaw, &grantableRaw, &out.ResourceIDs,
		&createdAt, &out.IsActive,
	); err != nil {
		return groupAclAdminRow{}, err
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

func scanGroupAclRow(rows *sql.Rows) (groupAclAdminRow, error) {
	var out groupAclAdminRow
	var rolesRaw, grantableRaw []byte
	var createdAt sql.NullString
	if err := rows.Scan(
		&out.ID, &out.ApplicationID, &out.GroupID,
		&rolesRaw, &grantableRaw, &out.ResourceIDs,
		&createdAt, &out.IsActive,
	); err != nil {
		return groupAclAdminRow{}, err
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
