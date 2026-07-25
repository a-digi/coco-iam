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

type groupAclView struct {
	GroupID        string   `json:"group_id"`
	Roles          []string `json:"roles"`
	GrantableRoles []string `json:"grantable_roles"`
	IsActive       bool     `json:"is_active"`
}

// GroupAclGetHandler serves GET .../groups/{groupId}/acl.
type GroupAclGetHandler struct{}

// @Summary     Get a group's ACL
// @Tags        public-app-group-acl
// @Produce     json
// @Param       id path string true "Application ID"
// @Param       groupId path string true "Group ID"
// @Success     200 {object} interface{}
// @Failure     400,401,403,500 {object} map[string]interface{}
// @Router      /public/applications/{id}/groups/{groupId}/acl [get]
func (h *GroupAclGetHandler) ServeHTTP(reqCtx request.RequestContext) {
	caller := auth.Authenticate(reqCtx, "acl:read")
	if caller == nil {
		return
	}
	groupID := groupIDFromPathBetween(reqCtx.GetRequest().URL.Path, "groups", "acl")
	if !caller.CanActOnID("acl:read", groupID) {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusNotFound, "group not found")
		return
	}
	view, err := loadGroupACL(caller.OrgDB, caller.ApplicationID, groupID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.ErrorResponse(reqCtx.GetWriter(), http.StatusNotFound, "group not found")
			return
		}
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(reqCtx.GetWriter(), http.StatusOK, view)
}

// GroupAclPutHandler serves PUT .../groups/{groupId}/acl.
type GroupAclPutHandler struct{}

// @Summary     Replace a group's ACL
// @Tags        public-app-group-acl
// @Accept      json
// @Produce     json
// @Param       id path string true "Application ID"
// @Param       groupId path string true "Group ID"
// @Param       body body interface{} true "Request body"
// @Success     200 {object} interface{}
// @Failure     400,401,403,500 {object} map[string]interface{}
// @Router      /public/applications/{id}/groups/{groupId}/acl [put]
func (h *GroupAclPutHandler) ServeHTTP(reqCtx request.RequestContext) {
	caller := auth.Authenticate(reqCtx, "acl:write")
	if caller == nil {
		return
	}
	groupID := groupIDFromPathBetween(reqCtx.GetRequest().URL.Path, "groups", "acl")
	if !caller.CanActOnID("acl:write", groupID) {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusNotFound, "group not found")
		return
	}
	existing, err := loadGroupACL(caller.OrgDB, caller.ApplicationID, groupID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.ErrorResponse(reqCtx.GetWriter(), http.StatusNotFound, "group not found")
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
	if err := caller.EnsureGrantable(diff(newRoles, existing.Roles)); err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusForbidden, err.Error())
		return
	}
	if err := caller.EnsureGrantable(diff(newGrantable, existing.GrantableRoles)); err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusForbidden, err.Error())
		return
	}
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
		`UPDATE application_group_acl
		    SET roles = ?, grantable_roles = ?
		  WHERE application_id = ? AND group_id = ? AND is_active = TRUE`,
		string(rolesJSON), string(grantableJSON), caller.ApplicationID, groupID,
	); err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, err.Error())
		return
	}
	refreshed, _ := loadGroupACL(caller.OrgDB, caller.ApplicationID, groupID)
	response.SuccessResponse(reqCtx.GetWriter(), http.StatusOK, refreshed)
}

// GroupAclDeleteHandler serves DELETE .../groups/{groupId}/acl.
type GroupAclDeleteHandler struct{}

// @Summary     Delete a group's ACL (soft)
// @Tags        public-app-group-acl
// @Produce     json
// @Param       id path string true "Application ID"
// @Param       groupId path string true "Group ID"
// @Success     200 {object} interface{}
// @Failure     400,401,403,500 {object} map[string]interface{}
// @Router      /public/applications/{id}/groups/{groupId}/acl [delete]
func (h *GroupAclDeleteHandler) ServeHTTP(reqCtx request.RequestContext) {
	caller := auth.Authenticate(reqCtx, "acl:delete")
	if caller == nil {
		return
	}
	groupID := groupIDFromPathBetween(reqCtx.GetRequest().URL.Path, "groups", "acl")
	if !caller.CanActOnID("acl:delete", groupID) || !groupOnACL(caller.OrgDB, caller.ApplicationID, groupID) {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusNotFound, "group not found")
		return
	}
	if _, err := caller.OrgDB.Exec(
		`UPDATE application_group_acl SET is_active = FALSE
		 WHERE application_id = ? AND group_id = ?`,
		caller.ApplicationID, groupID,
	); err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(reqCtx.GetWriter(), http.StatusOK, map[string]string{"status": "deleted"})
}

func loadGroupACL(db *sql.DB, appID, groupID string) (groupAclView, error) {
	var view groupAclView
	var rolesRaw, grantableRaw []byte
	err := db.QueryRow(
		`SELECT group_id, roles, grantable_roles, is_active
		 FROM application_group_acl
		 WHERE application_id = ? AND group_id = ? AND is_active = TRUE
		 LIMIT 1`,
		appID, groupID,
	).Scan(&view.GroupID, &rolesRaw, &grantableRaw, &view.IsActive)
	if err != nil {
		return groupAclView{}, err
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
