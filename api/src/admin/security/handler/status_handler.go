package handler

import (
	"net/http"
	"runtime"

	"github.com/a-digi/coco-iam/config/di"
	archives_query "github.com/a-digi/coco-iam/src/admin/security/archives/repository/query"
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
// @Description plan/ip-abuse-protection/plan.md section 14. Also reports
// @Description ip-attacks.db's current size relative to the archiving
// @Description threshold — see plan/ip-attacks-db-archiving/plan.md.
// @Description Also reports whether port-scan detection has a log source
// @Description available on this host — see plan/port-scan-detection/plan.md.
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

	status := security_entity.SecurityStatus{
		OS:                runtime.GOOS,
		FirewallBackend:   name,
		FirewallAvailable: available,
		FirewallDetail:    detail,
	}

	// Archiver/archive-count visibility degrades gracefully rather than
	// failing the whole status response — the firewall fields above are
	// the important ones for this endpoint's original purpose, and a
	// missing archiver (never expected in the real wiring, but not
	// guaranteed in every DI setup) shouldn't take those down with it.
	if bag, ok := reqCtx.GetDI().(*di.ContextBag); ok {
		if archiver := bag.GetDBArchiver(); archiver != nil {
			status.IPAttacksEntryCount = archiver.EntryCount()
			status.IPAttacksThreshold = archiver.Threshold()
		}
		if manager := bag.GetDatabaseManager(); manager != nil && manager.Connector != nil && manager.Connector.DB != nil {
			if n, err := archives_query.NewArchiveQueryRepo(manager.Connector.DB).CountArchives(); err == nil {
				status.IPAttacksArchivesCount = n
			}
		}
		if source := bag.GetScanSource(); source != nil {
			status.ScanWatchSource = source.Name()
			status.ScanWatchAvailable = source.Available()
			status.ScanWatchDetail = source.Detail()
		}
	}

	response.SuccessResponse(w, http.StatusOK, status)
}
