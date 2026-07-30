package handler

import (
	"net/http"
	"time"

	security_entity "github.com/a-digi/coco-iam/src/admin/security/entity"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// FirewallResyncHandler serves POST /api/v1/admin/security/firewall/resync.
type FirewallResyncHandler struct{}

// @Summary     Resync active IP bans into the OS-level firewall
// @Description Re-applies every currently-active (non-expired) ip_bans row through the same
// @Description Ban() path a fresh ban already uses — useful after a host reboot or manual firewall
// @Description flush. Already-loaded IPs are safely re-applied, not duplicated at the DB level.
// @Tags        security
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} security_entity.FirewallResyncSuccess
// @Failure     401,403,500 {object} response.ErrorBody
// @Router      /admin/security/firewall/resync [post]
func (h *FirewallResyncHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()

	guard, ok := resolveIPGuard(reqCtx)
	if !ok {
		return
	}

	bans, err := guard.ListBans()
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	now := time.Now().UTC()
	var synced, skipped, failed int
	for _, b := range bans {
		expiresAt, err := time.Parse(time.RFC3339, b.ExpiresAt)
		if err != nil {
			failed++
			continue
		}
		remaining := expiresAt.Sub(now)
		if remaining <= 0 {
			skipped++
			continue
		}
		if err := guard.Ban(b.IP, b.Tier, b.Reason, remaining, nil); err != nil {
			failed++
			continue
		}
		synced++
	}

	response.SuccessResponse(w, http.StatusOK, security_entity.FirewallResyncResponse{
		Synced:         synced,
		SkippedExpired: skipped,
		Failed:         failed,
	})
}
