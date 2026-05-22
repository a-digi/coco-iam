package admin

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/a-digi/coco-lift/resource/uri"
	"github.com/a-digi/coco-iam/src/applications/cleanup"
	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	users_entity "github.com/a-digi/coco-iam/src/organizations/users/entity"
	"github.com/a-digi/coco-iam/src/organizations/users/notify"
	org_user_query "github.com/a-digi/coco-iam/src/organizations/users/repository/query"
	"github.com/a-digi/coco-iam/src/orgrouter"
	"github.com/a-digi/coco-queue"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// orgUserResponse wraps users_entity.User with the computed activation_pending
// field so the frontend can show/hide the resend-activation button without an
// extra round-trip. users_entity.User is left unchanged.
type orgUserResponse struct {
	users_entity.User
	ActivationPending bool `json:"activation_pending"`
}

// CustomGetOrganizationUsersHandler serves GET /{res:organization_users} (list)
// AND GET /{res:organization_users}/{id:uuid} (single). The custom handler
// overrides generic ORM dispatch entirely, so both shapes land here.
//
// - List: requires filter[@exact:organization_id]=<uuid>; queries per-org DB.
// - By ID: scans per-org DBs to find the user; queries per-org DB.
//
//	@Summary		List or get organization users
//	@Description	Returns a list of users for the given organization (requires organization_id filter) or a single user by ID.
//	@Tags			org-users
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id						path		string	false	"User ID (single-user lookup)"
//	@Param			filter[@exact:organization_id]	query		string	false	"Organization ID filter (list lookup)"
//	@Param			limit					query		int		false	"Page size (max 500, default 50)"
//	@Param			page					query		int		false	"Page number (1-based)"
//	@Success		200		{object}	users_entity.OrgUserListSuccess
//	@Failure		400		{object}	response.ErrorBody
//	@Failure		404		{object}	response.ErrorBody
//	@Failure		500		{object}	response.ErrorBody
//	@Router			/admin/organization_users [get]
//	@Router			/admin/organization_users/{id} [get]
func CustomGetOrganizationUsersHandler(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	_, userID := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if userID != "" {
		customGetOrganizationUserByID(reqCtx, w, r, userID)
	} else {
		customListOrganizationUsers(reqCtx, w, r)
	}
}

func customListOrganizationUsers(reqCtx request.RequestContext, w http.ResponseWriter, r *http.Request) {
	ctx := reqCtx.GetDI()

	orgID := extractOrgIDFilter(r)
	if orgID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "organization_id filter is required")
		return
	}

	reg := resolveOrgUserRegistry(ctx)
	if reg == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "org user db registry not available")
		return
	}
	orgDB, err := orgrouter.ForOrg(reg, orgID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to open org db: "+err.Error())
		return
	}

	limit := parseLimitParam(r.URL.Query().Get("limit"), 50)
	page := parsePageParam(r.URL.Query().Get("page"), 1)
	offset := (page - 1) * limit

	rows, err := orgDB.Query(
		`SELECT u.id, u.username, u.email, u.is_active, u.created_at,
		        NOT EXISTS (
		            SELECT 1 FROM user_activations
		            WHERE user_id = u.id AND consumed_at IS NOT NULL
		        ) AS activation_pending
		 FROM users u
		 ORDER BY u.created_at DESC
		 LIMIT ? OFFSET ?`,
		limit, offset,
	)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to query users: "+err.Error())
		return
	}
	defer rows.Close()

	out := []orgUserResponse{}
	for rows.Next() {
		var u users_entity.User
		var createdAt sql.NullString
		var activationPending bool
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.IsActive, &createdAt, &activationPending); err != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, "failed to scan user: "+err.Error())
			return
		}
		if createdAt.Valid {
			u.CreatedAt = createdAt.String
		}
		u.OrganizationID = orgID
		out = append(out, orgUserResponse{User: u, ActivationPending: activationPending})
	}

	response.SuccessResponse(w, http.StatusOK, out)
}

func customGetOrganizationUserByID(reqCtx request.RequestContext, w http.ResponseWriter, r *http.Request, userID string) {
	mainDB, reg, ok := resolveDBs(reqCtx, w)
	if !ok {
		return
	}

	orgDB, orgID, err := orgDBForUser(mainDB, reg, userID)
	if err != nil {
		response.ErrorResponse(w, http.StatusNotFound, "user not found")
		return
	}

	u, err := fetchOrgUser(orgDB, orgID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.ErrorResponse(w, http.StatusNotFound, "user not found")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, u)
}

