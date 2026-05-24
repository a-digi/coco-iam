// Package handler implements CustomWorkspacesHandler which routes all CRUD
// for the workspace entity to the per-org users DB. Org routing is resolved
// by scanning per-org DBs for the workspace id.
package handler

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

// CustomWorkspacesHandler routes all CRUD operations for the workspace entity
// to the per-org DB (data/db/organization/{orgID}/users.db).
func CustomWorkspacesHandler(reqCtx request.RequestContext) {
	r := reqCtx.GetRequest()
	switch r.Method {
	case http.MethodPost:
		workspaceCreate(reqCtx)
	case http.MethodGet:
		workspaceGet(reqCtx)
	case http.MethodPatch, http.MethodPut:
		workspaceUpdate(reqCtx)
	case http.MethodDelete:
		workspaceDelete(reqCtx)
	default:
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusMethodNotAllowed, "method not allowed")
	}
}

type workspaceBody struct {
	WorkspaceID    string `json:"workspace_id"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	OrganizationID string `json:"organization_id"`
	IsActive       *bool  `json:"is_active"`
}

type workspaceRow struct {
	ID             string `json:"id"`
	WorkspaceID    string `json:"workspace_id"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	OrganizationID string `json:"organization_id"`
	CreatedAt      string `json:"created_at"`
	IsActive       bool   `json:"is_active"`
}

// --- POST ---------------------------------------------------------------

