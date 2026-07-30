package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/a-digi/coco-iam/config/di"
	security_entity "github.com/a-digi/coco-iam/src/admin/security/entity"
	loginlog_query "github.com/a-digi/coco-iam/src/admin/security/loginlog/repository/query"
	app_loginlog_dbregistry "github.com/a-digi/coco-iam/src/applications/loginlog/dbregistry"
	app_loginlog_query "github.com/a-digi/coco-iam/src/applications/loginlog/repository/query"
	"github.com/a-digi/coco-iam/src/auth/scopecheck"
	users_dbregistry "github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-iam/src/orgrouter"
	"github.com/a-digi/coco-lift/resource/uri"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// IPBanListHandler serves GET /api/v1/admin/security/ip-bans.
type IPBanListHandler struct{}

// @Summary     List IP bans
// @Description Lists every currently-tracked IP ban, including
// @Description already-expired rows the sweeper hasn't pruned yet.
// @Tags        security
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} security_entity.IPBanListSuccess
// @Failure     401,403,500 {object} response.ErrorBody
// @Router      /admin/security/ip-bans [get]
func (h *IPBanListHandler) ServeHTTP(reqCtx request.RequestContext) {
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
	if bans == nil {
		bans = []security_entity.IPBan{}
	}
	response.SuccessResponse(w, http.StatusOK, bans)
}

// IPBanCreateHandler serves POST /api/v1/admin/security/ip-bans — a
// manual ban (tier "manual"), re-using the same enforcement path as
// auto-bans so a manually-banned IP is indistinguishable from one the
// rate limiter caught.
type IPBanCreateHandler struct{}

// @Summary     Manually ban an IP
// @Tags        security
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       body body security_entity.IPBanRequest true "Ban request"
// @Success     201 {object} security_entity.IPBanSuccess
// @Failure     400,401,403,500 {object} response.ErrorBody
// @Router      /admin/security/ip-bans [post]
func (h *IPBanCreateHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	var req security_entity.IPBanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	defer r.Body.Close()

	req.IP = strings.TrimSpace(req.IP)
	req.Reason = strings.TrimSpace(req.Reason)
	if !validateIP(req.IP) {
		response.ErrorResponse(w, http.StatusBadRequest, "ip must be a valid IPv4 or IPv6 address")
		return
	}
	if req.Reason == "" {
		req.Reason = "manually banned by admin"
	}
	if req.DurationMinutes <= 0 {
		response.ErrorResponse(w, http.StatusBadRequest, "duration_minutes must be > 0")
		return
	}

	guard, ok := resolveIPGuard(reqCtx)
	if !ok {
		return
	}

	var createdBy *string
	if id := callerAdminUserID(r); id != "" {
		createdBy = &id
	}

	duration := time.Duration(req.DurationMinutes) * time.Minute
	if err := guard.Ban(req.IP, "manual", req.Reason, duration, createdBy); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	bans, err := guard.ListBans()
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	for _, b := range bans {
		if b.IP == req.IP {
			response.SuccessResponse(w, http.StatusCreated, b)
			return
		}
	}
	// Should be unreachable — Ban() just wrote this row — but avoid a
	// silent empty 201 if the read-back ever races with something.
	response.SuccessResponse(w, http.StatusCreated, security_entity.IPBan{IP: req.IP, Tier: "manual", Reason: req.Reason})
}

// IPBanDeleteHandler serves DELETE
// /api/v1/admin/security/ip-bans/{ip:<value>} — a manual unban. See
// this package's doc comment for the {key:value} path convention.
type IPBanDeleteHandler struct{}

// @Summary     Unban an IP
// @Tags        security
// @Produce     json
// @Security    BearerAuth
// @Param       ip path string true "IP address, as {ip:<value>}"
// @Success     200 {object} security_entity.StatusSuccess
// @Failure     400,401,403,404,500 {object} response.ErrorBody
// @Router      /admin/security/ip-bans/{ip} [delete]
func (h *IPBanDeleteHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	key, value := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if key != "ip" || value == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "ip is required")
		return
	}

	guard, ok := resolveIPGuard(reqCtx)
	if !ok {
		return
	}
	if err := guard.Unban(value); err != nil {
		response.ErrorResponse(w, http.StatusNotFound, "ip is not currently banned")
		return
	}
	response.SuccessResponse(w, http.StatusOK, security_entity.StatusResponse{Status: "unbanned"})
}

// IPBanAccountsHandler serves GET
// /api/v1/admin/security/ip-bans/{ip:<value>}/accounts — summarizes
// which accounts a banned IP tried to log in as. AdminAttempts is
// populated only when the caller also holds
// admin:security:login-log:read; ApplicationAttempts only when the
// caller also holds applications:login_log:read — both independent
// of the admin:security:ipbans:read scope gating this endpoint
// itself. Each is nil, not an empty slice, when the caller lacks the
// corresponding scope, so the frontend can distinguish "not
// authorized" from "authorized, nothing found". See
// plan/ip-ban-accounts/plan.md.
type IPBanAccountsHandler struct{}

