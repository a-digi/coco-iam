// Package handler serves the read-only admin archive-history API
// under /api/v1/admin/security/archives*. See
// plan/ip-attacks-db-archiving/plan.md.
//
// Browsing INTO one archive reuses attacks_entity/attacks_query's
// exact response shapes — an archived generation has the identical
// ip_attacks/ip_attack_targets schema as the live one, so there's no
// reason to duplicate those types just because the data came from a
// rotated-out file instead of the live connection.
package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/a-digi/coco-iam/config/di"
	archives_entity "github.com/a-digi/coco-iam/src/admin/security/archives/entity"
	archives_query "github.com/a-digi/coco-iam/src/admin/security/archives/repository/query"
	attacks_entity "github.com/a-digi/coco-iam/src/admin/security/attacks/entity"
	attacks_query "github.com/a-digi/coco-iam/src/admin/security/attacks/repository/query"
	"github.com/a-digi/coco-lift/resource/uri"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

const (
	defaultLimit = 50
	maxLimit     = 500
)

// ArchiveListHandler serves GET /api/v1/admin/security/archives.
// Query parameters: limit=<n> (max 500, default 50), offset=<n>.
type ArchiveListHandler struct{}

// @Summary     List ip-attacks.db archives
// @Description Lists rotated-out ip-attacks.db generations, newest first.
// @Tags        security
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} archives_entity.ArchiveListSuccess
// @Failure     401,403,500 {object} response.ErrorBody
// @Router      /admin/security/archives [get]
func (h *ArchiveListHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	query, ok := resolveArchiveQuery(reqCtx)
	if !ok {
		return
	}

	q := r.URL.Query()
	limit := clampLimit(parseIntOr(q.Get("limit"), defaultLimit))
	offset := maxInt(parseIntOr(q.Get("offset"), 0), 0)

	archives, err := query.ListArchives(limit, offset)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if archives == nil {
		archives = []archives_entity.ArchiveSummary{}
	}
	total, err := query.CountArchives()
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.SuccessResponse(w, http.StatusOK, archives_entity.ArchiveListResponse{
		Archives: archives,
		Total:    total,
		Limit:    limit,
		Offset:   offset,
	})
}

// ArchiveDetailHandler serves GET
// /api/v1/admin/security/archives/{id:<value>} — registry metadata for
// one archive. See admin/security/handler's doc comment for the
// {key:value} path convention.
type ArchiveDetailHandler struct{}