func workspaceCreate(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	_, reg, ok := resolveWorkspaceDBs(reqCtx, w)
	if !ok {
		return
	}

	var body workspaceBody
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if body.Title == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "title is required")
		return
	}
	if body.OrganizationID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "organization_id is required")
		return
	}

	orgDB, err := orgrouter.ForOrg(reg, body.OrganizationID)
	if err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "could not open org db: "+err.Error())
		return
	}

	id := uuid.New().String()
	isActive := true
	if body.IsActive != nil {
		isActive = *body.IsActive
	}

	if _, err := orgDB.Exec(
		`INSERT INTO workspace (id, workspace_id, title, description, organization_id, is_active)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		id, body.WorkspaceID, body.Title, body.Description, body.OrganizationID, isActive,
	); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to create workspace: "+err.Error())
		return
	}

	row, err := fetchWorkspaceRow(orgDB, id)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusCreated, row)
}

// --- GET ----------------------------------------------------------------

func workspaceGet(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	_, reg, ok := resolveWorkspaceDBs(reqCtx, w)
	if !ok {
		return
	}

	_, wsID := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if wsID == "" {
		wsID = reqCtx.GetURI().GetPathVariable("id")
	}

	orgID := extractOrgIDParam(r)

	if wsID != "" {
		// Get by ID: scan per-org DBs to resolve the workspace.
		orgDB, _, err := wsOrgDB(reg, wsID)
		if err != nil {
			response.ErrorResponse(w, http.StatusNotFound, "workspace not found")
			return
		}
		row, err := fetchWorkspaceRow(orgDB, wsID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				response.ErrorResponse(w, http.StatusNotFound, "workspace not found")
				return
			}
			response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		response.SuccessResponse(w, http.StatusOK, row)
		return
	}

	if orgID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "organization_id filter is required")
		return
	}

	orgDB, err := orgrouter.ForOrg(reg, orgID)
	if err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "could not open org db: "+err.Error())
		return
	}

	rows, err := orgDB.Query(
		`SELECT id, workspace_id, title, description, organization_id, created_at, is_active
		 FROM workspace
		 WHERE organization_id = ? AND is_active = TRUE
		 ORDER BY created_at DESC`,
		orgID,
	)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to query workspaces: "+err.Error())
		return
	}
	defer rows.Close()

	out := []workspaceRow{}
	for rows.Next() {
		row, err := scanWorkspaceRow(rows)
		if err != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		out = append(out, row)
	}
	response.SuccessResponse(w, http.StatusOK, out)
}

// --- PATCH / PUT --------------------------------------------------------

func workspaceUpdate(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	_, reg, ok := resolveWorkspaceDBs(reqCtx, w)
	if !ok {
		return
	}

	_, wsID := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if wsID == "" {
		wsID = reqCtx.GetURI().GetPathVariable("id")
	}
	if wsID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "workspace id missing from path")
		return
	}

	var body workspaceBody
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}

	orgDB, _, err := wsOrgDB(reg, wsID)
	if err != nil {
		response.ErrorResponse(w, http.StatusNotFound, "workspace not found")
		return
	}

	existing, err := fetchWorkspaceRow(orgDB, wsID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.ErrorResponse(w, http.StatusNotFound, "workspace not found")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	newTitle := existing.Title
	newDesc := existing.Description
	newWsID := existing.WorkspaceID
	newIsActive := existing.IsActive

	if body.Title != "" {
		newTitle = body.Title
	}
	if body.Description != "" {
		newDesc = body.Description
	}
	if body.WorkspaceID != "" {
		newWsID = body.WorkspaceID
	}
	if body.IsActive != nil {
		newIsActive = *body.IsActive
	}

	if _, err := orgDB.Exec(
		`UPDATE workspace SET workspace_id = ?, title = ?, description = ?, is_active = ? WHERE id = ?`,
		newWsID, newTitle, newDesc, newIsActive, wsID,
	); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to update workspace: "+err.Error())
		return
	}

	row, err := fetchWorkspaceRow(orgDB, wsID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, row)
}

// --- DELETE -------------------------------------------------------------

func workspaceDelete(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()

	_, reg, ok := resolveWorkspaceDBs(reqCtx, w)
	if !ok {
		return
	}

	_, wsID := uri.ExtractKeyAndValueFromURI(reqCtx.GetURI().GetPath())
	if wsID == "" {
		wsID = reqCtx.GetURI().GetPathVariable("id")
	}
	if wsID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "workspace id missing from path")
		return
	}

	orgDB, _, err := wsOrgDB(reg, wsID)
	if err != nil {
		response.ErrorResponse(w, http.StatusNotFound, "workspace not found")
		return
	}

	existing, err := fetchWorkspaceRow(orgDB, wsID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.ErrorResponse(w, http.StatusNotFound, "workspace not found")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	if _, err := orgDB.Exec(`UPDATE workspace SET is_active = FALSE WHERE id = ?`, wsID); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to delete workspace: "+err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, existing)
}

// --- internal helpers --------------------------------------------------

func resolveWorkspaceDBs(reqCtx request.RequestContext, w http.ResponseWriter) (*sql.DB, *dbregistry.OrgUserDBRegistry, bool) {
	ctx := reqCtx.GetDI()
	mgr := ctx.GetDatabaseManager()
	if mgr == nil || mgr.Connector == nil || mgr.Connector.DB == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "database manager not available")
		return nil, nil, false
	}
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
	return mgr.Connector.DB, reg, true
}

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

func extractOrgIDParam(r *http.Request) string {
	q := r.URL.Query()
	if v := strings.TrimSpace(q.Get("filter[@exact:organization_id]")); v != "" {
		return v
	}
	return strings.TrimSpace(q.Get("organization_id"))
}

func fetchWorkspaceRow(db *sql.DB, id string) (workspaceRow, error) {
	row := db.QueryRow(
		`SELECT id, workspace_id, title, description, organization_id, created_at, is_active
		 FROM workspace WHERE id = ? LIMIT 1`,
		id,
	)
	return scanWorkspaceRowFrom(row)
}

func scanWorkspaceRowFrom(row *sql.Row) (workspaceRow, error) {
	var out workspaceRow
	var createdAt sql.NullString
	var orgID sql.NullString
	if err := row.Scan(
		&out.ID, &out.WorkspaceID, &out.Title, &out.Description, &orgID, &createdAt, &out.IsActive,
	); err != nil {
		return workspaceRow{}, err
	}
	if createdAt.Valid {
		out.CreatedAt = createdAt.String
	}
	if orgID.Valid {
		out.OrganizationID = orgID.String
	}
	return out, nil
}

func scanWorkspaceRow(rows *sql.Rows) (workspaceRow, error) {
	var out workspaceRow
	var createdAt sql.NullString
	var orgID sql.NullString
	if err := rows.Scan(
		&out.ID, &out.WorkspaceID, &out.Title, &out.Description, &orgID, &createdAt, &out.IsActive,
	); err != nil {
		return workspaceRow{}, err
	}
	if createdAt.Valid {
		out.CreatedAt = createdAt.String
	}
	if orgID.Valid {
		out.OrganizationID = orgID.String
	}
	return out, nil
}
