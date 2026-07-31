// Package handler serves the read-only admin attack-history API
// under /api/v1/admin/security/attacks*. See
// plan/ip-abuse-protection/plan.md sections 10 and 13.
//
// Unlike the ban/allowlist handlers (api/src/admin/security/handler),
// these talk to the attacks repository directly rather than through
// IPGuardSecurityLayer — attack history is pure historical read data
// with no shared-mutable in-process state to keep in sync, and this
// API has no write endpoints at all.
package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/a-digi/coco-iam/config/di"
	attacks_entity "github.com/a-digi/coco-iam/src/admin/security/attacks/entity"
	cocosecentity "github.com/a-digi/coco-sec/ipguard/entity"
	attacks_query "github.com/a-digi/coco-sec/ipguard/repository/query"
	"github.com/a-digi/coco-lift/resource/uri"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

const (
	defaultLimit = 50
	maxLimit     = 500
)

// AttackListHandler serves GET /api/v1/admin/security/attacks.
// Query parameters:
//
//	ip=<addr>       exact match
//	tier=<tier>     exact match ("global" | "sensitive" | "manual")
//	active=true     only currently-open episodes (ended_at IS NULL)
//	limit=<n>       max 500, default 50
//	offset=<n>
type AttackListHandler struct{}

// @Summary     List IP-abuse attack episodes
// @Description Lists historical (and currently ongoing) attack episodes, newest first.
// @Tags        security
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} attacks_entity.AttackListSuccess
// @Failure     401,403,500 {object} response.ErrorBody
// @Router      /admin/security/attacks [get]
func (h *AttackListHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	query, ok := resolveAttackQuery(reqCtx)
	if !ok {
		return
	}

	q := r.URL.Query()
	filter := attacks_query.ListFilter{
		IP:         strings.TrimSpace(q.Get("ip")),
		Tier:       strings.TrimSpace(q.Get("tier")),
		ActiveOnly: q.Get("active") == "true",
		Limit:      clampLimit(parseIntOr(q.Get("limit"), defaultLimit)),
		Offset:     maxInt(parseIntOr(q.Get("offset"), 0), 0),
	}

	attacks, err := query.ListAttacks(filter)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if attacks == nil {
		attacks = []cocosecentity.Attack{}
	}
	total, err := query.CountAttacks(filter)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.SuccessResponse(w, http.StatusOK, attacks_entity.AttackListResponse{
		Attacks: attacks,
		Total:   total,
		Limit:   filter.Limit,
		Offset:  filter.Offset,
	})
}

// AttackDetailHandler serves GET
// /api/v1/admin/security/attacks/{id:<value>} — one episode plus its
// per-endpoint breakdown. See admin/security/handler's doc comment
// for the {key:value} path convention.
type AttackDetailHandler struct{}

// @Summary     Get one attack episode
// @Tags        security
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Attack episode id, as {id:<value>}"
// @Success     200 {object} attacks_entity.AttackDetailSuccess
// @Failure     400,401,403,404,500 {object} response.ErrorBody
// @Router      /admin/security/attacks/{id} [get]
func (h *AttackDetailHandler) ServeHTTP(reqCtx request.RequestContext) {
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

	targets, err := query.ListTargets(value)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if targets == nil {
		targets = []cocosecentity.AttackTarget{}
	}

	response.SuccessResponse(w, http.StatusOK, attacks_entity.AttackDetailResponse{
		Attack:  *attack,
		Targets: targets,
	})
}

// resolveAttackQuery builds a query repo against the separate
// ip-attacks.db, resolved via the DI ContextBag. Reads through the
// same *dbhandle.Handle ipguard writes through, so this keeps reading
// the live generation across the archiver rotating the file out from
// under it (see plan/ip-attacks-db-archiving/plan.md) instead of
// freezing on a stale connection. Writes a 500 response itself and
// returns ok=false if unavailable.
func resolveAttackQuery(reqCtx request.RequestContext) (*attacks_query.AttackQueryRepo, bool) {
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
	return attacks_query.NewAttackQueryRepo(handle), true
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
