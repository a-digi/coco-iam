package stats

import (
	"database/sql"
	"net/http"

	"github.com/a-digi/coco-iam/config/di"
	"github.com/a-digi/coco-iam/src/auth/scopecheck"
	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-queue"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// DashboardStats holds aggregate counters shown in the dashboard stat-card row.
type DashboardStats struct {
	TotalAdminUsers       int `json:"total_admin_users"`
	ActiveAdminUsers      int `json:"active_admin_users"`
	TotalOrgUsers         int `json:"total_org_users"`
	ActiveOrgUsers        int `json:"active_org_users"`
	OrgUsersWithAppAccess int `json:"org_users_with_app_access"`
	TotalOrganizations    int `json:"total_organizations"`
	TotalWorkspaces       int `json:"total_workspaces"`
	TotalApplications     int `json:"total_applications"`
	TotalGroups           int `json:"total_groups"`
	QueuePending          int `json:"queue_pending"`
	QueueFailed           int `json:"queue_failed"`
}

// AdminDashboardStatsHandler serves GET /api/v1/admin/dashboard/stats.
type AdminDashboardStatsHandler struct{}

// @Summary     Get dashboard stats
// @Tags        admin-dashboard
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} interface{}
// @Failure     400,401,403,500 {object} map[string]interface{}
// @Router      /admin/dashboard/stats [get]
func (h *AdminDashboardStatsHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	ctx := reqCtx.GetDI()

	checker := scopecheck.NewChecker()
	if ok, _ := checker.HasScope(r.Header, "admin:dashboard:read"); !ok {
		response.ErrorResponse(w, http.StatusForbidden, "forbidden")
		return
	}

	manager := ctx.GetDatabaseManager()
	if manager == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "database manager not available")
		return
	}

	// Main-DB aggregates.
	var stats DashboardStats
	row := manager.Connector.DB.QueryRow(`
		SELECT
			(SELECT COUNT(*) FROM admin_users)                   AS total_admin_users,
			(SELECT COUNT(*) FROM admin_users WHERE is_active=1) AS active_admin_users,
			(SELECT COUNT(*) FROM organization)                  AS total_orgs,
			(SELECT COUNT(*) FROM admin_groups)                  AS total_groups
	`)
	if err := row.Scan(
		&stats.TotalAdminUsers,
		&stats.ActiveAdminUsers,
		&stats.TotalOrganizations,
		&stats.TotalGroups,
	); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to load dashboard stats: "+err.Error())
		return
	}

	// Fan out across per-org DBs for user, ACL, workspace, and application counts.
	if bag, ok := ctx.(*di.ContextBag); ok {
		if raw, ok := bag.Get(dbregistry.ContextBagKey); ok {
			if reg, ok := raw.(*dbregistry.OrgUserDBRegistry); ok {
				_ = reg.SweepExisting()
				for _, orgID := range reg.KnownOrgIDs() {
					mgr, err := reg.For(orgID)
					if err != nil || mgr == nil || mgr.Connector == nil {
						continue
					}
					orgDB := mgr.Connector.DB
					var total, active, withAccess, wsCount, appCount int
					_ = orgDB.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&total)
					_ = orgDB.QueryRow(`SELECT COUNT(*) FROM users WHERE is_active = 1`).Scan(&active)
					_ = orgDB.QueryRow(`SELECT COUNT(DISTINCT user_id) FROM application_user_acl WHERE is_active = 1`).Scan(&withAccess)
					_ = orgDB.QueryRow(`SELECT COUNT(*) FROM workspace`).Scan(&wsCount)
					_ = orgDB.QueryRow(`SELECT COUNT(*) FROM applications`).Scan(&appCount)
					stats.TotalOrgUsers += total
					stats.ActiveOrgUsers += active
					stats.OrgUsersWithAppAccess += withAccess
					stats.TotalWorkspaces += wsCount
					stats.TotalApplications += appCount
				}
			}
		}
	}

	// Queue aggregates — fan out across per-queue DB files.
	if bag, ok := ctx.(*di.ContextBag); ok {
		if raw, ok := bag.Get(queue.ContextBagKey); ok {
			if mgr, ok := raw.(queue.Manager); ok {
				_ = mgr.ForEachQueueDB(func(_ string, db *sql.DB) error {
					var p, f int
					_ = db.QueryRow(`SELECT COUNT(*) FROM queue_tasks WHERE status = 'pending'`).Scan(&p)
					_ = db.QueryRow(`SELECT COUNT(*) FROM queue_tasks WHERE status = 'failed'`).Scan(&f)
					stats.QueuePending += p
					stats.QueueFailed += f
					return nil
				})
			}
		}
	}

	response.SuccessResponse(w, http.StatusOK, stats)
}
