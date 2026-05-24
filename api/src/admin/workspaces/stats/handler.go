package stats

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-iam/src/orgrouter"
	"github.com/a-digi/coco-lift/resource/uri"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

type countBreakdown struct {
	Total    int `json:"total"`
	Active   int `json:"active"`
	Inactive int `json:"inactive"`
}

type appBreakdown struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	UserCount int      `json:"user_count"`
	TopScopes []string `json:"top_scopes"`
}

type WorkspaceStatsResponse struct {
	OrganizationTitle     string         `json:"organization_title"`
	CreatedAt             string         `json:"created_at"`
	IsActive              bool           `json:"is_active"`
	Applications          countBreakdown `json:"applications"`
	Users                 countBreakdown `json:"users"`
	ApplicationsBreakdown []appBreakdown `json:"applications_breakdown"`
}

// WorkspaceStatsHandler serves GET /api/v1/workspaces/{res:workspaces}/{id}/stats
// and returns aggregate counts for applications and users belonging to the workspace.
type WorkspaceStatsHandler struct{}

// @Summary     Get workspace stats
// @Tags        workspaces
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} interface{}
// @Failure     400,401,403,500 {object} map[string]interface{}
// @Router      /workspaces/workspaces/{id}/stats [get]
func (h *WorkspaceStatsHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	ctx := reqCtx.GetDI()

	key, workspaceID := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	workspaceID = strings.TrimSpace(workspaceID)
	if key != "id" || workspaceID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "workspace id is required")
		return
	}

	manager := ctx.GetDatabaseManager()
	if manager == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "database manager not available")
		return
	}

	// Resolve registry.
	bag, ok := ctx.(interface{ Get(string) (interface{}, bool) })
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "DI context not keyed")
		return
	}
	raw, ok := bag.Get(dbregistry.ContextBagKey)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "org user db registry not available")
		return
	}
	reg, ok := raw.(*dbregistry.OrgUserDBRegistry)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "org user db registry type mismatch")
		return
	}

	// Scan per-org DBs to locate the workspace.
	var orgID string
	var orgDB *sql.DB
	for _, oid := range reg.KnownOrgIDs() {
		odb, err := orgrouter.ForOrg(reg, oid)
		if err != nil {
			continue
		}
		var found string
		if err := odb.QueryRow(`SELECT id FROM workspace WHERE id = ? LIMIT 1`, workspaceID).Scan(&found); err == nil {
			orgID = oid
			orgDB = odb
			break
		}
	}
	if orgDB == nil {
		response.ErrorResponse(w, http.StatusNotFound, "workspace not found")
		return
	}

	// Org title from main DB.
	var orgTitle string
	_ = manager.Connector.DB.QueryRow(
		`SELECT title FROM organization WHERE id = ? LIMIT 1`, orgID,
	).Scan(&orgTitle)

	var stats WorkspaceStatsResponse
	stats.OrganizationTitle = orgTitle

	// Workspace meta from per-org DB.
	if err := orgDB.QueryRow(
		`SELECT created_at, is_active FROM workspace WHERE id = ? LIMIT 1`,
		workspaceID,
	).Scan(&stats.CreatedAt, &stats.IsActive); err != nil {
		if err == sql.ErrNoRows {
			response.ErrorResponse(w, http.StatusNotFound, "workspace not found")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to load workspace: "+err.Error())
		return
	}

	// Application counts from per-org DB.
	appRow := orgDB.QueryRow(
		`SELECT
			COUNT(*),
			COALESCE(SUM(CASE WHEN is_active = 1 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN is_active = 0 THEN 1 ELSE 0 END), 0)
		 FROM applications
		 WHERE workspace_id = ? AND is_active = 1`,
		workspaceID,
	)
	if err := appRow.Scan(&stats.Applications.Total, &stats.Applications.Active, &stats.Applications.Inactive); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to load application counts: "+err.Error())
		return
	}

	// App IDs for this workspace from per-org DB.
	appIDRows, err := orgDB.Query(
		`SELECT id FROM applications WHERE workspace_id = ? AND is_active = 1`, workspaceID,
	)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to load app ids: "+err.Error())
		return
	}
	var wsAppIDs []string
	for appIDRows.Next() {
		var id string
		if appIDRows.Scan(&id) == nil {
			wsAppIDs = append(wsAppIDs, id)
		}
	}
	appIDRows.Close()

	if len(wsAppIDs) > 0 {
		ph := "?"
		args := make([]interface{}, len(wsAppIDs))
		args[0] = wsAppIDs[0]
		for i := 1; i < len(wsAppIDs); i++ {
			ph += ",?"
			args[i] = wsAppIDs[i]
		}
		userRow := orgDB.QueryRow(
			`SELECT
				COUNT(*),
				COALESCE(SUM(CASE WHEN u.is_active = 1 THEN 1 ELSE 0 END), 0),
				COALESCE(SUM(CASE WHEN u.is_active = 0 THEN 1 ELSE 0 END), 0)
			 FROM (
				SELECT DISTINCT u.id, u.is_active
				FROM application_user_acl acl
				JOIN users u ON u.id = acl.user_id
				WHERE acl.is_active = 1 AND acl.application_id IN (`+ph+`)
			 ) u`,
			args...,
		)
		_ = userRow.Scan(&stats.Users.Total, &stats.Users.Active, &stats.Users.Inactive)
	}

	// Per-application list from per-org DB.
	appRows, err := orgDB.Query(
		`SELECT id, title FROM applications WHERE workspace_id = ? AND is_active = 1 ORDER BY title`,
		workspaceID,
	)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to load application breakdown: "+err.Error())
		return
	}
	defer appRows.Close()

	breakdown := []appBreakdown{}
	appIndex := map[string]int{} // id to index in breakdown slice
	for appRows.Next() {
		var b appBreakdown
		if err := appRows.Scan(&b.ID, &b.Title); err != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, "failed to scan application row: "+err.Error())
			return
		}
		b.TopScopes = []string{}
		appIndex[b.ID] = len(breakdown)
		breakdown = append(breakdown, b)
	}
	appRows.Close()

	// Fill per-app user counts from the per-org DB.
	for i := range breakdown {
		_ = orgDB.QueryRow(
			`SELECT COUNT(DISTINCT user_id) FROM application_user_acl WHERE application_id = ? AND is_active = 1`,
			breakdown[i].ID,
		).Scan(&breakdown[i].UserCount)
	}

	// Top 4 active scopes per application from per-org DB.
	if len(wsAppIDs) > 0 {
		ph := "?"
		scopeArgs := make([]interface{}, len(wsAppIDs))
		scopeArgs[0] = wsAppIDs[0]
		for i := 1; i < len(wsAppIDs); i++ {
			ph += ",?"
			scopeArgs[i] = wsAppIDs[i]
		}
		scopeRows, err := orgDB.Query(
			`SELECT application_id, scope_id
			 FROM application_scopes
			 WHERE application_id IN (`+ph+`) AND is_active = 1
			 ORDER BY application_id, scope_id`,
			scopeArgs...,
		)
		if err != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, "failed to load scope breakdown: "+err.Error())
			return
		}
		defer scopeRows.Close()

		scopeCount := map[string]int{}
		for scopeRows.Next() {
			var appID, scopeID string
			if err := scopeRows.Scan(&appID, &scopeID); err != nil {
				response.ErrorResponse(w, http.StatusInternalServerError, "failed to scan scope row: "+err.Error())
				return
			}
			idx, ok := appIndex[appID]
			if !ok {
				continue
			}
			if scopeCount[appID] < 4 {
				breakdown[idx].TopScopes = append(breakdown[idx].TopScopes, scopeID)
				scopeCount[appID]++
			}
		}
		scopeRows.Close()
	}

	stats.ApplicationsBreakdown = breakdown

	response.SuccessResponse(w, http.StatusOK, stats)
}
