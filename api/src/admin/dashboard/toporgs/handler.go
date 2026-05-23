package toporgs

import (
	"net/http"
	"sort"

	"github.com/a-digi/coco-iam/config/di"
	"github.com/a-digi/coco-iam/src/auth/scopecheck"
	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// OrgUserCount represents how many users belong to a single organisation.
type OrgUserCount struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// AdminDashboardTopOrgsHandler serves GET /api/v1/admin/dashboard/top-orgs.
type AdminDashboardTopOrgsHandler struct{}

// @Summary     Get top organisations by user count
// @Tags        admin-dashboard
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} interface{}
// @Failure     400,401,403,500 {object} map[string]interface{}
// @Router      /admin/dashboard/top-orgs [get]
func (h *AdminDashboardTopOrgsHandler) ServeHTTP(reqCtx request.RequestContext) {
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

	// Load all organizations from the main DB.
	rows, err := manager.Connector.DB.Query(`SELECT id, title FROM organization`)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to load organizations: "+err.Error())
		return
	}
	type orgRow struct {
		id    string
		title string
	}
	var orgs []orgRow
	for rows.Next() {
		var o orgRow
		if err := rows.Scan(&o.id, &o.title); err != nil {
			rows.Close()
			response.ErrorResponse(w, http.StatusInternalServerError, "failed to scan org: "+err.Error())
			return
		}
		orgs = append(orgs, o)
	}
	rows.Close()

	// Count users per org from per-org DBs.
	result := make([]OrgUserCount, 0, len(orgs))
	bag, hasBag := ctx.(*di.ContextBag)
	for _, o := range orgs {
		count := 0
		if hasBag {
			if raw, ok := bag.Get(dbregistry.ContextBagKey); ok {
				if reg, ok := raw.(*dbregistry.OrgUserDBRegistry); ok {
					if mgr, err := reg.For(o.id); err == nil && mgr != nil && mgr.Connector != nil {
						_ = mgr.Connector.DB.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
					}
				}
			}
		}
		result = append(result, OrgUserCount{ID: o.id, Name: o.title, Count: count})
	}

	sort.Slice(result, func(i, j int) bool { return result[i].Count > result[j].Count })
	if len(result) > 5 {
		result = result[:5]
	}

	response.SuccessResponse(w, http.StatusOK, result)
}
