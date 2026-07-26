package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	security_entity "github.com/a-digi/coco-iam/src/admin/security/entity"
	"github.com/a-digi/coco-lift/resource/uri"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// IPAllowlistListHandler serves GET /api/v1/admin/security/ip-allowlist.
type IPAllowlistListHandler struct{}

// @Summary     List allowlisted IPs
// @Tags        security
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} security_entity.IPAllowlistListSuccess
// @Failure     401,403,500 {object} response.ErrorBody
// @Router      /admin/security/ip-allowlist [get]
func (h *IPAllowlistListHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	guard, ok := resolveIPGuard(reqCtx)
	if !ok {
		return
	}
	entries, err := guard.ListAllowlist()
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if entries == nil {
		entries = []security_entity.IPAllowlistEntry{}
	}
	response.SuccessResponse(w, http.StatusOK, entries)
}

// IPAllowlistCreateHandler serves POST /api/v1/admin/security/ip-allowlist
// — exempts an IP from rate limiting and bans entirely. Intended for
// legitimate shared-IP traffic (NAT/office egress) that would
// otherwise trip the global tier.
type IPAllowlistCreateHandler struct{}

// @Summary     Add an IP to the allowlist
// @Tags        security
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       body body security_entity.IPAllowlistRequest true "Allowlist request"
// @Success     201 {object} security_entity.IPAllowlistEntrySuccess
// @Failure     400,401,403,500 {object} response.ErrorBody
// @Router      /admin/security/ip-allowlist [post]
func (h *IPAllowlistCreateHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	var req security_entity.IPAllowlistRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	defer r.Body.Close()

	req.IP = strings.TrimSpace(req.IP)
	req.Note = strings.TrimSpace(req.Note)
	if !validateIP(req.IP) {
		response.ErrorResponse(w, http.StatusBadRequest, "ip must be a valid IPv4 or IPv6 address")
		return
	}

	guard, ok := resolveIPGuard(reqCtx)
	if !ok {
		return
	}
	createdBy := callerAdminUserID(r)

	if err := guard.AllowIP(req.IP, req.Note, createdBy); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	entries, err := guard.ListAllowlist()
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	for _, e := range entries {
		if e.IP == req.IP {
			response.SuccessResponse(w, http.StatusCreated, e)
			return
		}
	}
	response.SuccessResponse(w, http.StatusCreated, security_entity.IPAllowlistEntry{IP: req.IP, Note: req.Note, CreatedBy: createdBy})
}

// IPAllowlistDeleteHandler serves DELETE
// /api/v1/admin/security/ip-allowlist/{ip:<value>}.
type IPAllowlistDeleteHandler struct{}

// @Summary     Remove an IP from the allowlist
// @Tags        security
// @Produce     json
// @Security    BearerAuth
// @Param       ip path string true "IP address, as {ip:<value>}"
// @Success     200 {object} security_entity.StatusSuccess
// @Failure     400,401,403,404,500 {object} response.ErrorBody
// @Router      /admin/security/ip-allowlist/{ip} [delete]
func (h *IPAllowlistDeleteHandler) ServeHTTP(reqCtx request.RequestContext) {
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
	if err := guard.DisallowIP(value); err != nil {
		response.ErrorResponse(w, http.StatusNotFound, "ip is not in the allowlist")
		return
	}
	response.SuccessResponse(w, http.StatusOK, security_entity.StatusResponse{Status: "removed"})
}
