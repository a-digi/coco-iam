package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/a-digi/coco-iam/config/di"
	attacks_entity "github.com/a-digi/coco-iam/src/admin/security/attacks/entity"
	attacks_persistent "github.com/a-digi/coco-sec/ipguard/repository/persistent"
	attacks_query "github.com/a-digi/coco-sec/ipguard/repository/query"
	"github.com/a-digi/coco-sec/geoip"
	"github.com/a-digi/coco-lift/resource/uri"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// FetchGeoIPHandler serves POST
// /api/v1/admin/security/attacks/{id:<value>}/geoip — backfills
// geoip_info for an episode recorded before the GeoIP feature
// existed, via a live lookup against the current geoip.db. See
// plan/geoip-enrichment/plan.md's "Extension: backfill GeoIP for
// historical attack episodes" section.
type FetchGeoIPHandler struct{}

// @Summary     Backfill GeoIP data for an attack episode
// @Description Runs a live GeoIP lookup for this episode's IP and persists the result into
// @Description geoip_info — for historical episodes recorded before the GeoIP feature existed.
// @Description Fails with 409 if geoip_info is already set (never overwrites an existing frozen
// @Description snapshot) or if GeoIP is not currently enabled. matched:false (still 200) means the
// @Description IP is loopback/private or has no current GeoLite2 coverage — nothing is persisted,
// @Description so the button stays available for a future retry.
// @Tags        security
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Attack episode id, as {id:<value>}"
// @Success     200 {object} attacks_entity.FetchGeoIPSuccess
// @Failure     401,403,404,409,500 {object} response.ErrorBody
// @Router      /admin/security/attacks/{id}/geoip [post]
func (h *FetchGeoIPHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	key, value := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if key != "id" || value == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "attack id is required")
		return
	}

	query, ok := resolveAttackQuery(reqCtx)
	if !ok {
		return
	}
	attack, err := query.FindAttack(value)
	if err != nil {
		if errors.Is(err, attacks_query.ErrAttackNotFound) {
			response.ErrorResponse(w, http.StatusNotFound, "attack episode not found")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	// The real safety property, not just the frontend hiding the
	// button — a re-fetch on an already-populated episode would
	// silently replace historical truth with today's IP-allocation
	// data, which may have genuinely changed since.
	if attack.GeoIPInfo != "" {
		response.ErrorResponse(w, http.StatusConflict, "geoip_info is already set for this episode")
		return
	}

	geo, ok := resolveGeoIPLookup(reqCtx)
	if !ok {
		return
	}
	if !geo.Enabled() {
		response.ErrorResponse(w, http.StatusConflict, "geoip is not currently enabled")
		return
	}

	// Mirrors ipguard's own creation-time check — a loopback/private
	// address is never going to resolve, regardless of what geoip.db
	// currently holds.
	if geoip.IsLoopbackOrPrivate(attack.IP) {
		response.SuccessResponse(w, http.StatusOK, attacks_entity.FetchGeoIPResponse{Matched: false})
		return
	}

	info, matched := geo.Lookup(attack.IP)
	if !matched {
		response.SuccessResponse(w, http.StatusOK, attacks_entity.FetchGeoIPResponse{Matched: false})
		return
	}

	raw, err := json.Marshal(info)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	geoInfo := string(raw)

	persist, ok := resolveAttackPersist(reqCtx)
	if !ok {
		return
	}
	if err := persist.SetGeoIPInfo(value, geoInfo); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.SuccessResponse(w, http.StatusOK, attacks_entity.FetchGeoIPResponse{Matched: true, GeoIPInfo: geoInfo})
}

// resolveAttackPersist builds a persistent repo against the same
// ip-attacks.db handle resolveAttackQuery reads from.
func resolveAttackPersist(reqCtx request.RequestContext) (*attacks_persistent.AttackPersistentRepo, bool) {
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
	return attacks_persistent.NewAttackPersistentRepo(handle), true
}

// resolveGeoIPLookup returns the shared geoip.Lookup instance
// constructed once at boot (see config/routes/routes.go).
func resolveGeoIPLookup(reqCtx request.RequestContext) (geoip.Lookup, bool) {
	w := reqCtx.GetWriter()
	bag, ok := reqCtx.GetDI().(*di.ContextBag)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "DI context has unexpected type")
		return nil, false
	}
	geo := bag.GetGeoIP()
	if geo == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "geoip lookup not available")
		return nil, false
	}
	return geo, true
}
