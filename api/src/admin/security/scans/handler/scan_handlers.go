// Package handler serves the read-only admin port-scan-history API
// under /api/v1/admin/security/scans*. See
// plan/port-scan-detection/plan.md Phase B.
package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/a-digi/coco-iam/config/di"
	scans_entity "github.com/a-digi/coco-iam/src/admin/security/scans/entity"
	scans_query "github.com/a-digi/coco-iam/src/admin/security/scans/repository/query"
	"github.com/a-digi/coco-lift/resource/uri"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

const (
	defaultLimit = 50
	maxLimit     = 500
)

// ScanListHandler serves GET /api/v1/admin/security/scans. Query
// parameters:
//
//	ip=<addr>     exact match
//	active=true   only currently-open episodes (ended_at IS NULL)
//	limit=<n>     max 500, default 50
//	offset=<n>
type ScanListHandler struct{}

// @Summary     List port-scan episodes
// @Description Lists historical (and currently ongoing) port-scan episodes, newest first.
// @Tags        security
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} scans_entity.ScanListSuccess
// @Failure     401,403,500 {object} response.ErrorBody
// @Router      /admin/security/scans [get]
func (h *ScanListHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	query, ok := resolveScanQuery(reqCtx)
	if !ok {
		return
	}

	q := r.URL.Query()
	filter := scans_query.ListFilter{
		IP:         strings.TrimSpace(q.Get("ip")),
		ActiveOnly: q.Get("active") == "true",
		Limit:      clampLimit(parseIntOr(q.Get("limit"), defaultLimit)),
		Offset:     maxInt(parseIntOr(q.Get("offset"), 0), 0),
	}

	scans, err := query.ListScans(filter)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if scans == nil {
		scans = []scans_entity.Scan{}
	}
	total, err := query.CountScans(filter)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.SuccessResponse(w, http.StatusOK, scans_entity.ScanListResponse{
		Scans:  scans,
		Total:  total,
		Limit:  filter.Limit,
		Offset: filter.Offset,
	})
}

// ScanDetailHandler serves GET
// /api/v1/admin/security/scans/{id:<value>} — a single episode. See
// admin/security/handler's doc comment for the {key:value} path
// convention.
type ScanDetailHandler struct{}

// @Summary     Get one port-scan episode
// @Tags        security
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Scan episode id, as {id:<value>}"
// @Success     200 {object} scans_entity.ScanDetailSuccess
// @Failure     400,401,403,404,500 {object} response.ErrorBody
// @Router      /admin/security/scans/{id} [get]
func (h *ScanDetailHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	key, value := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if key != "id" || value == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "scan id is required")
		return
	}

	query, ok := resolveScanQuery(reqCtx)
	if !ok {
		return
	}

	scan, err := query.FindScan(value)
	if err != nil {
		if errors.Is(err, scans_query.ErrNotFound) {
			response.ErrorResponse(w, http.StatusNotFound, "scan episode not found")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.SuccessResponse(w, http.StatusOK, scan)
}

// resolveScanQuery builds a query repo against the separate
// ip-attacks.db, resolved via the DI ContextBag. Writes a 500
// response itself and returns ok=false if unavailable.
func resolveScanQuery(reqCtx request.RequestContext) (*scans_query.ScanQueryRepo, bool) {
	w := reqCtx.GetWriter()
	bag, ok := reqCtx.GetDI().(*di.ContextBag)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "DI context has unexpected type")
		return nil, false
	}
	handle := bag.GetIPAttacksHandle()
	if handle == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "ip-attacks database not available")
		return nil, false
	}
	return scans_query.NewScanQueryRepo(handle), true
}

func parseIntOr(s string, def int) int {
	if s == "" {
		return def
	}
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return def
}

func clampLimit(n int) int {
	if n <= 0 {
		return defaultLimit
	}
	if n > maxLimit {
		return maxLimit
	}
	return n
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
