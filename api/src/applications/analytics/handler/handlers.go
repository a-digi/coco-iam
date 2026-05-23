// Package handler serves the per-application analytics widgets on
// the Detail panel. Each handler corresponds to one widget and is
// route-gated by its own scope + the parent :read scope + super:admin.
package handler

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-iam/src/orgrouter"
	uri "github.com/a-digi/coco-lift/resource/uri"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// orgDBForApp resolves the per-org DB for a given application ID.
// Returns nil when the registry or org cannot be resolved — callers
// should degrade gracefully (return zero counts rather than errors).
func orgDBForApp(reqCtx request.RequestContext, appID string) *sql.DB {
	bag, ok := reqCtx.GetDI().(interface{ Get(string) (interface{}, bool) })
	if !ok {
		return nil
	}
	raw, ok := bag.Get(dbregistry.ContextBagKey)
	if !ok {
		return nil
	}
	reg, ok := raw.(*dbregistry.OrgUserDBRegistry)
	if !ok {
		return nil
	}
	orgDB, _, err := orgrouter.OrgDBForApp(reg, appID)
	if err != nil {
		return nil
	}
	return orgDB
}

func appIDFromPath(reqCtx request.RequestContext) string {
	key, value := uri.ExtractKeyAndValueFromURI(reqCtx.GetRequest().URL.Path)
	if key != "id" {
		return ""
	}
	return strings.TrimSpace(value)
}

type countPair struct {
	Total  int `json:"total"`
	Active int `json:"active"`
}

// UsersCountHandler — GET /api/v1/applications/{res:applications}/{id}/analytics/users-count
type UsersCountHandler struct{}

// @Summary     Get users count for an application
// @Tags        app-analytics
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Application ID"
// @Router      /applications/applications/{id}/analytics/users-count [get]
func (h *UsersCountHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	appID := appIDFromPath(reqCtx)
	if appID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "missing application id")
		return
	}
	var c countPair
	orgDB := orgDBForApp(reqCtx, appID)
	if orgDB != nil {
		_ = orgDB.QueryRow(
			`SELECT COUNT(*),
			        COALESCE(SUM(CASE WHEN is_active = 1 THEN 1 ELSE 0 END), 0)
			 FROM application_user_acl WHERE application_id = ?`,
			appID,
		).Scan(&c.Total, &c.Active)
	}
	response.SuccessResponse(w, http.StatusOK, c)
}

// GroupsCountHandler — GET /api/v1/applications/{res:applications}/{id}/analytics/groups-count
type GroupsCountHandler struct{}

// @Summary     Get groups count for an application
// @Tags        app-analytics
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Application ID"
// @Router      /applications/applications/{id}/analytics/groups-count [get]
func (h *GroupsCountHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	appID := appIDFromPath(reqCtx)
	if appID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "missing application id")
		return
	}
	orgDB := orgDBForApp(reqCtx, appID)
	if orgDB == nil {
		response.SuccessResponse(w, http.StatusOK, countPair{})
		return
	}
	var c countPair
	if err := orgDB.QueryRow(
		`SELECT COUNT(*),
		        COALESCE(SUM(CASE WHEN is_active = 1 THEN 1 ELSE 0 END), 0)
		 FROM application_group_acl WHERE application_id = ?`,
		appID,
	).Scan(&c.Total, &c.Active); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to count groups: "+err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, c)
}

// ScopesCountHandler — GET /api/v1/applications/{res:applications}/{id}/analytics/scopes-count
type ScopesCountHandler struct{}

// @Summary     Get scopes count for an application
// @Tags        app-analytics
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Application ID"
// @Router      /applications/applications/{id}/analytics/scopes-count [get]
func (h *ScopesCountHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	appID := appIDFromPath(reqCtx)
	if appID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "missing application id")
		return
	}
	orgDBScopes := orgDBForApp(reqCtx, appID)
	if orgDBScopes == nil {
		response.SuccessResponse(w, http.StatusOK, countPair{})
		return
	}
	var c countPair
	if err := orgDBScopes.QueryRow(
		`SELECT COUNT(*),
		        COALESCE(SUM(CASE WHEN is_active = 1 THEN 1 ELSE 0 END), 0)
		 FROM application_scopes WHERE application_id = ?`,
		appID,
	).Scan(&c.Total, &c.Active); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to count scopes: "+err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, c)
}

// RecentGrant is one row in the recent-grants widget.
type RecentGrant struct {
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	CreatedAt string `json:"created_at"`
}

// RecentGrantsHandler — GET /api/v1/applications/{res:applications}/{id}/analytics/recent-grants
type RecentGrantsHandler struct{}

// @Summary     Get recent grants for an application
// @Tags        app-analytics
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Application ID"
// @Router      /applications/applications/{id}/analytics/recent-grants [get]
func (h *RecentGrantsHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	appID := appIDFromPath(reqCtx)
	if appID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "missing application id")
		return
	}
	out := []RecentGrant{}
	orgDB := orgDBForApp(reqCtx, appID)
	if orgDB != nil {
		rows, err := orgDB.Query(
			`SELECT u.id, u.username, u.email, acl.created_at
			 FROM application_user_acl acl
			 JOIN users u ON u.id = acl.user_id
			 WHERE acl.application_id = ? AND acl.is_active = 1
			 ORDER BY acl.created_at DESC
			 LIMIT 5`,
			appID,
		)
		if err != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, "failed to load recent grants: "+err.Error())
			return
		}
		defer rows.Close()
		for rows.Next() {
			var g RecentGrant
			if err := rows.Scan(&g.UserID, &g.Username, &g.Email, &g.CreatedAt); err != nil {
				response.ErrorResponse(w, http.StatusInternalServerError, "failed to scan recent grant: "+err.Error())
				return
			}
			out = append(out, g)
		}
	}
	response.SuccessResponse(w, http.StatusOK, out)
}

type pendingCount struct {
	Count int `json:"count"`
}

// PendingRecoveriesHandler — GET /api/v1/applications/{res:applications}/{id}/analytics/pending-recoveries
// Counts active (un-consumed, un-expired) recovery tokens whose user
// is on this application's ACL.
type PendingRecoveriesHandler struct{}

// @Summary     Get pending recoveries count for an application
// @Tags        app-analytics
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Application ID"
// @Router      /applications/applications/{id}/analytics/pending-recoveries [get]
func (h *PendingRecoveriesHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	appID := appIDFromPath(reqCtx)
	if appID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "missing application id")
		return
	}
	var out pendingCount
	// Both application_user_acl and password_recoveries are in the per-org DB.
	orgDB := orgDBForApp(reqCtx, appID)
	if orgDB != nil {
		aclRows, err := orgDB.Query(
			`SELECT user_id FROM application_user_acl WHERE application_id = ? AND is_active = 1`,
			appID,
		)
		if err == nil {
			defer aclRows.Close()
			for aclRows.Next() {
				var userID string
				if aclRows.Scan(&userID) != nil {
					continue
				}
				var n int
				_ = orgDB.QueryRow(
					`SELECT COUNT(*) FROM password_recoveries
					 WHERE user_id = ? AND consumed_at IS NULL AND expires_at > CURRENT_TIMESTAMP`,
					userID,
				).Scan(&n)
				out.Count += n
			}
		}
	}
	response.SuccessResponse(w, http.StatusOK, out)
}
