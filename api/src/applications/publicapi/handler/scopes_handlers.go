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

// ApplicationScopeView mirrors `application_scopes` trimmed to the
// fields the public API cares about.
type ApplicationScopeView struct {
	ID          string `json:"id"           example:"7f7f175d-cefa-4098-afec-b5469aeb2af5"`
	ScopeID     string `json:"scope_id"     example:"docs:read"`
	Description string `json:"description"  example:"Read documentation pages."`
	ResourceIDs string `json:"resource_ids" example:"[]"`
	IsActive    bool   `json:"is_active"    example:"true"`
}

// ApplicationScopeListPayload is the `message` body of the list
// endpoint's success envelope.
type ApplicationScopeListPayload struct {
	Scopes []ApplicationScopeView `json:"scopes"`
	Limit  int                    `json:"limit"  example:"50"`
	Offset int                    `json:"offset" example:"0"`
}

// ApplicationScopeListSuccess is the full response body for
// GET /public/applications/{id}/scopes.
type ApplicationScopeListSuccess struct {
	Success bool                        `json:"success" example:"true"`
	Message ApplicationScopeListPayload `json:"message"`
}

// ApplicationScopeSuccess is the full response body returned by the
// single-scope endpoints (get, create, patch).
type ApplicationScopeSuccess struct {
	Success bool                 `json:"success" example:"true"`
	Message ApplicationScopeView `json:"message"`
}

// ScopeDeleteStatus is the `message` body of the delete endpoint's
// success envelope.
type ScopeDeleteStatus struct {
	Status string `json:"status" example:"deleted"`
}

// ScopeDeleteSuccess is the full response body for
// DELETE /public/applications/{id}/scopes/{scopeId}.
type ScopeDeleteSuccess struct {
	Success bool              `json:"success" example:"true"`
	Message ScopeDeleteStatus `json:"message"`
}

// CreateScopeBody is the request body for
// POST /public/applications/{id}/scopes.
type CreateScopeBody struct {
	// ScopeID must match acl.ScopeIDFormat: letters/underscores per
	// segment, separated by colons. Immutable once created.
	ScopeID     string `json:"scope_id"     example:"docs:read"`
	Description string `json:"description"  example:"Read documentation pages."`
	// ResourceIDs is an opaque JSON-encoded array of ids this scope is
	// constrained to. Defaults to "[]" (unconstrained) when omitted.
	ResourceIDs string `json:"resource_ids" example:"[]"`
}

// PatchScopeBody is the request body for
// PATCH /public/applications/{id}/scopes/{scopeId}. scope_id is
// deliberately not patchable — see patchScopeBody's original comment.
type PatchScopeBody struct {
	Description *string `json:"description,omitempty"  example:"Read documentation pages."`
	ResourceIDs *string `json:"resource_ids,omitempty" example:"[]"`
	IsActive    *bool   `json:"is_active,omitempty"    example:"true"`
}

// -- Scopes list --------------------------------------------------------

type ScopesListHandler struct{}

// @Summary     List scopes for an application
// @Description Requires application scope `scopes:read` (held via the caller's `application_user_acl.roles` and carried in the bearer JWT's `scope` claim). Resource-id constrained callers only see ids in their `scopes:read` allow-list.
// @Tags        public-app-scopes
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Application ID"
// @Param       limit query int false "Page size (max 500, default 50)"
// @Param       offset query int false "Row offset (default 0)"
// @Success     200 {object} ApplicationScopeListSuccess
// @Failure     401 {object} response.ErrorBody "missing/invalid bearer token, or token was not issued for this application"
// @Failure     403 {object} response.ErrorBody "token does not carry the scopes:read scope"
// @Failure     500 {object} response.ErrorBody
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

	out := []ApplicationScopeView{}
	for rows.Next() {
		var s ApplicationScopeView
		if err := rows.Scan(&s.ID, &s.ScopeID, &s.Description, &s.ResourceIDs, &s.IsActive); err != nil {
			response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, err.Error())
			return
		}
		out = append(out, s)
	}
	response.SuccessResponse(reqCtx.GetWriter(), http.StatusOK, ApplicationScopeListPayload{
		Scopes: out,
		Limit:  limit,
		Offset: offset,
	})
}

