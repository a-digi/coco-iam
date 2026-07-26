package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	security_entity "github.com/a-digi/coco-iam/src/admin/security/entity"
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
