// Package handler serves the public per-application management
// endpoints under /api/v1/public/applications/{id}/... — users today,
// groups + group members + ACLs rolling out in the same style.
//
// Every handler in here starts by calling auth.Authenticate() with
// the scope the route declares. Authenticate returns nil (and writes
// the error response itself) on any failure; handlers then short-
// circuit. Mutations that bestow roles call caller.EnsureGrantable
// to enforce the per-caller grantable budget.
package handler

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/a-digi/coco-iam/src/applications/publicapi/auth"
	"github.com/a-digi/coco-iam/src/auth/crypto/bcrypt"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// -- DTOs ---------------------------------------------------------------

type publicUser struct {
	ID             string   `json:"id"`
	Username       string   `json:"username"`
	Email          string   `json:"email"`
	OrganizationID string   `json:"organization_id"`
	IsActive       bool     `json:"is_active"`
	Roles          []string `json:"roles"`
}

type createUserBody struct {
	Username       string   `json:"username"`
	Email          string   `json:"email"`
	Password       string   `json:"password"`
	Roles          []string `json:"roles"`
	GrantableRoles []string `json:"grantable_roles"`
}

type patchUserBody struct {
	Username *string `json:"username,omitempty"`
	Email    *string `json:"email,omitempty"`
	IsActive *bool   `json:"is_active,omitempty"`
}

type passwordBody struct {
	Password string `json:"password"`
}

// -- Users list ---------------------------------------------------------

type UsersListHandler struct{}