// CustomPatchOrganizationUserHandler serves PATCH /{res:organization_users}/{id:uuid}.
// Accepts a JSON body with any subset of {username, email, is_active}.
//
//	@Summary		Update organization user
//	@Description	Partially updates an org user. Username changes are rejected.
//	@Tags			org-users
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		string							true	"User ID"
//	@Param			body	body		users_entity.PatchOrgUserRequest	true	"Fields to update"
//	@Success		200		{object}	users_entity.OrgUserSuccess
//	@Failure		400		{object}	response.ErrorBody
//	@Failure		404		{object}	response.ErrorBody
//	@Failure		409		{object}	response.ErrorBody
//	@Failure		500		{object}	response.ErrorBody
//	@Router			/admin/organization_users/{id} [patch]
func CustomPatchOrganizationUserHandler(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	_, userID := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if userID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "user id missing from path")
		return
	}

	mainDB, reg, ok := resolveDBs(reqCtx, w)
	if !ok {
		return
	}

	orgDB, orgID, err := orgDBForUser(mainDB, reg, userID)
	if err != nil {
		response.ErrorResponse(w, http.StatusNotFound, "user not found")
		return
	}

	existing, err := fetchOrgUser(orgDB, orgID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.ErrorResponse(w, http.StatusNotFound, "user not found")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	var patch struct {
		Username *string `json:"username"`
		Email    *string `json:"email"`
		IsActive *bool   `json:"is_active"`
	}
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}

	if patch.Username != nil && *patch.Username != existing.Username {
		response.ErrorResponse(w, http.StatusBadRequest, "username changes are not allowed")
		return
	}

	newEmail := existing.Email
	newIsActive := existing.IsActive
	if patch.Email != nil && *patch.Email != existing.Email {
		qrepo := org_user_query.New(orgDB)
		if exists, err := qrepo.ExistsByEmailExcludingID(*patch.Email, userID); err != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, "failed to check email: "+err.Error())
			return
		} else if exists {
			response.ErrorResponse(w, http.StatusConflict, "email already taken")
			return
		}
		newEmail = *patch.Email
	} else if patch.Email != nil {
		newEmail = *patch.Email
	}
	if patch.IsActive != nil {
		newIsActive = *patch.IsActive
	}

	if _, err := orgDB.Exec(
		`UPDATE users SET email = ?, is_active = ? WHERE id = ?`,
		newEmail, newIsActive, userID,
	); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to update user: "+err.Error())
		return
	}

	if existing.IsActive && !newIsActive {
		if svc := resolveNotifyService(reqCtx.GetDI()); svc != nil {
			svc.OnDeactivated(context.Background(), userID, existing.Username, newEmail, orgID)
		}
	}

	existing.Email = newEmail
	existing.IsActive = newIsActive
	response.SuccessResponse(w, http.StatusOK, existing)
}

// CustomDeleteOrganizationUserHandler serves DELETE /{res:organization_users}/{id:uuid}.
// Removes the user from the per-org DB and cleans the main DB routing indexes.
//
//	@Summary		Delete organization user
//	@Description	Removes the org user from the per-org DB and enqueues ACL cleanup.
//	@Tags			org-users
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"User ID"
//	@Success		200	{object}	users_entity.OrgUserSuccess
//	@Failure		400	{object}	response.ErrorBody
//	@Failure		404	{object}	response.ErrorBody
//	@Failure		500	{object}	response.ErrorBody
//	@Router			/admin/organization_users/{id} [delete]
func CustomDeleteOrganizationUserHandler(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	_, userID := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if userID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "user id missing from path")
		return
	}

	mainDB, reg, ok := resolveDBs(reqCtx, w)
	if !ok {
		return
	}

	orgDB, orgID, err := orgDBForUser(mainDB, reg, userID)
	if err != nil {
		response.ErrorResponse(w, http.StatusNotFound, "user not found")
		return
	}

	existing, err := fetchOrgUser(orgDB, orgID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.ErrorResponse(w, http.StatusNotFound, "user not found")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	if _, err := orgDB.Exec(`DELETE FROM users WHERE id = ?`, userID); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to delete user: "+err.Error())
		return
	}

	if svc := resolveNotifyService(reqCtx.GetDI()); svc != nil {
		svc.OnRemoved(context.Background(), userID, existing.Username, existing.Email, orgID)
	}

	enqueueUserCleanup(reqCtx, userID, orgID)

	response.SuccessResponse(w, http.StatusOK, existing)
}

