package users

import (
	"encoding/json"
	"net/http"

	user_entity "github.com/a-digi/coco-iam/src/admin/users/entity"
	crypto_bcrypt "github.com/a-digi/coco-iam/src/auth/crypto/bcrypt"
	"github.com/a-digi/coco-lift/resource/uri"
	db "github.com/a-digi/coco-orm/orm"
	"github.com/a-digi/coco-orm/orm/model"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// AdminUserResetPasswordHandler handles POST /admin/users/{id}/reset-password
// — an explicitly admin-privileged password reset, deliberately separate
// from PATCH /admin/{res:users}/{id} (which used to accept password +
// old_password, but verified old_password against the TARGET user's
// password rather than the caller's, so it could never succeed unless
// the admin already knew that user's current password). The privilege
// boundary here is the route's own scope check (admin:users:write +
// super:admin), not a password the admin has no way of knowing — same
// pattern already used by AdminUserSendActivationHandler. Self-service
// password changes stay on the separate account/password/change flow,
// which still requires the caller's current password.
type AdminUserResetPasswordHandler struct{}

type resetPasswordRequest struct {
	NewPassword string `json:"new_password"`
}

// @Summary     Reset another user's password (admin-privileged)
// @Description Sets a new password for the target user without requiring their current
// @Description one — an explicitly privileged admin action, distinct from the self-service
// @Description account/password/change flow.
// @Tags        admin-users
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id   path string                     true "User ID"
// @Param       body body entity.ResetPasswordRequest true "New password"
// @Success     200 {object} entity.ResetPasswordSuccess
// @Failure     400 {object} response.ErrorBody
// @Failure     401 {object} response.ErrorBody
// @Failure     403 {object} response.ErrorBody
// @Failure     404 {object} response.ErrorBody
// @Failure     500 {object} response.ErrorBody
// @Router      /admin/users/{id}/reset-password [post]
func (h *AdminUserResetPasswordHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	ctx := reqCtx.GetDI()
	r := reqCtx.GetRequest()

	manager := ctx.GetDatabaseManager()
	if manager == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "database manager not available")
		return
	}

	_, id := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if id == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "user id missing from path")
		return
	}

	var req resetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid json payload: "+err.Error())
		return
	}
	defer r.Body.Close()

	if req.NewPassword == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "new_password must be a non-empty string")
		return
	}

	var users []user_entity.User
	selectQuery := model.SelectQuery{
		Entity: &users,
		Filters: []model.Filter{
			{Column: "id", Value: id, Type: model.FilterTypeExactMatch},
		},
		Pagination: model.Pagination{Page: 1, Limit: 1},
	}
	if err := db.FindByQuery(manager, selectQuery); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to fetch user: "+err.Error())
		return
	}
	if len(users) == 0 {
		response.ErrorResponse(w, http.StatusNotFound, "user not found")
		return
	}

	hashed, err := crypto_bcrypt.HashPassword(req.NewPassword, 0)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to hash password: "+err.Error())
		return
	}

	// admin_auth_password (main DB) — NOT password_entity.Password's
	// user_auth_password, which is the per-organization end-user table
	// and doesn't exist in the main DB at all. Mirrors
	// password.Service.upsertPassword's exact upsert shape (the
	// self-service change-password flow's already-correct
	// implementation) rather than an UPDATE-only builder: a freshly
	// created admin user may have no password row yet (e.g. before
	// completing activation), and this must succeed either way.
	if _, err := manager.Connector.DB.Exec(
		`INSERT INTO admin_auth_password (user_id, password, created_at, is_active, changed_at)
		 VALUES (?, ?, CURRENT_TIMESTAMP, TRUE, CURRENT_TIMESTAMP)
		 ON CONFLICT(user_id) DO UPDATE SET password = excluded.password, is_active = TRUE, changed_at = CURRENT_TIMESTAMP`,
		id, hashed,
	); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to update password: "+err.Error())
		return
	}

	response.SuccessResponse(w, http.StatusOK, map[string]bool{"ok": true})
}
