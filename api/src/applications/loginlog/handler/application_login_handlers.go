// Package handler serves the read-only per-application login-log API
// under /api/v1/applications/{res:applications}/{id}/login-log*.
// Mirrors admin/security/loginlog/handler exactly, adapted to this
// domain's per-application (rather than single global) database.
// See plan/login-audit-log/plan.md Step 8.
package handler

import (
	"database/sql"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	archives_entity "github.com/a-digi/coco-iam/src/admin/security/archives/entity"
	applications_admin "github.com/a-digi/coco-iam/src/applications/admin"
	loginlog_dbregistry "github.com/a-digi/coco-iam/src/applications/loginlog/dbregistry"
	loginlog_entity "github.com/a-digi/coco-iam/src/applications/loginlog/entity"
	loginlog_query "github.com/a-digi/coco-iam/src/applications/loginlog/repository/query"
	users_dbregistry "github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-iam/src/orgrouter"
	"github.com/a-digi/coco-lift/resource/uri"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

const (
	defaultLimit = 50
	maxLimit     = 500
)

// bagGetter is the minimal slice of di.ContextBag the resolvers need —
// same pattern apicredentials/admin's common.go uses, so this package
// never needs to import config/di directly.
type bagGetter interface {
	Get(key string) (interface{}, bool)
}

// ErrAppNotFound signals the URL's application id doesn't map to any
// known org.
var ErrAppNotFound = errors.New("application login-log: application not found")

// appIDFromPath pulls the `{id:<uuid>}` segment out of the URL —
// matches apicredentials/admin's appIDFromPath.
func appIDFromPath(reqCtx request.RequestContext) string {
	key, value := uri.ExtractKeyAndValueFromURI(reqCtx.GetRequest().URL.Path)
	if key != "id" {
		return ""
	}
	return strings.TrimSpace(value)
}

// resolveOrgIDForApp scans per-org DBs to find the one that owns
// appID — the same ownership check apicredentials/admin's
// resolveOrgIDForApp performs, reused verbatim rather than
// re-derived, per plan/login-audit-log/plan.md Step 8's own flagged
// security note. This is what prevents one application's admin from
// browsing another application's login history just by guessing an
// id belonging to a different org.
func resolveOrgIDForApp(reqCtx request.RequestContext, appID string) (string, error) {
	bag, ok := reqCtx.GetDI().(bagGetter)
	if !ok {
		return "", errors.New("application login-log: DI context not keyed")
	}
	raw, ok := bag.Get(users_dbregistry.ContextBagKey)
	if !ok {
		return "", errors.New("application login-log: users registry not in DI")
	}
	reg, ok := raw.(*users_dbregistry.OrgUserDBRegistry)
	if !ok {
		return "", errors.New("application login-log: users registry type mismatch")
	}
	_, orgID, err := orgrouter.OrgDBForApp(reg, appID)
	if err != nil {
		return "", ErrAppNotFound
	}
	return orgID, nil
}

// selfHealProvisioning attempts to provision appID's login-log DB on
// first read, covering two populations that never got provisioned at
// application-creation time: (a) applications that have a slug but
// whose Provision call failed transiently, and (b) applications with
// no slug at all — either because they predate the slug column
// entirely, or ReserveApplicationSlug itself failed at creation time.
// Both cases are silently unrecoverable otherwise: SweepExisting (run
// at every server boot) only re-opens a <slug>_login.db that already
// exists on disk, it never creates one from scratch. Best-effort,
// like every other step in this chain — any failure here just means
// the caller keeps seeing the original 404. See
// plan/login-log-provisioning-selfheal/plan.md.
func selfHealProvisioning(reqCtx request.RequestContext, registry *loginlog_dbregistry.Registry, appID string) bool {
	bag, ok := reqCtx.GetDI().(bagGetter)
	if !ok {
		return false
	}
	raw, ok := bag.Get(users_dbregistry.ContextBagKey)
	if !ok {
		return false
	}
	reg, ok := raw.(*users_dbregistry.OrgUserDBRegistry)
	if !ok {
		return false
	}
	orgDB, orgID, err := orgrouter.OrgDBForApp(reg, appID)
	if err != nil {
		return false
	}

	var slug, title sql.NullString
	if err := orgDB.QueryRow(`SELECT slug, title FROM applications WHERE id = ?`, appID).Scan(&slug, &title); err != nil {
		return false
	}

	resolvedSlug := slug.String
	if resolvedSlug == "" {
		resolvedSlug = applications_admin.ReserveApplicationSlug(reqCtx, appID, orgID, title.String)
		if resolvedSlug == "" {
			return false
		}
		if _, err := orgDB.Exec(`UPDATE applications SET slug = ? WHERE id = ?`, resolvedSlug, appID); err != nil {
			return false
		}
	}

	return registry.Provision(appID, orgID, resolvedSlug) == nil
}

// AppLoginLogListHandler serves GET
// /api/v1/applications/{res:applications}/{id}/login-log. Query
// parameters:
//
//	username=<name>              exact match
//	application_user_id=<id>     exact match
//	success=true|false           exact match
//	ip=<addr>                    exact match
//	from=<RFC3339>                created_at >= from
//	to=<RFC3339>                  created_at <= to
//	limit=<n>                    max 500, default 50
//	offset=<n>
type AppLoginLogListHandler struct{}

// @Summary     List application end-user login attempts
// @Description Lists login attempts (success and failure) for one application's own end-users, newest first.
// @Tags        applications
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Application ID"
// @Success     200 {object} loginlog_entity.ApplicationLoginAttemptListSuccess
// @Failure     400,401,403,404,500 {object} response.ErrorBody
// @Router      /applications/{id}/login-log [get]
func (h *AppLoginLogListHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	appID := appIDFromPath(reqCtx)
	if appID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "missing application id")
		return
	}
	if _, err := resolveOrgIDForApp(reqCtx, appID); err != nil {
		writeAppLookupError(w, err)
		return
	}

	registry := resolveAppLoginLogRegistry(reqCtx)
	if registry == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "application login-log registry not available")
		return
	}
	handle, err := registry.For(appID)
	if err != nil {
		// Self-heal: this application was never provisioned (predates
		// the provisioning hook, or a transient failure at creation
		// time swallowed the error — both are best-effort at creation
		// time and never retried since). Try once, on this read, to
		// fix it permanently rather than failing forever. See
		// plan/login-log-provisioning-selfheal/plan.md.
		if selfHealProvisioning(reqCtx, registry, appID) {
			handle, err = registry.For(appID)
		}
		if err != nil {
			response.ErrorResponse(w, http.StatusNotFound, "login log not provisioned for this application")
			return
		}
	}
	query := loginlog_query.NewApplicationLoginQueryRepo(handle)

	filter := filterFromQuery(r.URL.Query())

	attempts, err := query.ListAttempts(filter)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if attempts == nil {
		attempts = []loginlog_entity.ApplicationLoginAttempt{}
	}
	total, err := query.CountAttempts(filter)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.SuccessResponse(w, http.StatusOK, loginlog_entity.ApplicationLoginAttemptListResponse{
		Attempts: attempts,
		Total:    total,
		Limit:    filter.Limit,
		Offset:   filter.Offset,
	})
}

