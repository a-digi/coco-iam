package admin

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/a-digi/coco-iam/src/applications/acl"
	"github.com/a-digi/coco-lift/resource/uri"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
	"github.com/google/uuid"
)

// CustomApplicationScopesHandler routes all CRUD operations for
// application_scopes to the per-org DB (data/db/organization/{orgID}/users.db).
//
// Dispatches by method:
//
//	POST              — create scope (body must contain application_id + scope_id)
//	GET (list)        — requires filter[@exact:application_id] or ?application_id=
//	GET (by id)       — requires ?application_id= query param for org routing
//	PATCH / PUT       — body must contain application_id for org routing
//	DELETE            — requires ?application_id= query param for org routing
func CustomApplicationScopesHandler(reqCtx request.RequestContext) {
	r := reqCtx.GetRequest()
	switch r.Method {
	case http.MethodPost:
		appScopeCreate(reqCtx)
	case http.MethodGet:
		appScopeGet(reqCtx)
	case http.MethodPatch, http.MethodPut:
		appScopeUpdate(reqCtx)
	case http.MethodDelete:
		appScopeDelete(reqCtx)
	default:
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusMethodNotAllowed, "method not allowed")
	}
}

type scopeBody struct {
	ApplicationID string  `json:"application_id"`
	ScopeID       string  `json:"scope_id"`
	Description   string  `json:"description"`
	ResourceIDs   string  `json:"resource_ids"`
	IsActive      *bool   `json:"is_active"`
}

type scopeRow struct {
	ID            string `json:"id"`
	ApplicationID string `json:"application_id"`
	ScopeID       string `json:"scope_id"`
	Description   string `json:"description"`
	ResourceIDs   string `json:"resource_ids"`
	CreatedAt     string `json:"created_at"`
	IsActive      bool   `json:"is_active"`
}

// --- POST ------------------------------------------------------------------

func appScopeCreate(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	_, reg, ok := resolveAppAclDBs(reqCtx, w)
	if !ok {
		return
	}

	var body scopeBody
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if body.ApplicationID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "application_id is required")
		return
	}
	if body.ScopeID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "scope_id is required")
		return
	}
	if !acl.ScopeIDFormat.MatchString(body.ScopeID) {
		response.ErrorResponse(w, http.StatusBadRequest,
			fmt.Sprintf("scope_id %q is invalid — only letters, underscores and colon separators are allowed", body.ScopeID))
		return
	}

	orgDB, _, err := appOrgDB(reg, body.ApplicationID)
	if err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	resourceIDs := body.ResourceIDs
	if resourceIDs == "" {
		resourceIDs = "[]"
	}
	isActive := true
	if body.IsActive != nil {
		isActive = *body.IsActive
	}

	id := uuid.New().String()
	if _, err := orgDB.Exec(
		`INSERT INTO application_scopes
		    (id, application_id, scope_id, description, resource_ids, is_active)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		id, body.ApplicationID, body.ScopeID, body.Description, resourceIDs, isActive,
	); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to create scope: "+err.Error())
		return
	}

	row, err := fetchScopeRow(orgDB, id)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusCreated, row)
}

// --- GET -------------------------------------------------------------------

func appScopeGet(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	_, reg, ok := resolveAppAclDBs(reqCtx, w)
	if !ok {
		return
	}

	_, scopeID := uri.ExtractKeyAndValueFromURI(r.URL.Path)
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

	if scopeID != "" {
		row, err := fetchScopeRow(orgDB, scopeID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				response.ErrorResponse(w, http.StatusNotFound, "scope not found")
				return
			}
			response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		response.SuccessResponse(w, http.StatusOK, row)
		return
	}

	rows, err := orgDB.Query(
		`SELECT id, application_id, scope_id, description, resource_ids, created_at, is_active
		 FROM application_scopes
		 WHERE application_id = ?
		 ORDER BY scope_id ASC`,
		appID,
	)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to query scopes: "+err.Error())
		return
	}
	defer rows.Close()

	out := []scopeRow{}
	for rows.Next() {
		row, err := scanScopeRow(rows)
		if err != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		out = append(out, row)
	}
	response.SuccessResponse(w, http.StatusOK, out)
}

// --- PATCH / PUT -----------------------------------------------------------

func appScopeUpdate(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	_, reg, ok := resolveAppAclDBs(reqCtx, w)
	if !ok {
		return
	}

	_, scopeID := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if scopeID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "scope id missing from path")
		return
	}

	var body scopeBody
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if body.ApplicationID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "application_id is required in body for routing")
		return
	}

	orgDB, _, err := appOrgDB(reg, body.ApplicationID)
	if err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	existing, err := fetchScopeRow(orgDB, scopeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.ErrorResponse(w, http.StatusNotFound, "scope not found")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	newDesc := existing.Description
	newResourceIDs := existing.ResourceIDs
	newIsActive := existing.IsActive

	if body.Description != "" {
		newDesc = body.Description
	}
	if body.ResourceIDs != "" {
		newResourceIDs = body.ResourceIDs
	}
	if body.IsActive != nil {
		newIsActive = *body.IsActive
	}

	if _, err := orgDB.Exec(
		`UPDATE application_scopes
		    SET description = ?, resource_ids = ?, is_active = ?
		  WHERE id = ?`,
		newDesc, newResourceIDs, newIsActive, scopeID,
	); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to update scope: "+err.Error())
		return
	}

	row, err := fetchScopeRow(orgDB, scopeID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, row)
}

// --- DELETE ----------------------------------------------------------------

func appScopeDelete(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	_, reg, ok := resolveAppAclDBs(reqCtx, w)
	if !ok {
		return
	}

	_, scopeID := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if scopeID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "scope id missing from path")
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

	existing, err := fetchScopeRow(orgDB, scopeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.ErrorResponse(w, http.StatusNotFound, "scope not found")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	if _, err := orgDB.Exec(`DELETE FROM application_scopes WHERE id = ?`, scopeID); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to delete scope: "+err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, existing)
}

// --- internal helpers ------------------------------------------------------

func fetchScopeRow(db *sql.DB, id string) (scopeRow, error) {
	row := db.QueryRow(
		`SELECT id, application_id, scope_id, description, resource_ids, created_at, is_active
		 FROM application_scopes WHERE id = ? LIMIT 1`,
		id,
	)
	return scanScopeRowFrom(row)
}

type scopeScanner interface {
	Scan(dest ...any) error
}

func scanScopeRow(rows *sql.Rows) (scopeRow, error) {
	var out scopeRow
	var createdAt sql.NullString
	if err := rows.Scan(
		&out.ID, &out.ApplicationID, &out.ScopeID,
		&out.Description, &out.ResourceIDs, &createdAt, &out.IsActive,
	); err != nil {
		return scopeRow{}, fmt.Errorf("scan scope row: %w", err)
	}
	if createdAt.Valid {
		out.CreatedAt = createdAt.String
	}
	return out, nil
}

func scanScopeRowFrom(row *sql.Row) (scopeRow, error) {
	var out scopeRow
	var createdAt sql.NullString
	if err := row.Scan(
		&out.ID, &out.ApplicationID, &out.ScopeID,
		&out.Description, &out.ResourceIDs, &createdAt, &out.IsActive,
	); err != nil {
		return scopeRow{}, err
	}
	if createdAt.Valid {
		out.CreatedAt = createdAt.String
	}
	return out, nil
}
