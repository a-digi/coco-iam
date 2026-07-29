// Package handler serves the read-only admin login-log API under
// /api/v1/admin/security/login-log/admin*. See
// plan/login-audit-log/plan.md Step 4.
//
// Browsing INTO an archive reuses loginlog_entity/loginlog_query's
// exact response shapes — an archived generation has the identical
// admin_login_attempts schema as the live one, so there's no reason
// to duplicate those types just because the data came from a
// rotated-out file instead of the live connection.
package handler

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/a-digi/coco-iam/config/di"
	archives_entity "github.com/a-digi/coco-iam/src/admin/security/archives/entity"
	loginlog_entity "github.com/a-digi/coco-iam/src/admin/security/loginlog/entity"
	loginlog_query "github.com/a-digi/coco-iam/src/admin/security/loginlog/repository/query"
	"github.com/a-digi/coco-lift/resource/uri"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

const (
	defaultLimit = 50
	maxLimit     = 500
)

// AdminLoginListHandler serves GET
// /api/v1/admin/security/login-log/admin. Query parameters:
//
//	username=<name>        exact match
//	admin_user_id=<id>     exact match
//	success=true|false     exact match
//	ip=<addr>              exact match
//	from=<RFC3339>         created_at >= from
//	to=<RFC3339>           created_at <= to
//	limit=<n>              max 500, default 50
//	offset=<n>
type AdminLoginListHandler struct{}

// @Summary     List admin-console login attempts
// @Description Lists admin login attempts (success and failure), newest first.
// @Tags        security
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} loginlog_entity.AdminLoginAttemptListSuccess
// @Failure     401,403,500 {object} response.ErrorBody
// @Router      /admin/security/login-log/admin [get]
func (h *AdminLoginListHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	query, ok := resolveAdminLoginQuery(reqCtx)
	if !ok {
		return
	}

	filter := filterFromQuery(r.URL.Query())

	attempts, err := query.ListAttempts(filter)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if attempts == nil {
		attempts = []loginlog_entity.AdminLoginAttempt{}
	}
	total, err := query.CountAttempts(filter)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.SuccessResponse(w, http.StatusOK, loginlog_entity.AdminLoginAttemptListResponse{
		Attempts: attempts,
		Total:    total,
		Limit:    filter.Limit,
		Offset:   filter.Offset,
	})
}

// AdminLoginArchiveListHandler serves GET
// /api/v1/admin/security/login-log/admin/archives.
type AdminLoginArchiveListHandler struct{}

// @Summary     List admin_login.db archives
// @Description Lists rotated-out admin_login.db generations, newest first.
// @Tags        security
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} archives_entity.ArchiveListSuccess
// @Failure     401,403,500 {object} response.ErrorBody
// @Router      /admin/security/login-log/admin/archives [get]
func (h *AdminLoginArchiveListHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	archiveQuery, ok := resolveAdminLoginArchiveQuery(reqCtx)
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

// AdminLoginArchiveAttemptsHandler serves GET
// /api/v1/admin/security/login-log/admin/archives/{id:<value>}/attempts
// — the same filter/pagination shape as the live list, read from the
// archived file instead. See admin/security/handler's doc comment for
// the {key:value} path convention.
type AdminLoginArchiveAttemptsHandler struct{}

// @Summary     List admin login attempts within one archive
// @Tags        security
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Archive id, as {id:<value>}"
// @Success     200 {object} loginlog_entity.AdminLoginAttemptListSuccess
// @Failure     400,401,403,404,500 {object} response.ErrorBody
// @Router      /admin/security/login-log/admin/archives/{id}/attempts [get]
func (h *AdminLoginArchiveAttemptsHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	key, archiveID := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if key != "id" || archiveID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "archive id is required")
		return
	}

	archiveQuery, ok := resolveAdminLoginArchiveQuery(reqCtx)
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
		attempts = []loginlog_entity.AdminLoginAttempt{}
	}
	total, err := attemptsRepo.CountAttempts(filter)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.SuccessResponse(w, http.StatusOK, loginlog_entity.AdminLoginAttemptListResponse{
		Attempts: attempts,
		Total:    total,
		Limit:    filter.Limit,
		Offset:   filter.Offset,
	})
}

// filterFromQuery builds a ListFilter from the request's query
// string — shared by the live list and the archive-browse list, since
// both accept the identical filter/pagination shape.
func filterFromQuery(q url.Values) loginlog_query.ListFilter {
	return loginlog_query.ListFilter{
		Username:    strings.TrimSpace(q.Get("username")),
		AdminUserID: strings.TrimSpace(q.Get("admin_user_id")),
		Success:     parseBoolFilter(q.Get("success")),
		IP:          strings.TrimSpace(q.Get("ip")),
		From:        parseTimeFilter(q.Get("from")),
		To:          parseTimeFilter(q.Get("to")),
		Limit:       clampLimit(parseIntOr(q.Get("limit"), defaultLimit)),
		Offset:      maxInt(parseIntOr(q.Get("offset"), 0), 0),
	}
}

// resolveAdminLoginQuery builds a query repo against the live
// admin_login.db, resolved via the DI ContextBag. Reads through the
// same *dbhandle.Handle the login-log recorder writes through, so
// this keeps reading the live generation across the archiver rotating
// admin_login.db out from under it. Writes a 500 response itself and
// returns ok=false if unavailable.
func resolveAdminLoginQuery(reqCtx request.RequestContext) (*loginlog_query.AdminLoginQueryRepo, bool) {
	w := reqCtx.GetWriter()
	bag, ok := reqCtx.GetDI().(*di.ContextBag)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "DI context has unexpected type")
		return nil, false
	}
	handle := bag.GetAdminLoginHandle()
	if handle == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "admin-login database not available")
		return nil, false
	}
	return loginlog_query.NewAdminLoginQueryRepo(handle), true
}

// resolveAdminLoginArchiveQuery builds a query repo against the main
// DB (which holds admin_login_archives — unlike admin_login.db
// itself, the main DB is never rotated).
func resolveAdminLoginArchiveQuery(reqCtx request.RequestContext) (*loginlog_query.AdminLoginArchiveQueryRepo, bool) {
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
	return loginlog_query.NewAdminLoginArchiveQueryRepo(manager.Connector.DB), true
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
// admin_login_attempts.created_at's own storage format, for direct
// string comparison in SQL. Returns "" (don't filter) if s is empty or
// fails to parse — a malformed date filter degrades to "no filter"
// rather than failing the request.
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
