package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/a-digi/coco-iam/src/applications/publicapi/auth"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

type aclView struct {
	UserID         string   `json:"user_id"`
	Roles          []string `json:"roles"`
	GrantableRoles []string `json:"grantable_roles"`
	IsActive       bool     `json:"is_active"`
}

type aclPutBody struct {
	Roles          []string `json:"roles"`
	GrantableRoles []string `json:"grantable_roles"`
}

// UserAclGetHandler serves GET .../users/{userId}/acl.
type UserAclGetHandler struct{}

func (h *UserAclGetHandler) ServeHTTP(reqCtx request.RequestContext) {
	caller := auth.Authenticate(reqCtx, "acl:read")
	if caller == nil {
		return
	}
	if caller.OrgDB == nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, "org db not available")
		return
	}
	userID := userIDFromPathBetween(reqCtx.GetRequest().URL.Path, "users", "acl")
	if userID == "" {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusBadRequest, "missing user id")
		return
	}
	if !caller.CanActOnID("acl:read", userID) {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusNotFound, "user not found")
		return
	}
	view, err := loadUserACL(caller.OrgDB, caller.ApplicationID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.ErrorResponse(reqCtx.GetWriter(), http.StatusNotFound, "user not found")
			return
		}
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(reqCtx.GetWriter(), http.StatusOK, view)
}

// UserAclPutHandler serves PUT .../users/{userId}/acl.
// The existing ACL row's role list is replaced wholesale. Every role
// that's in the new set but not in the old set passes through
// EnsureGrantable; revocations bypass the check (we trust callers to
// tighten permissions even on roles they can't re-grant).
type UserAclPutHandler struct{}

func (h *UserAclPutHandler) ServeHTTP(reqCtx request.RequestContext) {
	caller := auth.Authenticate(reqCtx, "acl:write")
	if caller == nil {
		return
	}
	if caller.OrgDB == nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, "org db not available")
		return
	}
	userID := userIDFromPathBetween(reqCtx.GetRequest().URL.Path, "users", "acl")
	if !caller.CanActOnID("acl:write", userID) {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusNotFound, "user not found")
		return
	}
	existing, err := loadUserACL(caller.OrgDB, caller.ApplicationID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.ErrorResponse(reqCtx.GetWriter(), http.StatusNotFound, "user not found")
			return
		}
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, err.Error())
		return
	}
	var body aclPutBody
	if err := reqCtx.BindJSON(&body); err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusBadRequest, "invalid JSON body")
		return
	}
	newRoles := uniqueStrings(body.Roles)
	newGrantable := uniqueStrings(body.GrantableRoles)

	added := diff(newRoles, existing.Roles)
	addedGrantable := diff(newGrantable, existing.GrantableRoles)

	if err := caller.EnsureGrantable(added); err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusForbidden, err.Error())
		return
	}
	if err := caller.EnsureGrantable(addedGrantable); err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusForbidden, err.Error())
		return
	}
	// Invariant: grantable_roles ⊆ roles. We enforce on write so
	// direct SQL edits aren't the only safeguard.
	roleSet := toSet(newRoles)
	for _, g := range newGrantable {
		if _, ok := roleSet[g]; !ok {
			response.ErrorResponse(reqCtx.GetWriter(), http.StatusBadRequest,
				"grantable role "+g+" must also be present in roles")
			return
		}
	}

	rolesJSON, _ := json.Marshal(newRoles)
	grantableJSON, _ := json.Marshal(newGrantable)
	if _, err := caller.OrgDB.Exec(
		`UPDATE application_user_acl
		    SET roles = ?, grantable_roles = ?
		  WHERE application_id = ? AND user_id = ? AND is_active = TRUE`,
		string(rolesJSON), string(grantableJSON), caller.ApplicationID, userID,
	); err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, err.Error())
		return
	}
	refreshed, _ := loadUserACL(caller.OrgDB, caller.ApplicationID, userID)
	response.SuccessResponse(reqCtx.GetWriter(), http.StatusOK, refreshed)
}

// UserAclDeleteHandler serves DELETE .../users/{userId}/acl.
// Soft-delete only — the row survives for audit.
type UserAclDeleteHandler struct{}

func (h *UserAclDeleteHandler) ServeHTTP(reqCtx request.RequestContext) {
	caller := auth.Authenticate(reqCtx, "acl:delete")
	if caller == nil {
		return
	}
	if caller.OrgDB == nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, "org db not available")
		return
	}
	userID := userIDFromPathBetween(reqCtx.GetRequest().URL.Path, "users", "acl")
	if !caller.CanActOnID("acl:delete", userID) || !userOnACL(caller.OrgDB, caller.ApplicationID, userID) {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusNotFound, "user not found")
		return
	}
	if _, err := caller.OrgDB.Exec(
		`UPDATE application_user_acl SET is_active = FALSE
		 WHERE application_id = ? AND user_id = ?`,
		caller.ApplicationID, userID,
	); err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(reqCtx.GetWriter(), http.StatusOK, map[string]string{"status": "deleted"})
}

// -- shared ------------------------------------------------------------

func loadUserACL(db *sql.DB, appID, userID string) (aclView, error) {
	var view aclView
	var rolesRaw, grantableRaw []byte
	err := db.QueryRow(
		`SELECT user_id, roles, grantable_roles, is_active
		 FROM application_user_acl
		 WHERE application_id = ? AND user_id = ? AND is_active = TRUE
		 LIMIT 1`,
		appID, userID,
	).Scan(&view.UserID, &rolesRaw, &grantableRaw, &view.IsActive)
	if err != nil {
		return aclView{}, err
	}
	_ = json.Unmarshal(rolesRaw, &view.Roles)
	_ = json.Unmarshal(grantableRaw, &view.GrantableRoles)
	if view.Roles == nil {
		view.Roles = []string{}
	}
	if view.GrantableRoles == nil {
		view.GrantableRoles = []string{}
	}
	return view, nil
}

func uniqueStrings(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func diff(next, prev []string) []string {
	prevSet := toSet(prev)
	out := []string{}
	for _, v := range next {
		if _, ok := prevSet[v]; !ok {
			out = append(out, v)
		}
	}
	return out
}

func toSet(in []string) map[string]struct{} {
	out := make(map[string]struct{}, len(in))
	for _, v := range in {
		out[v] = struct{}{}
	}
	return out
}