// AppLoginLogArchiveListHandler serves GET
// /api/v1/applications/{res:applications}/{id}/login-log/archives.
type AppLoginLogArchiveListHandler struct{}

// @Summary     List an application's login-log archives
// @Description Lists rotated-out <slug>_login.db generations for one application, newest first.
// @Tags        applications
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Application ID"
// @Success     200 {object} archives_entity.ArchiveListSuccess
// @Failure     400,401,403,404,500 {object} response.ErrorBody
// @Router      /applications/{id}/login-log/archives [get]
func (h *AppLoginLogArchiveListHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	appID := appIDFromPath(reqCtx)
	if appID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "missing application id")
		return
	}
	archiveQuery, ok := resolveArchiveQuery(reqCtx, appID)
	if !ok {
		return
	}

	q := r.URL.Query()
	limit := clampLimit(parseIntOr(q.Get("limit"), defaultLimit))
	offset := maxInt(parseIntOr(q.Get("offset"), 0), 0)

	archives, err := archiveQuery.ListArchives(limit, offset)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if archives == nil {
		archives = []archives_entity.ArchiveSummary{}
	}
	total, err := archiveQuery.CountArchives()
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

// AppLoginLogArchiveAttemptsHandler serves GET
// /api/v1/applications/{res:applications}/{id}/login-log/archives/{archiveId}/attempts
// — the same filter/pagination shape as the live list, read from the
// archived file instead.
type AppLoginLogArchiveAttemptsHandler struct{}

