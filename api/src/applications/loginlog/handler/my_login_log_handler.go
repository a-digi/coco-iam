package handler

import (
	"crypto/rsa"
	"net/http"
	"time"

	loginlog_entity "github.com/a-digi/coco-iam/src/applications/loginlog/entity"
	loginlog_query "github.com/a-digi/coco-iam/src/applications/loginlog/repository/query"
	"github.com/a-digi/coco-iam/src/applications/userprofile"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

const genericUnauthorized = "unauthorized"

// MyLoginLogHandler serves
//
//	GET /a/{orgSlug}/{wsSlug}/{appSlug}/profile/me/login-log
//
// Self-service: the caller reads their OWN login attempts, identity
// resolved from their own bearer token — never from a URL/query
// parameter. Mirrors userprofile.GetMeHandler's auth flow exactly
// (same SlugResolver -> AuthenticateUser sequence), then reuses this
// package's own DB-handle resolution and query repo unchanged. See
// plan/self-service-login-log/plan.md.
type MyLoginLogHandler struct {
	Slugs userprofile.SlugResolver
	Keys  userprofile.KeyLoader
	Users userprofile.UserOrgReader
	Now   func() time.Time
}

// @Summary     Get my own login history
// @Description Lists the calling end-user's own login attempts (success and failure), newest
// @Description first. Identity comes from the caller's own bearer token — there is no way to
// @Description query another user's history through this endpoint.
// @Tags        app-profile-me
// @Produce     json
// @Param       success query bool false "Filter by outcome"
// @Param       from    query string false "created_at >= from, RFC3339"
// @Param       to      query string false "created_at <= to, RFC3339"
// @Param       limit   query int false "Max 500, default 50"
// @Param       offset  query int false "Default 0"
// @Success     200 {object} loginlog_entity.MyLoginAttemptListSuccess
// @Failure     401 {object} response.ErrorBody
// @Router      /a/{orgSlug}/{wsSlug}/{appSlug}/profile/me/login-log [get]
func (h *MyLoginLogHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	orgSlug, wsSlug, appSlug, ok := parseSlugSegments(r.URL.Path)
	if !ok {
		response.ErrorResponse(w, http.StatusUnauthorized, genericUnauthorized)
		return
	}

	appID, orgID, err := h.Slugs.ResolveSlugs(orgSlug, wsSlug, appSlug)
	if err != nil {
		response.ErrorResponse(w, http.StatusUnauthorized, genericUnauthorized)
		return
	}

	loadKey := userprofile.LoadPublicKeyFunc(func(kid string) (*rsa.PublicKey, error) {
		return h.Keys.LoadPublicKey(appID, kid)
	})
	userOrg := userprofile.UserOrgLookupFunc(h.Users.UserOrg)

	nowFn := time.Now
	if h.Now != nil {
		nowFn = h.Now
	}

	userID, authErr := userprofile.AuthenticateUser(
		r.Header.Get("Authorization"),
		orgID,
		loadKey,
		userOrg,
		nowFn(),
	)
	if authErr != nil {
		if authErr.Status == http.StatusInternalServerError {
			response.ErrorResponse(w, http.StatusInternalServerError, genericUnauthorized)
			return
		}
		response.ErrorResponse(w, http.StatusUnauthorized, genericUnauthorized)
		return
	}

	registry := resolveAppLoginLogRegistry(reqCtx)
	if registry == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "application login-log registry not available")
		return
	}
	handle, err := registry.For(appID)
	if err != nil {
		response.ErrorResponse(w, http.StatusNotFound, "login log not provisioned for this application")
		return
	}
	query := loginlog_query.NewApplicationLoginQueryRepo(handle)

	// Reuse the shared query-param parser for success/from/to/limit/
	// offset, then force the identity-bearing fields regardless of
	// what the query string contained — Username/IP/ApplicationUserID
	// are never client-controlled on this endpoint, only the caller's
	// own verified user id.
	filter := filterFromQuery(r.URL.Query())
	filter.Username = ""
	filter.IP = ""
	filter.ApplicationUserID = userID

	attempts, err := query.ListAttempts(filter)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]loginlog_entity.MyLoginAttempt, 0, len(attempts))
	for _, a := range attempts {
		out = append(out, loginlog_entity.MyLoginAttempt{
			ID:            a.ID,
			Success:       a.Success,
			FailureReason: a.FailureReason,
			IP:            a.IP,
			UserAgent:     a.UserAgent,
			CreatedAt:     a.CreatedAt,
		})
	}
	total, err := query.CountAttempts(filter)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.SuccessResponse(w, http.StatusOK, loginlog_entity.MyLoginAttemptListResponse{
		Attempts: out,
		Total:    total,
		Limit:    filter.Limit,
		Offset:   filter.Offset,
	})
}

// -- slug parsing (duplicated from userprofile/handler.go so this
// package's import surface stays narrow — same convention that
// file's own doc comment establishes) --------------------------------

func parseSlugSegments(path string) (org, ws, app string, ok bool) {
	parts := splitSegments(path)
	if len(parts) < 5 {
		return "", "", "", false
	}
	if parts[0] != "a" {
		return "", "", "", false
	}
	return parts[1], parts[2], parts[3], true
}

func splitSegments(path string) []string {
	out := make([]string, 0, 8)
	start := 0
	for i := 0; i <= len(path); i++ {
		if i == len(path) || path[i] == '/' {
			if i > start {
				out = append(out, path[start:i])
			}
			start = i + 1
		}
	}
	return out
}