// @Summary     Get one ip-attacks.db archive's metadata
// @Tags        security
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Archive id, as {id:<value>}"
// @Success     200 {object} archives_entity.ArchiveDetailSuccess
// @Failure     400,401,403,404,500 {object} response.ErrorBody
// @Router      /admin/security/archives/{id} [get]
func (h *ArchiveDetailHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	key, value := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if key != "id" || value == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "archive id is required")
		return
	}

	query, ok := resolveArchiveQuery(reqCtx)
	if !ok {
		return
	}

	archive, _, err := query.FindArchive(value)
	if err != nil {
		if errors.Is(err, archives_query.ErrNotFound) {
			response.ErrorResponse(w, http.StatusNotFound, "archive not found")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.SuccessResponse(w, http.StatusOK, archive)
}

// ArchiveAttacksListHandler serves GET
// /api/v1/admin/security/archives/{id:<value>}/attacks — the same
// filter/pagination shape as the live attacks list, read from the
// archived file instead. Only one bracketed path segment here
// ("attacks" is a literal suffix), so the existing single-pair
// extractor applies unchanged.
type ArchiveAttacksListHandler struct{}

// @Summary     List attack episodes within one archive
// @Tags        security
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Archive id, as {id:<value>}"
// @Success     200 {object} attacks_entity.AttackListSuccess
// @Failure     400,401,403,404,500 {object} response.ErrorBody
// @Router      /admin/security/archives/{id}/attacks [get]
func (h *ArchiveAttacksListHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	key, archiveID := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if key != "id" || archiveID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "archive id is required")
		return
	}

	archiveQuery, ok := resolveArchiveQuery(reqCtx)
	if !ok {
		return
	}
	_, filePath, err := archiveQuery.FindArchive(archiveID)
	if err != nil {
		if errors.Is(err, archives_query.ErrNotFound) {
			response.ErrorResponse(w, http.StatusNotFound, "archive not found")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	attacksRepo, archiveDB, err := archives_query.OpenArchivedAttacks(filePath)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer archiveDB.Close()

	q := r.URL.Query()
	filter := attacks_query.ListFilter{
		IP:         strings.TrimSpace(q.Get("ip")),
		Tier:       strings.TrimSpace(q.Get("tier")),
		ActiveOnly: q.Get("active") == "true",
		Limit:      clampLimit(parseIntOr(q.Get("limit"), defaultLimit)),
		Offset:     maxInt(parseIntOr(q.Get("offset"), 0), 0),
	}

	attacks, err := attacksRepo.ListAttacks(filter)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if attacks == nil {
		attacks = []attacks_entity.Attack{}
	}
	total, err := attacksRepo.CountAttacks(filter)
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

// ArchiveAttackDetailHandler serves GET
// /api/v1/admin/security/archives/{id:<value>}/attacks/{attackId:<value>}
// — a single archived episode plus its per-endpoint breakdown. This
// path has TWO bracketed segments, which
// uri.ExtractKeyAndValueFromURI can't handle (it returns only the
// first pair it finds) — extractBracedPairs below is a local,
// host-repo-only extension of that same per-segment parsing logic
// rather than a change to the vendored coco-lift package, since only
// this one route needs more than one placeholder.
type ArchiveAttackDetailHandler struct{}

// @Summary     Get one attack episode within one archive
// @Tags        security
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Archive id, as {id:<value>}"
// @Param       attackId path string true "Attack episode id, as {attackId:<value>}"
// @Success     200 {object} attacks_entity.AttackDetailSuccess
// @Failure     400,401,403,404,500 {object} response.ErrorBody
// @Router      /admin/security/archives/{id}/attacks/{attackId} [get]
func (h *ArchiveAttackDetailHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	pairs := extractBracedPairs(r.URL.Path)
	archiveID := pairs["id"]
	attackID := pairs["attackId"]
	if archiveID == "" || attackID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "archive id and attack id are required")
		return
	}

	archiveQuery, ok := resolveArchiveQuery(reqCtx)
	if !ok {
		return
	}
	_, filePath, err := archiveQuery.FindArchive(archiveID)
	if err != nil {
		if errors.Is(err, archives_query.ErrNotFound) {
			response.ErrorResponse(w, http.StatusNotFound, "archive not found")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	attacksRepo, archiveDB, err := archives_query.OpenArchivedAttacks(filePath)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer archiveDB.Close()

	attack, err := attacksRepo.FindAttack(attackID)
	if err != nil {
		if errors.Is(err, attacks_query.ErrNotFound) {
			response.ErrorResponse(w, http.StatusNotFound, "attack episode not found")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	targets, err := attacksRepo.ListTargets(attackID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if targets == nil {
		targets = []attacks_entity.AttackTarget{}
	}

	response.SuccessResponse(w, http.StatusOK, attacks_entity.AttackDetailResponse{
		Attack:  *attack,
		Targets: targets,
	})
}

// extractBracedPairs returns every {key:value}-shaped path segment in
// path, keyed by key — the same per-segment parsing
// uri.ExtractKeyAndValueFromURI uses, just collecting all matches
// instead of returning on the first.
func extractBracedPairs(path string) map[string]string {
	out := make(map[string]string)
	for _, seg := range uri.SplitURIPath(path) {
		if !strings.HasPrefix(seg, "{") || !strings.HasSuffix(seg, "}") || strings.HasPrefix(seg, "{res:") {
			continue
		}
		inner := seg[1 : len(seg)-1]
		parts := strings.SplitN(inner, ":", 2)
		if len(parts) == 2 {
			out[parts[0]] = parts[1]
		}
	}
	return out
}

// resolveArchiveQuery builds a query repo against the main DB (which
// holds ip_attacks_archives — unlike ip-attacks.db itself, the main DB
// is never rotated). Writes a 500 response itself and returns
// ok=false if unavailable.
func resolveArchiveQuery(reqCtx request.RequestContext) (*archives_query.ArchiveQueryRepo, bool) {
	w := reqCtx.GetWriter()
	bag, ok := reqCtx.GetDI().(*di.ContextBag)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "DI context has unexpected type")
		return nil, false
	}
	manager := bag.GetDatabaseManager()
	if manager == nil || manager.Connector == nil || manager.Connector.DB == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "database not available")
		return nil, false
	}
	return archives_query.NewArchiveQueryRepo(manager.Connector.DB), true
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