// -- Scopes get -----------------------------------------------------------

type ScopesGetHandler struct{}

// @Summary     Get a scope by ID
// @Description Requires application scope `scopes:read`. Returns 404 both when the row doesn't exist and when it's outside the caller's resource-id allow-list for scopes:read.
// @Tags        public-app-scopes
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Application ID"
// @Param       scopeId path string true "Scope ID"
// @Success     200 {object} ApplicationScopeSuccess
// @Failure     401 {object} response.ErrorBody "missing/invalid bearer token, or token was not issued for this application"
// @Failure     403 {object} response.ErrorBody "token does not carry the scopes:read scope"
// @Failure     404 {object} response.ErrorBody "scope not found"
// @Failure     500 {object} response.ErrorBody
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
// @Description Requires application scope `scopes:write`. `scope_id` must match letters/underscores per colon-separated segment (e.g. `docs:read`) and is immutable once created.
// @Tags        public-app-scopes
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Application ID"
// @Param       body body CreateScopeBody true "Request body"
// @Success     201 {object} ApplicationScopeSuccess
// @Failure     400 {object} response.ErrorBody "invalid JSON, missing scope_id, or scope_id fails format validation"
// @Failure     401 {object} response.ErrorBody "missing/invalid bearer token, or token was not issued for this application"
// @Failure     403 {object} response.ErrorBody "token does not carry the scopes:write scope"
// @Failure     500 {object} response.ErrorBody
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
	var body CreateScopeBody
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
// @Description Requires application scope `scopes:write`. Only description, resource_ids, and is_active can be changed — scope_id is immutable.
// @Tags        public-app-scopes
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Application ID"
// @Param       scopeId path string true "Scope ID"
// @Param       body body PatchScopeBody true "Request body"
// @Success     200 {object} ApplicationScopeSuccess
// @Failure     400 {object} response.ErrorBody "invalid JSON body"
// @Failure     401 {object} response.ErrorBody "missing/invalid bearer token, or token was not issued for this application"
// @Failure     403 {object} response.ErrorBody "token does not carry the scopes:write scope"
// @Failure     404 {object} response.ErrorBody "scope not found"
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
	var body PatchScopeBody
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
// @Description Requires application scope `scopes:delete`. Sets is_active=false — the row is not removed, so a subsequent GET on the same id still returns it with is_active:false.
// @Tags        public-app-scopes
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Application ID"
// @Param       scopeId path string true "Scope ID"
// @Success     200 {object} ScopeDeleteSuccess
// @Failure     401 {object} response.ErrorBody "missing/invalid bearer token, or token was not issued for this application"
// @Failure     403 {object} response.ErrorBody "token does not carry the scopes:delete scope"
// @Failure     404 {object} response.ErrorBody "scope not found"
// @Failure     500 {object} response.ErrorBody
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
	response.SuccessResponse(reqCtx.GetWriter(), http.StatusOK, ScopeDeleteStatus{Status: "deleted"})
}

// -- helpers ---------------------------------------------------------------

func fetchScope(orgDB *sql.DB, appID, scopeID string) (ApplicationScopeView, error) {
	var s ApplicationScopeView
	err := orgDB.QueryRow(
		`SELECT id, scope_id, description, resource_ids, is_active
		 FROM application_scopes WHERE application_id = ? AND id = ? LIMIT 1`,
		appID, scopeID,
	).Scan(&s.ID, &s.ScopeID, &s.Description, &s.ResourceIDs, &s.IsActive)
	if err != nil {
		return ApplicationScopeView{}, err
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