// --- internal helpers ---

func resolveNotifyService(ctx interface{}) *notify.Service {
	bag, ok := ctx.(bagGetter)
	if !ok {
		return nil
	}
	raw, ok := bag.Get(notify.ContextBagKey)
	if !ok {
		return nil
	}
	svc, _ := raw.(*notify.Service)
	return svc
}

// resolveDBs extracts the main DatabaseManager and OrgUserDBRegistry from
// the DI context, writing an error response and returning false on failure.
func resolveDBs(reqCtx request.RequestContext, w http.ResponseWriter) (*sql.DB, *dbregistry.OrgUserDBRegistry, bool) {
	ctx := reqCtx.GetDI()
	manager := ctx.GetDatabaseManager()
	if manager == nil || manager.Connector == nil || manager.Connector.DB == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "database manager not available")
		return nil, nil, false
	}
	reg := resolveOrgUserRegistry(ctx)
	if reg == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "org user db registry not available")
		return nil, nil, false
	}
	return manager.Connector.DB, reg, true
}

// orgDBForUser resolves the per-org DB for a given user ID by scanning
// all known per-org DBs.
func orgDBForUser(_ *sql.DB, reg *dbregistry.OrgUserDBRegistry, userID string) (*sql.DB, string, error) {
	orgDB, orgID, err := orgrouter.OrgDBFor(reg, userID)
	if err != nil {
		return nil, "", fmt.Errorf("user %s not found in any org: %w", userID, err)
	}
	return orgDB, orgID, nil
}

// fetchOrgUser SELECTs one user row from the per-org DB, injects orgID, and
// computes activation_pending via a correlated subquery on user_activations.
func fetchOrgUser(orgDB *sql.DB, orgID, userID string) (orgUserResponse, error) {
	var u users_entity.User
	var createdAt sql.NullString
	var activationPending bool
	err := orgDB.QueryRow(
		`SELECT u.id, u.username, u.email, u.is_active, u.created_at,
		        NOT EXISTS (
		            SELECT 1 FROM user_activations
		            WHERE user_id = u.id AND consumed_at IS NOT NULL
		        ) AS activation_pending
		 FROM users u WHERE u.id = ? LIMIT 1`,
		userID,
	).Scan(&u.ID, &u.Username, &u.Email, &u.IsActive, &createdAt, &activationPending)
	if err != nil {
		return orgUserResponse{}, err
	}
	if createdAt.Valid {
		u.CreatedAt = createdAt.String
	}
	u.OrganizationID = orgID
	return orgUserResponse{User: u, ActivationPending: activationPending}, nil
}

// extractOrgIDFilter reads organization_id from the canonical coco-lift
// filter param (filter[@exact:organization_id]) or a plain ?organization_id=.
func extractOrgIDFilter(r *http.Request) string {
	q := r.URL.Query()
	if v := strings.TrimSpace(q.Get("filter[@exact:organization_id]")); v != "" {
		return v
	}
	return strings.TrimSpace(q.Get("organization_id"))
}

func parseLimitParam(s string, def int) int {
	if v, err := strconv.Atoi(s); err == nil && v > 0 && v <= 500 {
		return v
	}
	return def
}

func parsePageParam(s string, def int) int {
	if v, err := strconv.Atoi(s); err == nil && v > 0 {
		return v
	}
	return def
}

// enqueueUserCleanup publishes an application-user-cleanup task for the given
// user. orgID is included so the consumer can open the per-org DB directly.
// Errors are logged but never fail the response — the delete has already succeeded.
func enqueueUserCleanup(reqCtx request.RequestContext, userID, orgID string) {
	ctx := reqCtx.GetDI()
	raw, ok := ctx.(interface{ Get(string) (interface{}, bool) })
	if !ok {
		return
	}
	v, ok := raw.Get(queue.ContextBagKey)
	if !ok {
		return
	}
	mgr, ok := v.(queue.Manager)
	if !ok {
		return
	}
	if err := mgr.Publish("application-user-cleanup", cleanup.Payload{
		UserID: userID,
		OrgID:  orgID,
	}); err != nil {
		ctx.GetLogger().Warning("application-user-cleanup enqueue failed for user %s: %v", userID, err)
	}
}
