package handler

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/a-digi/coco-iam/src/applications/acl"
	"github.com/a-digi/coco-iam/src/applications/publicapi/auth"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// publicScope mirrors `application_scopes` trimmed to the fields the
// public API cares about.
type publicScope struct {
	ID          string `json:"id"`
	ScopeID     string `json:"scope_id"`
	Description string `json:"description"`
	ResourceIDs string `json:"resource_ids"`
	IsActive    bool   `json:"is_active"`
}

type createScopeBody struct {
	ScopeID     string `json:"scope_id"`
	Description string `json:"description"`
	ResourceIDs string `json:"resource_ids"`
}

// patchScopeBody deliberately has no ScopeID field — scope_id is
// immutable after creation. Any `application_user_acl.roles` entry
// already referencing the original string would silently break
// otherwise.
type patchScopeBody struct {
	Description *string `json:"description,omitempty"`
	ResourceIDs *string `json:"resource_ids,omitempty"`
	IsActive    *bool   `json:"is_active,omitempty"`
}

// -- Scopes list --------------------------------------------------------

type ScopesListHandler struct{}

// @Summary     List scopes for an application
// @Tags        public-api
// @Produce     json
// @Param       id path string true "Application ID"
// @Success     200 {object} interface{}
// @Failure     400,401,403,500 {object} map[string]interface{}
// @Router      /public/applications/{id}/scopes [get]
func (h *ScopesListHandler) ServeHTTP(reqCtx request.RequestContext) {
	caller := auth.Authenticate(reqCtx, "scopes:read")
	if caller == nil {
		return
	}
	if caller.OrgDB == nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, "org db not available")
		return
	}
	q := reqCtx.GetRequest().URL.Query()
	limit := parseLimit(q.Get("limit"), 50, 500)
	offset := parseOffset(q.Get("offset"))

	query := `SELECT id, scope_id, description, resource_ids, is_active
	          FROM application_scopes WHERE application_id = ?`
	args := []interface{}{caller.ApplicationID}
	if allowed := caller.AllowedIDs("scopes:read"); allowed != nil {
		query += ` AND id ` + buildInClause(len(allowed))
		args = append(args, stringArgs(allowed)...)
	}
	query += ` ORDER BY scope_id ASC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := caller.OrgDB.Query(query, args...)
	if err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	out := []publicScope{}
	for rows.Next() {
		var s publicScope
		if err := rows.Scan(&s.ID, &s.ScopeID, &s.Description, &s.ResourceIDs, &s.IsActive); err != nil {
			response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, err.Error())
			return
		}
		out = append(out, s)
	}
	response.SuccessResponse(reqCtx.GetWriter(), http.StatusOK, map[string]any{
		"scopes": out,
		"limit":  limit,
		"offset": offset,
	})
}

// -- Scopes get -----------------------------------------------------------

type ScopesGetHandler struct{}

// @Summary     Get a scope by ID
// @Tags        public-api
// @Produce     json
// @Param       id path string true "Application ID"
// @Param       scopeId path string true "Scope ID"
// @Success     200 {object} interface{}
// @Failure     400,401,403,500 {object} map[string]interface{}
// @Router      /public/applications/{id}/scopes/{scopeId} [get]
func (h *ScopesGetHandler) ServeHTTP(reqCtx request.RequestContext) {
	caller := auth.Authenticate(reqCtx, "scopes:read")
	if caller == nil {
		return
	}
	if caller.OrgDB == nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, "org db not available")
		return
	}
	scopeID := scopeIDFromPath(reqCtx.GetRequest().URL.Path)
	if !caller.CanActOnID("scopes:read", scopeID) {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusNotFound, "scope not found")
		return
	}
	s, err := fetchScope(caller.OrgDB, caller.ApplicationID, scopeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.ErrorResponse(reqCtx.GetWriter(), http.StatusNotFound, "scope not found")
			return
		}
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(reqCtx.GetWriter(), http.StatusOK, s)
}

// -- Scopes create --------------------------------------------------------

type ScopesCreateHandler struct{}

// @Summary     Create a scope for an application
// @Tags        public-api
// @Accept      json
// @Produce     json
// @Param       id path string true "Application ID"
// @Param       body body interface{} true "Request body"
// @Success     200 {object} interface{}
// @Failure     400,401,403,500 {object} map[string]interface{}
// @Router      /public/applications/{id}/scopes [post]
func (h *ScopesCreateHandler) ServeHTTP(reqCtx request.RequestContext) {
	caller := auth.Authenticate(reqCtx, "scopes:write")
	if caller == nil {
		return
	}
	if caller.OrgDB == nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, "org db not available")
		return
	}
	var body createScopeBody
	if err := reqCtx.BindJSON(&body); err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusBadRequest, "invalid JSON body")
		return
	}
	body.ScopeID = strings.TrimSpace(body.ScopeID)
	if body.ScopeID == "" {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusBadRequest, "scope_id is required")
		return
	}
	if !acl.ScopeIDFormat.MatchString(body.ScopeID) {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusBadRequest,
			fmt.Sprintf("scope_id %q is invalid — only letters, underscores and colon separators are allowed", body.ScopeID))
		return
	}
	resourceIDs := body.ResourceIDs
	if resourceIDs == "" {
		resourceIDs = "[]"
	}

	scopeID := newUUID()
	if _, err := caller.OrgDB.Exec(
		`INSERT INTO application_scopes (id, application_id, scope_id, description, resource_ids, is_active)
		 VALUES (?, ?, ?, ?, ?, TRUE)`,
		scopeID, caller.ApplicationID, body.ScopeID, body.Description, resourceIDs,
	); err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusBadRequest, err.Error())
		return
	}

	s, err := fetchScope(caller.OrgDB, caller.ApplicationID, scopeID)
	if err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(reqCtx.GetWriter(), http.StatusCreated, s)
}

// -- Scopes patch -----------------------------------------------------------

type ScopesPatchHandler struct{}

// @Summary     Patch a scope
// @Tags        public-api
// @Accept      json
// @Produce     json
// @Param       id path string true "Application ID"
// @Param       scopeId path string true "Scope ID"
// @Param       body body interface{} true "Request body"
// @Success     200 {object} interface{}
// @Failure     400,401,403,500 {object} map[string]interface{}
// @Router      /public/applications/{id}/scopes/{scopeId} [patch]
func (h *ScopesPatchHandler) ServeHTTP(reqCtx request.RequestContext) {
	caller := auth.Authenticate(reqCtx, "scopes:write")
	if caller == nil {
		return
	}
	scopeID := scopeIDFromPath(reqCtx.GetRequest().URL.Path)
	if !caller.CanActOnID("scopes:write", scopeID) || !scopeOnApp(caller.OrgDB, caller.ApplicationID, scopeID) {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusNotFound, "scope not found")
		return
	}
	var body patchScopeBody
	if err := reqCtx.BindJSON(&body); err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusBadRequest, "invalid JSON body")
		return
	}
	sets := []string{}
	args := []interface{}{}
	if body.Description != nil {
		sets = append(sets, "description = ?")
		args = append(args, *body.Description)
	}
	if body.ResourceIDs != nil {
		sets = append(sets, "resource_ids = ?")
		args = append(args, *body.ResourceIDs)
	}
	if body.IsActive != nil {
		sets = append(sets, "is_active = ?")
		args = append(args, *body.IsActive)
	}
	if len(sets) == 0 {
		s, _ := fetchScope(caller.OrgDB, caller.ApplicationID, scopeID)
		response.SuccessResponse(reqCtx.GetWriter(), http.StatusOK, s)
		return
	}
	args = append(args, scopeID)
	if _, err := caller.OrgDB.Exec(
		`UPDATE application_scopes SET `+strings.Join(sets, ", ")+` WHERE id = ?`,
		args...,
	); err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusBadRequest, err.Error())
		return
	}
	s, _ := fetchScope(caller.OrgDB, caller.ApplicationID, scopeID)
	response.SuccessResponse(reqCtx.GetWriter(), http.StatusOK, s)
}

// -- Scopes delete (soft) ---------------------------------------------------

type ScopesDeleteHandler struct{}

// @Summary     Delete a scope (soft)
// @Tags        public-api
// @Produce     json
// @Param       id path string true "Application ID"
// @Param       scopeId path string true "Scope ID"
// @Success     200 {object} interface{}
// @Failure     400,401,403,500 {object} map[string]interface{}
// @Router      /public/applications/{id}/scopes/{scopeId} [delete]
func (h *ScopesDeleteHandler) ServeHTTP(reqCtx request.RequestContext) {
	caller := auth.Authenticate(reqCtx, "scopes:delete")
	if caller == nil {
		return
	}
	scopeID := scopeIDFromPath(reqCtx.GetRequest().URL.Path)
	if !caller.CanActOnID("scopes:delete", scopeID) || !scopeOnApp(caller.OrgDB, caller.ApplicationID, scopeID) {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusNotFound, "scope not found")
		return
	}
	if _, err := caller.OrgDB.Exec(`UPDATE application_scopes SET is_active = FALSE WHERE id = ?`, scopeID); err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(reqCtx.GetWriter(), http.StatusOK, map[string]string{"status": "deleted"})
}

// -- helpers ---------------------------------------------------------------

func fetchScope(orgDB *sql.DB, appID, scopeID string) (publicScope, error) {
	var s publicScope
	err := orgDB.QueryRow(
		`SELECT id, scope_id, description, resource_ids, is_active
		 FROM application_scopes WHERE application_id = ? AND id = ? LIMIT 1`,
		appID, scopeID,
	).Scan(&s.ID, &s.ScopeID, &s.Description, &s.ResourceIDs, &s.IsActive)
	if err != nil {
		return publicScope{}, err
	}
	return s, nil
}

func scopeOnApp(orgDB *sql.DB, appID, scopeID string) bool {
	if orgDB == nil || scopeID == "" {
		return false
	}
	var exists int
	err := orgDB.QueryRow(
		`SELECT 1 FROM application_scopes WHERE application_id = ? AND id = ? LIMIT 1`,
		appID, scopeID,
	).Scan(&exists)
	return err == nil
}

func scopeIDFromPath(path string) string {
	segs := strings.Split(strings.Trim(path, "/"), "/")
	for i := 0; i+1 < len(segs); i++ {
		if segs[i] == "scopes" {
			return strings.TrimSpace(segs[i+1])
		}
	}
	return ""
}
