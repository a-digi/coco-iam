package recentusers

import (
	"net/http"

	"github.com/a-digi/coco-iam/src/auth/scopecheck"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// RecentUser is a trimmed view of a recently registered admin user.
type RecentUser struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	CreatedAt string `json:"created_at"`
}

// AdminDashboardRecentUsersHandler serves GET /api/v1/admin/dashboard/recent-users.
type AdminDashboardRecentUsersHandler struct{}

func (h *AdminDashboardRecentUsersHandler) ServeHTTP(reqCtx request.RequestContext) {
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

	result := []RecentUser{}
	rows, err := manager.Connector.DB.Query(`
		SELECT id, username, created_at
		FROM admin_users
		ORDER BY created_at DESC
		LIMIT 5
	`)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to load recent users: "+err.Error())
		return
	}
	defer rows.Close()
	for rows.Next() {
		var u RecentUser
		if err := rows.Scan(&u.ID, &u.Username, &u.CreatedAt); err != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, "failed to scan recent user row: "+err.Error())
			return
		}
		result = append(result, u)
	}
	rows.Close()

	response.SuccessResponse(w, http.StatusOK, result)
}