// @Summary     List accounts a banned IP attempted to log into
// @Description Summarizes failed login attempts from this IP — admin console and every
// @Description application — username, attempt count, last-attempt time. admin_attempts is
// @Description omitted (null) without admin:security:login-log:read; application_attempts without
// @Description applications:login_log:read.
// @Tags        security
// @Produce     json
// @Security    BearerAuth
// @Param       ip path string true "Banned IP address, as {ip:<value>}"
// @Success     200 {object} security_entity.IPBanAccountsSuccess
// @Failure     400,401,403,500 {object} response.ErrorBody
// @Router      /admin/security/ip-bans/{ip}/accounts [get]
func (h *IPBanAccountsHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	key, ip := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if key != "ip" || ip == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "ip is required")
		return
	}

	bag, ok := reqCtx.GetDI().(*di.ContextBag)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "DI context has unexpected type")
		return
	}

	var result security_entity.IPBanAccountsResponse

	checker := scopecheck.NewChecker()
	hasLoginLogRead, _ := checker.HasScope(r.Header, "admin:security:login-log:read")
	if hasLoginLogRead {
		handle := bag.GetAdminLoginHandle()
		if handle == nil {
			response.ErrorResponse(w, http.StatusInternalServerError, "admin login-log database not available")
			return
		}
		summaries, err := loginlog_query.NewAdminLoginQueryRepo(handle).ListFailedUsernamesForIP(ip)
		if err != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		result.AdminAttempts = make([]security_entity.FailedUsernameSummary, 0, len(summaries))
		for _, s := range summaries {
			result.AdminAttempts = append(result.AdminAttempts, security_entity.FailedUsernameSummary{
				Username:      s.Username,
				AccountID:     s.AdminUserID,
				Attempts:      s.Attempts,
				LastAttemptAt: s.LastAttemptAt,
			})
		}
	}

	hasAppLoginLogRead, _ := checker.HasScope(r.Header, "applications:login_log:read")
	if hasAppLoginLogRead {
		appAttempts, err := listApplicationFailedUsernamesForIP(bag, ip)
		if err != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		result.ApplicationAttempts = appAttempts
	}

	response.SuccessResponse(w, http.StatusOK, result)
}

// listApplicationFailedUsernamesForIP fans out across every
// currently-provisioned application's own login-log DB, collecting
// only the applications where ip has at least one recorded failure.
// O(known applications) per call — acceptable at the scale a single
// admin console realistically has; would need reconsidering (caching
// KnownAppIDs, requiring an org filter) at a much larger scale. See
// plan/ip-ban-accounts/plan.md's flagged tradeoff.
func listApplicationFailedUsernamesForIP(bag *di.ContextBag, ip string) ([]security_entity.ApplicationFailedUsernameSummary, error) {
	loginlogRaw, ok := bag.Get(app_loginlog_dbregistry.ContextBagKey)
	if !ok {
		return []security_entity.ApplicationFailedUsernameSummary{}, nil
	}
	loginlogReg, ok := loginlogRaw.(*app_loginlog_dbregistry.Registry)
	if !ok {
		return []security_entity.ApplicationFailedUsernameSummary{}, nil
	}
	usersRaw, ok := bag.Get(users_dbregistry.ContextBagKey)
	if !ok {
		return []security_entity.ApplicationFailedUsernameSummary{}, nil
	}
	usersReg, ok := usersRaw.(*users_dbregistry.OrgUserDBRegistry)
	if !ok {
		return []security_entity.ApplicationFailedUsernameSummary{}, nil
	}

	out := []security_entity.ApplicationFailedUsernameSummary{}
	for _, appID := range loginlogReg.KnownAppIDs() {
		handle, err := loginlogReg.For(appID)
		if err != nil {
			continue
		}
		summaries, err := app_loginlog_query.NewApplicationLoginQueryRepo(handle).ListFailedUsernamesForIP(ip)
		if err != nil || len(summaries) == 0 {
			continue
		}

		title := appID
		if orgDB, _, err := orgrouter.OrgDBForApp(usersReg, appID); err == nil {
			var t string
			if orgDB.QueryRow(`SELECT title FROM applications WHERE id = ?`, appID).Scan(&t) == nil && t != "" {
				title = t
			}
		}

		for _, s := range summaries {
			out = append(out, security_entity.ApplicationFailedUsernameSummary{
				ApplicationID:    appID,
				ApplicationTitle: title,
				Username:         s.Username,
				AccountID:        s.ApplicationUserID,
				Attempts:         s.Attempts,
				LastAttemptAt:    s.LastAttemptAt,
			})
		}
	}
	return out, nil
}
