// Package handler serves the admin IP-search API under
// /api/v1/admin/security/geoip/search. See
// plan/geoip-enrichment/plan.md's "Extension: IP search" section.
package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// SearchHandler serves GET /api/v1/admin/security/geoip/search.
type SearchHandler struct{}

// @Summary     Search for an IP, full or partial
// @Description Given a complete IP, returns a live GeoIP lookup for it. Given a
// @Description partial/incomplete input, returns up to `limit` known IPs (from
// @Description recorded attack/scan history) starting with that prefix, each with
// @Description its own live GeoIP lookup. See plan/geoip-enrichment/plan.md's
// @Description "Extension: IP search" section.
// @Tags        security-geoip
// @Produce     json
// @Security    BearerAuth
// @Param       q     query string true  "Full or partial IP address"
// @Param       limit query int    false "Max autocomplete suggestions (default 10, max 25)"
// @Success     200 {object} handler.IPSearchSuccess
// @Failure     400,401,403,500 {object} response.ErrorBody
// @Router      /admin/security/geoip/search [get]
func (h *SearchHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	searcher, ok := resolveSearcher(reqCtx)
	if !ok {
		return
	}

	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "q must not be empty")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	results, err := searcher.Search(q, limit)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	resp := IPSearchResponse{Query: q, Results: make([]IPSearchResult, 0, len(results))}
	for _, res := range results {
		resp.Results = append(resp.Results, IPSearchResult{
			IP:          res.IP,
			Matched:     res.Matched,
			CountryCode: res.CountryCode,
			Country:     res.Country,
			ASN:         res.ASN,
			ASOrg:       res.ASOrg,
		})
	}
	response.SuccessResponse(w, http.StatusOK, resp)
}
