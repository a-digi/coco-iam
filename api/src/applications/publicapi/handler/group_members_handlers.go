package handler

import (
	"net/http"
	"strings"

	"github.com/a-digi/coco-iam/src/applications/publicapi/auth"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

type memberView struct {
	MembershipID string `json:"membership_id"`
	UserID       string `json:"user_id"`
	Username     string `json:"username"`
	Email        string `json:"email"`
}

type addMemberBody struct {
	UserID string `json:"user_id"`
}

// GroupMembersListHandler serves GET .../groups/{groupId}/members.
// Members are further filtered to users that are themselves on the
// app's ACL — a group can technically include anyone in the org, but
// we surface only the slice relevant to this application.
type GroupMembersListHandler struct{}

// @Summary     List members of a group
// @Tags        public-api
// @Produce     json
// @Param       id path string true "Application ID"
// @Param       groupId path string true "Group ID"
// @Success     200 {object} interface{}
// @Failure     400,401,403,500 {object} map[string]interface{}
// @Router      /public/applications/{id}/groups/{groupId}/members [get]
func (h *GroupMembersListHandler) ServeHTTP(reqCtx request.RequestContext) {
	caller := auth.Authenticate(reqCtx, "groups:read")
	if caller == nil {
		return
	}
	if caller.OrgDB == nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, "org db not available")
		return
	}
	groupID := groupIDFromPathBetween(reqCtx.GetRequest().URL.Path, "groups", "members")
	if !caller.CanActOnID("groups:read", groupID) || !groupOnACL(caller.OrgDB, caller.ApplicationID, groupID) {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusNotFound, "group not found")
		return
	}
	// user_group_members, users, and application_user_acl are all in the per-org DB.
	rows, err := caller.OrgDB.Query(
		`SELECT m.id, u.id, u.username, u.email
		 FROM user_group_members m
		 JOIN users u ON u.id = m.user_id
		 JOIN application_user_acl acl ON acl.user_id = u.id AND acl.is_active = TRUE
		 WHERE m.group_id = ? AND m.is_active = TRUE
		   AND acl.application_id = ?
		 ORDER BY u.username ASC`,
		groupID, caller.ApplicationID,
	)
	if err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	out := []memberView{}
	for rows.Next() {
		var m memberView
		if err := rows.Scan(&m.MembershipID, &m.UserID, &m.Username, &m.Email); err != nil {
			response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, err.Error())
			return
		}
		out = append(out, m)
	}
	response.SuccessResponse(reqCtx.GetWriter(), http.StatusOK, map[string]any{"members": out})
}

// GroupMembersAddHandler serves POST .../groups/{groupId}/members.
// Body: `{ user_id }`. The user must already exist on the app's
// ACL — you can't use group membership to smuggle in an unaffiliated
// user. Duplicate insertion is ignored so the endpoint is idempotent.
type GroupMembersAddHandler struct{}

// @Summary     Add a member to a group
// @Tags        public-api
// @Accept      json
// @Produce     json
// @Param       id path string true "Application ID"
// @Param       groupId path string true "Group ID"
// @Param       body body interface{} true "Request body"
// @Success     200 {object} interface{}
// @Failure     400,401,403,500 {object} map[string]interface{}
// @Router      /public/applications/{id}/groups/{groupId}/members [post]
func (h *GroupMembersAddHandler) ServeHTTP(reqCtx request.RequestContext) {
	caller := auth.Authenticate(reqCtx, "groups:write")
	if caller == nil {
		return
	}
	if caller.OrgDB == nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, "org db not available")
		return
	}
	groupID := groupIDFromPathBetween(reqCtx.GetRequest().URL.Path, "groups", "members")
	if !caller.CanActOnID("groups:write", groupID) || !groupOnACL(caller.OrgDB, caller.ApplicationID, groupID) {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusNotFound, "group not found")
		return
	}
	var body addMemberBody
	if err := reqCtx.BindJSON(&body); err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusBadRequest, "invalid JSON body")
		return
	}
	body.UserID = strings.TrimSpace(body.UserID)
	if body.UserID == "" {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusBadRequest, "user_id is required")
		return
	}
	// userOnACL checks application_user_acl, which is in the per-org DB.
	if !userOnACL(caller.OrgDB, caller.ApplicationID, body.UserID) {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusNotFound, "user not found")
		return
	}
	// Idempotent insert — a unique index on (user_id, group_id)
	// already exists in the schema.
	if _, err := caller.OrgDB.Exec(
		`INSERT OR IGNORE INTO user_group_members (id, user_id, group_id, is_active)
		 VALUES (?, ?, ?, TRUE)`,
		newUUID(), body.UserID, groupID,
	); err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(reqCtx.GetWriter(), http.StatusOK, map[string]string{"status": "added"})
}

// GroupMembersRemoveHandler serves DELETE .../groups/{groupId}/members/{userId}.
type GroupMembersRemoveHandler struct{}

// @Summary     Remove a member from a group
// @Tags        public-api
// @Produce     json
// @Param       id path string true "Application ID"
// @Param       groupId path string true "Group ID"
// @Param       userId path string true "User ID"
// @Success     200 {object} interface{}
// @Failure     400,401,403,500 {object} map[string]interface{}
// @Router      /public/applications/{id}/groups/{groupId}/members/{userId} [delete]
func (h *GroupMembersRemoveHandler) ServeHTTP(reqCtx request.RequestContext) {
	caller := auth.Authenticate(reqCtx, "groups:write")
	if caller == nil {
		return
	}
	if caller.OrgDB == nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, "org db not available")
		return
	}
	path := reqCtx.GetRequest().URL.Path
	groupID := groupIDFromPathBetween(path, "groups", "members")
	userID := segmentAfterPath(path, "members")
	if groupID == "" || userID == "" {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusBadRequest, "missing group or user id")
		return
	}
	if !caller.CanActOnID("groups:write", groupID) || !groupOnACL(caller.OrgDB, caller.ApplicationID, groupID) {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusNotFound, "group not found")
		return
	}
	res, err := caller.OrgDB.Exec(
		`UPDATE user_group_members SET is_active = FALSE
		 WHERE group_id = ? AND user_id = ?`,
		groupID, userID,
	)
	if err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, err.Error())
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusNotFound, "membership not found")
		return
	}
	response.SuccessResponse(reqCtx.GetWriter(), http.StatusOK, map[string]string{"status": "removed"})
}

func segmentAfterPath(path, marker string) string {
	segs := strings.Split(strings.Trim(path, "/"), "/")
	for i, s := range segs {
		if s == marker && i+1 < len(segs) {
			return strings.TrimSpace(segs[i+1])
		}
	}
	return ""
}

