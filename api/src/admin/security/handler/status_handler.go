package handler

import (
	"net/http"
	"runtime"

	security_entity "github.com/a-digi/coco-iam/src/admin/security/entity"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// SecurityStatusHandler serves GET /api/v1/admin/security/status.
type SecurityStatusHandler struct{}

// @Summary     IP-guard security status
// @Description Reports whether OS-level firewall enforcement is active. When
// @Description firewall_available is false, only application-layer rate
// @Description limiting (429 responses) is protecting this host — see
// @Description plan/ip-abuse-protection/plan.md section 14.
// @Tags        security
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} security_entity.SecurityStatusSuccess
// @Failure     401,403,500 {object} response.ErrorBody
// @Router      /admin/security/status [get]
func (h *SecurityStatusHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	guard, ok := resolveIPGuard(reqCtx)
	if !ok {
		return
	}
	name, available, detail := guard.FirewallStatus()
	response.SuccessResponse(w, http.StatusOK, security_entity.SecurityStatus{
		OS:                runtime.GOOS,
		FirewallBackend:   name,
		FirewallAvailable: available,
		FirewallDetail:    detail,
	})
}