// @Summary     List login attempts within one application login-log archive
// @Tags        applications
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Application ID"
// @Param       archiveId path string true "Archive ID, as {archiveId:<value>}"
// @Success     200 {object} loginlog_entity.ApplicationLoginAttemptListSuccess
// @Failure     400,401,403,404,500 {object} response.ErrorBody
// @Router      /applications/{id}/login-log/archives/{archiveId}/attempts [get]
func (h *AppLoginLogArchiveAttemptsHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	pairs := extractBracedPairs(r.URL.Path)
	appID := pairs["id"]
	archiveID := pairs["archiveId"]
	if appID == "" || archiveID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "application id and archive id are required")
		return
	}

	archiveQuery, ok := resolveArchiveQuery(reqCtx, appID)
	if !ok {
		return
	}
	_, filePath, err := archiveQuery.FindArchive(archiveID)
	if err != nil {
		if errors.Is(err, loginlog_query.ErrArchiveNotFound) {
			response.ErrorResponse(w, http.StatusNotFound, "archive not found")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	attemptsRepo, archiveDB, err := loginlog_query.OpenArchivedAttempts(filePath)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer archiveDB.Close()

	filter := filterFromQuery(r.URL.Query())

	attempts, err := attemptsRepo.ListAttempts(filter)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if attempts == nil {
		attempts = []loginlog_entity.ApplicationLoginAttempt{}
	}
	total, err := attemptsRepo.CountAttempts(filter)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.SuccessResponse(w, http.StatusOK, loginlog_entity.ApplicationLoginAttemptListResponse{
		Attempts: attempts,
		Total:    total,
		Limit:    filter.Limit,
		Offset:   filter.Offset,
	})
}

// extractBracedPairs returns every {key:value}-shaped path segment in
// path, keyed by key — mirrors
// admin/security/archives/handler.extractBracedPairs, needed here
// too since this route has two bracketed segments
// (uri.ExtractKeyAndValueFromURI only returns the first).
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

// resolveArchiveQuery resolves appID's owning org, opens that org's
// users.db (which holds application_login_archives — unlike a
// <slug>_login.db, never itself rotated), and builds a query repo
// scoped to appID. Writes an error response and returns ok=false on
// any failure.
func resolveArchiveQuery(reqCtx request.RequestContext, appID string) (*loginlog_query.ApplicationLoginArchiveQueryRepo, bool) {
	w := reqCtx.GetWriter()
	orgID, err := resolveOrgIDForApp(reqCtx, appID)
	if err != nil {
		writeAppLookupError(w, err)
		return nil, false
	}

	bag, ok := reqCtx.GetDI().(bagGetter)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "DI context has unexpected type")
		return nil, false
	}
	raw, ok := bag.Get(users_dbregistry.ContextBagKey)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "users registry not available")
		return nil, false
	}
	reg, ok := raw.(*users_dbregistry.OrgUserDBRegistry)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "users registry has unexpected type")
		return nil, false
	}
	orgDB, err := orgrouter.ForOrg(reg, orgID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return nil, false
	}
	return loginlog_query.NewApplicationLoginArchiveQueryRepo(orgDB, appID), true
}

// resolveAppLoginLogRegistry fetches the per-application login-log db
// registry from the DI bag. Returns nil on failure — callers write
// their own error response, matching the resolveCredRegistry
// convention in apicredentials/admin/common.go.
func resolveAppLoginLogRegistry(reqCtx request.RequestContext) *loginlog_dbregistry.Registry {
	bag, ok := reqCtx.GetDI().(bagGetter)
	if !ok {
		return nil
	}
	raw, ok := bag.Get(loginlog_dbregistry.ContextBagKey)
	if !ok {
		return nil
	}
	reg, _ := raw.(*loginlog_dbregistry.Registry)
	return reg
}

func writeAppLookupError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrAppNotFound) {
		response.ErrorResponse(w, http.StatusNotFound, "application not found")
		return
	}
	response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
}

// filterFromQuery builds a ListFilter from the request's query
// string — shared by the live list and the archive-browse list, since
// both accept the identical filter/pagination shape.
func filterFromQuery(q url.Values) loginlog_query.ListFilter {
	return loginlog_query.ListFilter{
		Username:          strings.TrimSpace(q.Get("username")),
		ApplicationUserID: strings.TrimSpace(q.Get("application_user_id")),
		Success:           parseBoolFilter(q.Get("success")),
		IP:                strings.TrimSpace(q.Get("ip")),
		From:              parseTimeFilter(q.Get("from")),
		To:                parseTimeFilter(q.Get("to")),
		Limit:             clampLimit(parseIntOr(q.Get("limit"), defaultLimit)),
		Offset:            maxInt(parseIntOr(q.Get("offset"), 0), 0),
	}
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

// parseBoolFilter parses "true"/"false" into *bool — nil (don't
// filter) for anything else, including empty.
func parseBoolFilter(s string) *bool {
	switch s {
	case "true":
		v := true
		return &v
	case "false":
		v := false
		return &v
	default:
		return nil
	}
}

// parseTimeFilter parses an RFC3339 timestamp into
// application_login_attempts.created_at's own storage format, for
// direct string comparison in SQL. Returns "" (don't filter) if s is
// empty or fails to parse.
func parseTimeFilter(s string) string {
	if s == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return ""
	}
	return t.UTC().Format("2006-01-02 15:04:05")
}