// @Summary     List users for an application
// @Tags        public-api
// @Produce     json
// @Param       id path string true "Application ID"
// @Success     200 {object} interface{}
// @Failure     400,401,403,500 {object} map[string]interface{}
// @Router      /public/applications/{id}/users [get]
func (h *UsersListHandler) ServeHTTP(reqCtx request.RequestContext) {
	caller := auth.Authenticate(reqCtx, "users:read")
	if caller == nil {
		return
	}
	orgDB := caller.OrgDB
	if orgDB == nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, "org database unavailable")
		return
	}

	q := reqCtx.GetRequest().URL.Query()
	limit := parseLimit(q.Get("limit"), 50, 500)
	offset := parseOffset(q.Get("offset"))
	emailLike := strings.TrimSpace(q.Get("filter[@like:email]"))
	usernameLike := strings.TrimSpace(q.Get("filter[@like:username]"))

	query := `
		SELECT u.id, u.username, u.email, u.is_active, acl.roles
		FROM users u
		JOIN application_user_acl acl ON acl.user_id = u.id AND acl.is_active = TRUE
		WHERE acl.application_id = ?`
	args := []interface{}{caller.ApplicationID}
	if emailLike != "" {
		query += ` AND u.email LIKE ?`
		args = append(args, "%"+emailLike+"%")
	}
	if usernameLike != "" {
		query += ` AND u.username LIKE ?`
		args = append(args, "%"+usernameLike+"%")
	}
	// Resource-id constraint: if the caller's ACL restricts
	// `users:read` to specific ids, fold those into the WHERE as an
	// `IN (…)` clause. Unconstrained (nil) = no extra filter;
	// deny-all ([]) emits `IN (NULL)` so zero rows come back.
	if allowed := caller.AllowedIDs("users:read"); allowed != nil {
		query += ` AND u.id ` + buildInClause(len(allowed))
		args = append(args, stringArgs(allowed)...)
	}
	query += ` ORDER BY u.created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := orgDB.Query(query, args...)
	if err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	users := []publicUser{}
	for rows.Next() {
		var u publicUser
		var rolesRaw []byte
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.IsActive, &rolesRaw); err != nil {
			response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, err.Error())
			return
		}
		u.OrganizationID = caller.OrganizationID
		_ = json.Unmarshal(rolesRaw, &u.Roles)
		if u.Roles == nil {
			u.Roles = []string{}
		}
		users = append(users, u)
	}
	response.SuccessResponse(reqCtx.GetWriter(), http.StatusOK, map[string]any{
		"users":  users,
		"limit":  limit,
		"offset": offset,
	})
}

// -- Users get ----------------------------------------------------------

type UsersGetHandler struct{}

// @Summary     Get a user by ID
// @Tags        public-api
// @Produce     json
// @Param       id path string true "Application ID"
// @Param       userId path string true "User ID"
// @Success     200 {object} interface{}
// @Failure     400,401,403,500 {object} map[string]interface{}
// @Router      /public/applications/{id}/users/{userId} [get]
func (h *UsersGetHandler) ServeHTTP(reqCtx request.RequestContext) {
	caller := auth.Authenticate(reqCtx, "users:read")
	if caller == nil {
		return
	}
	orgDB := caller.OrgDB
	if orgDB == nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, "org database unavailable")
		return
	}
	userID := userIDFromPath(reqCtx.GetRequest().URL.Path)
	if userID == "" {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusBadRequest, "missing user id")
		return
	}
	if !caller.CanActOnID("users:read", userID) {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusNotFound, "user not found")
		return
	}
	u, err := fetchUser(orgDB, caller.ApplicationID, caller.OrganizationID, userID)
	if err != nil {
		writeUserNotFoundOr500(reqCtx, err)
		return
	}
	response.SuccessResponse(reqCtx.GetWriter(), http.StatusOK, u)
}

// -- Users create -------------------------------------------------------

type UsersCreateHandler struct{}

// @Summary     Create a user for an application
// @Tags        public-api
// @Accept      json
// @Produce     json
// @Param       id path string true "Application ID"
// @Param       body body interface{} true "Request body"
// @Success     200 {object} interface{}
// @Failure     400,401,403,500 {object} map[string]interface{}
// @Router      /public/applications/{id}/users [post]
func (h *UsersCreateHandler) ServeHTTP(reqCtx request.RequestContext) {
	caller := auth.Authenticate(reqCtx, "users:write")
	if caller == nil {
		return
	}
	orgDB := caller.OrgDB
	if orgDB == nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, "org database unavailable")
		return
	}
	var body createUserBody
	if err := reqCtx.BindJSON(&body); err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusBadRequest, "invalid JSON body")
		return
	}
	body.Username = strings.TrimSpace(body.Username)
	body.Email = strings.TrimSpace(body.Email)
	if body.Username == "" || body.Email == "" || body.Password == "" {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusBadRequest, "username, email, and password are required")
		return
	}
	if err := caller.EnsureGrantable(body.Roles); err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusForbidden, err.Error())
		return
	}
	if err := caller.EnsureGrantable(body.GrantableRoles); err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusForbidden, err.Error())
		return
	}

	orgID := caller.OrganizationID

	hash, err := bcrypt.HashPassword(body.Password, bcrypt.DefaultCost)
	if err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, "password hash failed")
		return
	}

	userID := newUUID()

	// Write per-org data in orgDB.
	orgTx, err := orgDB.Begin()
	if err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, err.Error())
		return
	}
	defer func() { _ = orgTx.Rollback() }()
	if _, err := orgTx.Exec(
		`INSERT INTO users (id, username, email, is_active, must_change_password)
		 VALUES (?, ?, ?, TRUE, FALSE)`,
		userID, body.Username, body.Email,
	); err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusBadRequest, err.Error())
		return
	}
	if _, err := orgTx.Exec(
		`INSERT INTO user_auth_password (user_id, password) VALUES (?, ?)`,
		userID, hash,
	); err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, err.Error())
		return
	}
	roles := body.Roles
	if roles == nil {
		roles = []string{}
	}
	grantable := body.GrantableRoles
	if grantable == nil {
		grantable = []string{}
	}
	rolesJSON, _ := json.Marshal(roles)
	grantableJSON, _ := json.Marshal(grantable)
	if _, err := orgTx.Exec(
		`INSERT INTO application_user_acl (id, application_id, user_id, roles, grantable_roles, is_active)
		 VALUES (?, ?, ?, ?, ?, TRUE)`,
		newUUID(), caller.ApplicationID, userID, string(rolesJSON), string(grantableJSON),
	); err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, err.Error())
		return
	}
	if err := orgTx.Commit(); err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, err.Error())
		return
	}

	u, err := fetchUser(orgDB, caller.ApplicationID, orgID, userID)
	if err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(reqCtx.GetWriter(), http.StatusCreated, u)
}

// -- Users patch --------------------------------------------------------

type UsersPatchHandler struct{}

// @Summary     Patch a user
// @Tags        public-api
// @Accept      json
// @Produce     json
// @Param       id path string true "Application ID"
// @Param       userId path string true "User ID"
// @Param       body body interface{} true "Request body"
// @Success     200 {object} interface{}
// @Failure     400,401,403,500 {object} map[string]interface{}
// @Router      /public/applications/{id}/users/{userId} [patch]
func (h *UsersPatchHandler) ServeHTTP(reqCtx request.RequestContext) {
	caller := auth.Authenticate(reqCtx, "users:write")
	if caller == nil {
		return
	}
	orgDB := caller.OrgDB
	if orgDB == nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, "org database unavailable")
		return
	}
	userID := userIDFromPath(reqCtx.GetRequest().URL.Path)
	if !caller.CanActOnID("users:write", userID) || !userOnACL(orgDB, caller.ApplicationID, userID) {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusNotFound, "user not found")
		return
	}
	var body patchUserBody
	if err := reqCtx.BindJSON(&body); err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusBadRequest, "invalid JSON body")
		return
	}
	sets := []string{}
	args := []interface{}{}
	if body.Username != nil {
		sets = append(sets, "username = ?")
		args = append(args, strings.TrimSpace(*body.Username))
	}
	if body.Email != nil {
		sets = append(sets, "email = ?")
		args = append(args, strings.TrimSpace(*body.Email))
	}
	if body.IsActive != nil {
		sets = append(sets, "is_active = ?")
		args = append(args, *body.IsActive)
	}
	if len(sets) == 0 {
		u, _ := fetchUser(orgDB, caller.ApplicationID, caller.OrganizationID, userID)
		response.SuccessResponse(reqCtx.GetWriter(), http.StatusOK, u)
		return
	}
	args = append(args, userID)
	if _, err := orgDB.Exec(
		`UPDATE users SET `+strings.Join(sets, ", ")+` WHERE id = ?`,
		args...,
	); err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusBadRequest, err.Error())
		return
	}
	u, _ := fetchUser(orgDB, caller.ApplicationID, caller.OrganizationID, userID)
	response.SuccessResponse(reqCtx.GetWriter(), http.StatusOK, u)
}

// -- Users password -----------------------------------------------------

type UsersPasswordHandler struct{}

// @Summary     Set a user's password
// @Tags        public-api
// @Accept      json
// @Produce     json
// @Param       id path string true "Application ID"
// @Param       userId path string true "User ID"
// @Param       body body interface{} true "Request body"
// @Success     200 {object} interface{}
// @Failure     400,401,403,500 {object} map[string]interface{}
// @Router      /public/applications/{id}/users/{userId}/password [post]
func (h *UsersPasswordHandler) ServeHTTP(reqCtx request.RequestContext) {
	caller := auth.Authenticate(reqCtx, "users:write")
	if caller == nil {
		return
	}
	orgDB := caller.OrgDB
	if orgDB == nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, "org database unavailable")
		return
	}
	userID := userIDFromPathBetween(reqCtx.GetRequest().URL.Path, "users", "password")
	if !caller.CanActOnID("users:write", userID) || !userOnACL(orgDB, caller.ApplicationID, userID) {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusNotFound, "user not found")
		return
	}
	var body passwordBody
	if err := reqCtx.BindJSON(&body); err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.Password == "" {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusBadRequest, "password is required")
		return
	}
	hash, err := bcrypt.HashPassword(body.Password, bcrypt.DefaultCost)
	if err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, "password hash failed")
		return
	}
	// The upsert here covers both "user already has a password row"
	// (the common case) and users seeded without a password row.
	if _, err := orgDB.Exec(
		`INSERT INTO user_auth_password (user_id, password)
		 VALUES (?, ?)
		 ON CONFLICT(user_id) DO UPDATE SET password = excluded.password`,
		userID, hash,
	); err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(reqCtx.GetWriter(), http.StatusOK, map[string]string{"status": "ok"})
}

// -- Users delete (soft) ------------------------------------------------

type UsersDeleteHandler struct{}

// @Summary     Delete a user (soft)
// @Tags        public-api
// @Produce     json
// @Param       id path string true "Application ID"
// @Param       userId path string true "User ID"
// @Success     200 {object} interface{}
// @Failure     400,401,403,500 {object} map[string]interface{}
// @Router      /public/applications/{id}/users/{userId} [delete]
func (h *UsersDeleteHandler) ServeHTTP(reqCtx request.RequestContext) {
	caller := auth.Authenticate(reqCtx, "users:delete")
	if caller == nil {
		return
	}
	orgDB := caller.OrgDB
	if orgDB == nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, "org database unavailable")
		return
	}
	userID := userIDFromPath(reqCtx.GetRequest().URL.Path)
	if !caller.CanActOnID("users:delete", userID) || !userOnACL(orgDB, caller.ApplicationID, userID) {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusNotFound, "user not found")
		return
	}
	tx, err := orgDB.Begin()
	if err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, err.Error())
		return
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`UPDATE users SET is_active = FALSE WHERE id = ?`, userID); err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := tx.Exec(
		`UPDATE application_user_acl SET is_active = FALSE
		 WHERE application_id = ? AND user_id = ?`,
		caller.ApplicationID, userID,
	); err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, err.Error())
		return
	}
	if err := tx.Commit(); err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(reqCtx.GetWriter(), http.StatusOK, map[string]string{"status": "deleted"})
}

// -- shared helpers -----------------------------------------------------

func fetchUser(orgDB *sql.DB, appID, orgID, userID string) (publicUser, error) {
	var u publicUser
	var rolesRaw []byte
	err := orgDB.QueryRow(
		`SELECT u.id, u.username, u.email, u.is_active, acl.roles
		 FROM users u
		 JOIN application_user_acl acl ON acl.user_id = u.id AND acl.is_active = TRUE
		 WHERE acl.application_id = ? AND u.id = ?
		 LIMIT 1`,
		appID, userID,
	).Scan(&u.ID, &u.Username, &u.Email, &u.IsActive, &rolesRaw)
	if err != nil {
		return publicUser{}, err
	}
	u.OrganizationID = orgID
	_ = json.Unmarshal(rolesRaw, &u.Roles)
	if u.Roles == nil {
		u.Roles = []string{}
	}
	return u, nil
}

func userOnACL(orgDB *sql.DB, appID, userID string) bool {
	if userID == "" {
		return false
	}
	var exists int
	err := orgDB.QueryRow(
		`SELECT 1 FROM application_user_acl
		 WHERE application_id = ? AND user_id = ? AND is_active = TRUE
		 LIMIT 1`, appID, userID,
	).Scan(&exists)
	return err == nil
}

func writeUserNotFoundOr500(reqCtx request.RequestContext, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusNotFound, "user not found")
		return
	}
	response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, err.Error())
}

// userIDFromPath walks `.../users/{userId}[/...]` and returns the id.
func userIDFromPath(path string) string {
	segs := strings.Split(strings.Trim(path, "/"), "/")
	for i := 0; i+1 < len(segs); i++ {
		if segs[i] == "users" {
			return strings.TrimSpace(segs[i+1])
		}
	}
	return ""
}

// userIDFromPathBetween returns the segment between `start` and `end`.
// Useful for sub-endpoints like `.../users/{id}/password`.
func userIDFromPathBetween(path, start, end string) string {
	segs := strings.Split(strings.Trim(path, "/"), "/")
	for i := 0; i+2 < len(segs); i++ {
		if segs[i] == start && segs[i+2] == end {
			return strings.TrimSpace(segs[i+1])
		}
	}
	return ""
}

func parseLimit(raw string, defaultVal, maxVal int) int {
	if raw == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return defaultVal
	}
	if n > maxVal {
		return maxVal
	}
	return n
}

func parseOffset(raw string) int {
	if raw == "" {
		return 0
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func dbFromCtx(reqCtx request.RequestContext) *sql.DB {
	manager := reqCtx.GetDI().GetDatabaseManager()
	if manager == nil || manager.Connector == nil || manager.Connector.DB == nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, "database unavailable")
		return nil
	}
	return manager.Connector.DB
}

func newUUID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "00000000-0000-0000-0000-000000000000"
	}
	hx := hex.EncodeToString(buf)
	return fmt.Sprintf("%s-%s-%s-%s-%s", hx[:8], hx[8:12], hx[12:16], hx[16:20], hx[20:32])
}
