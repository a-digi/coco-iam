package me

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	profile_repo "github.com/a-digi/coco-iam/src/admin/users/profile/repository"
	jwt_token "github.com/a-digi/coco-iam/src/auth/security/jwt"
	"github.com/a-digi/coco-oauth/oauth"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// MeHandler serves GET /api/v1/admin/users/me. Returns a joined
// view of the authenticated admin_users row + admin_user_profiles
// row. When no profile row exists yet, empty strings flow through
// rather than null — the FE gets a consistent shape regardless of
// whether the admin has ever touched their profile.
type MeHandler struct{}

type meResponse struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	Email        string `json:"email"`
	IsSuperAdmin bool   `json:"is_super_admin"`
	IsActive     bool   `json:"is_active"`
	CreatedAt    string `json:"created_at,omitempty"`

	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Phone     string `json:"phone"`
	Locale    string `json:"locale"`
	Timezone  string `json:"timezone"`

	// AvatarURL is the public serve path the FE renders in an
	// <img src>. Empty string means "no avatar" — the FE falls
	// back to the placeholder icon.
	AvatarURL string `json:"avatar_url"`
}

// @Summary     Get current admin user
// @Tags        admin-me
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} interface{}
// @Failure     400,401,403,500 {object} map[string]interface{}
// @Router      /admin/users/me [get]
func (h *MeHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	ctx := reqCtx.GetDI()

	userID, ok := subjectFromBearer(r.Header.Get("Authorization"))
	if !ok {
		response.ErrorResponse(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	manager := ctx.GetDatabaseManager()
	if manager == nil || manager.Connector == nil || manager.Connector.DB == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "database manager not available")
		return
	}
	db := manager.Connector.DB

	// Pull the core admin row. A missing row means the token's
	// subject doesn't point at any live admin — treat as
	// unauthorized so the client reacts correctly.
	var out meResponse
	var created sql.NullString
	err := db.QueryRow(
		`SELECT id, username, email, is_super_admin, is_active, created_at
		 FROM admin_users WHERE id = ? LIMIT 1`,
		userID,
	).Scan(&out.ID, &out.Username, &out.Email, &out.IsSuperAdmin, &out.IsActive, &created)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.ErrorResponse(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if created.Valid {
		out.CreatedAt = created.String
	}

	// Join the profile row. ErrNotFound is the happy-path "no
	// profile yet" signal — we don't lazy-create on a simple GET
	// to keep this read-only and save the write amplification.
	repo := profile_repo.New(db)
	profile, err := repo.FindByAdminUserID(userID)
	if err != nil && !errors.Is(err, profile_repo.ErrNotFound) {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if profile != nil {
		out.FirstName = profile.FirstName
		out.LastName = profile.LastName
		out.Phone = profile.Phone
		out.Locale = profile.Locale
		out.Timezone = profile.Timezone
		if profile.AvatarAssetID != "" {
			out.AvatarURL = "/p/admin-avatars/" + userID
		}
	}

	response.SuccessResponse(w, http.StatusOK, out)
}

// subjectFromBearer parses the Authorization header, pulls the JWT
// subject, and returns (id, true) on success. Anything malformed
// returns ok=false so the caller can 401 uniformly.
func subjectFromBearer(header string) (string, bool) {
	if header == "" {
		return "", false
	}
	token, err := oauth.ExtractBearer(header)
	if err != nil {
		return "", false
	}
	userID, err := jwt_token.ParseJWTSubject(token)
	if err != nil || userID == "" {
		return "", false
	}
	return userID, true
}

// parseAdminCreatedAt is kept for downstream tests that want to
// round-trip the `created_at` text column — exported so future
// handlers share the same parse rules.
func parseAdminCreatedAt(s string) (time.Time, bool) {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

var _ = parseAdminCreatedAt // reserved for future use; kept to avoid an import churn if time becomes unused
