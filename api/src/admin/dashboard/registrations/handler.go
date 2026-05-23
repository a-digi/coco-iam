package registrations

import (
	"net/http"

	"github.com/a-digi/coco-iam/src/auth/scopecheck"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// RegistrationPoint is one bucket in a registration breakdown.
type RegistrationPoint struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

// OrgRegistrations groups organisation registrations by weekday, month and year.
type OrgRegistrations struct {
	ByWeekday []RegistrationPoint `json:"by_weekday"`
	ByMonth   []RegistrationPoint `json:"by_month"`
	ByYear    []RegistrationPoint `json:"by_year"`
}

// AdminDashboardRegistrationsHandler serves GET /api/v1/admin/dashboard/registrations.
type AdminDashboardRegistrationsHandler struct{}

// @Summary     Get organisation registrations breakdown
// @Tags        admin-dashboard
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} interface{}
// @Failure     400,401,403,500 {object} map[string]interface{}
// @Router      /admin/dashboard/registrations [get]
func (h *AdminDashboardRegistrationsHandler) ServeHTTP(reqCtx request.RequestContext) {
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

	resp := OrgRegistrations{
		ByWeekday: []RegistrationPoint{},
		ByMonth:   []RegistrationPoint{},
		ByYear:    []RegistrationPoint{},
	}

	// Weekday (0=Sun ... 6=Sat in SQLite, rendered Mon → Sun)
	weekdayNames := []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
	weekdayCounts := make(map[string]int, 7)
	weekdayRows, err := manager.Connector.DB.Query(`
		SELECT strftime('%w', created_at) AS weekday, COUNT(*) AS cnt
		FROM organization
		GROUP BY weekday
	`)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to load weekday registrations: "+err.Error())
		return
	}
	defer weekdayRows.Close()
	for weekdayRows.Next() {
		var wd string
		var cnt int
		if err := weekdayRows.Scan(&wd, &cnt); err != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, "failed to scan weekday row: "+err.Error())
			return
		}
		weekdayCounts[wd] = cnt
	}
	weekdayRows.Close()
	order := []string{"1", "2", "3", "4", "5", "6", "0"}
	for _, idx := range order {
		i := int(idx[0] - '0')
		resp.ByWeekday = append(resp.ByWeekday, RegistrationPoint{
			Label: weekdayNames[i],
			Count: weekdayCounts[idx],
		})
	}

	// Last 12 months
	monthRows, err := manager.Connector.DB.Query(`
		SELECT strftime('%Y-%m', created_at) AS month, COUNT(*) AS cnt
		FROM organization
		WHERE created_at >= date('now', 'start of month', '-11 months')
		GROUP BY month
		ORDER BY month ASC
	`)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to load month registrations: "+err.Error())
		return
	}
	defer monthRows.Close()
	for monthRows.Next() {
		var p RegistrationPoint
		if err := monthRows.Scan(&p.Label, &p.Count); err != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, "failed to scan month row: "+err.Error())
			return
		}
		resp.ByMonth = append(resp.ByMonth, p)
	}
	monthRows.Close()

	// All years with data
	yearRows, err := manager.Connector.DB.Query(`
		SELECT strftime('%Y', created_at) AS year, COUNT(*) AS cnt
		FROM organization
		GROUP BY year
		ORDER BY year ASC
	`)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to load year registrations: "+err.Error())
		return
	}
	defer yearRows.Close()
	for yearRows.Next() {
		var p RegistrationPoint
		if err := yearRows.Scan(&p.Label, &p.Count); err != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, "failed to scan year row: "+err.Error())
			return
		}
		resp.ByYear = append(resp.ByYear, p)
	}
	yearRows.Close()

	response.SuccessResponse(w, http.StatusOK, resp)
}
