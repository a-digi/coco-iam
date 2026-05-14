package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/a-digi/coco-iam/src/applications/publicapi/auth"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// publicGroup mirrors `user_groups` trimmed to the fields the public API
// cares about. Roles come from application_group_acl (now in per-org DB).
type publicGroup struct {
	ID               string   `json:"id"`
	Title            string   `json:"title"`
	GroupDescription string   `json:"group_description"`
	OrganizationID   string   `json:"organization_id"`
	IsActive         bool     `json:"is_active"`
	Roles            []string `json:"roles"`
}

type createGroupBody struct {
	Title            string   `json:"title"`
	GroupDescription string   `json:"group_description"`
	Roles            []string `json:"roles"`
	GrantableRoles   []string `json:"grantable_roles"`
}

type patchGroupBody struct {
	Title            *string `json:"title,omitempty"`
	GroupDescription *string `json:"group_description,omitempty"`
	IsActive         *bool   `json:"is_active,omitempty"`
}

// -- Groups list ------------------------------------------------------

type GroupsListHandler struct{}

func (h *GroupsListHandler) ServeHTTP(reqCtx request.RequestContext) {
	caller := auth.Authenticate(reqCtx, "groups:read")
	if caller == nil {
		return
	}
	if caller.OrgDB == nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, "org db not available")
		return
	}
	q := reqCtx.GetRequest().URL.Query()
	limit := parseLimit(q.Get("limit"), 50, 500)
	offset := parseOffset(q.Get("offset"))
	titleLike := strings.TrimSpace(q.Get("filter[@like:title]"))

	// Step 1: fetch group_id → roles from per-org DB (application_group_acl).
	aclRows, err := caller.OrgDB.Query(
		`SELECT group_id, roles FROM application_group_acl WHERE application_id = ? AND is_active = TRUE`,
		caller.ApplicationID,
	)
	if err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, err.Error())
		return
	}
	groupRoles := map[string][]string{}
	var groupIDs []string
	for aclRows.Next() {
		var gid string
		var rolesRaw []byte
		if err := aclRows.Scan(&gid, &rolesRaw); err != nil {
			aclRows.Close()
			response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, err.Error())
			return
		}
		var roles []string
		_ = json.Unmarshal(rolesRaw, &roles)
		if roles == nil {
			roles = []string{}
		}
		groupRoles[gid] = roles
		groupIDs = append(groupIDs, gid)
	}
	aclRows.Close()

	if len(groupIDs) == 0 {
		response.SuccessResponse(reqCtx.GetWriter(), http.StatusOK, map[string]any{
			"groups": []publicGroup{},
			"limit":  limit,
			"offset": offset,
		})
		return
	}

	// Step 2: query user_groups from per-org DB using the group IDs collected above.
	ph := buildInClause(len(groupIDs))
	args := stringArgs(groupIDs)

	query := `SELECT id, title, group_description, organization_id, is_active FROM user_groups WHERE id IN (` + ph + `)`
	if titleLike != "" {
		query += ` AND title LIKE ?`
		args = append(args, "%"+titleLike+"%")
	}
	if allowed := caller.AllowedIDs("groups:read"); allowed != nil {
		query += ` AND id ` + buildInClause(len(allowed))
		args = append(args, stringArgs(allowed)...)
	}
	query += ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := caller.OrgDB.Query(query, args...)
	if err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	out := []publicGroup{}
	for rows.Next() {
		var g publicGroup
		if err := rows.Scan(&g.ID, &g.Title, &g.GroupDescription, &g.OrganizationID, &g.IsActive); err != nil {
			response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, err.Error())
			return
		}
		g.Roles = groupRoles[g.ID]
		if g.Roles == nil {
			g.Roles = []string{}
		}
		out = append(out, g)
	}
	response.SuccessResponse(reqCtx.GetWriter(), http.StatusOK, map[string]any{
		"groups": out,
		"limit":  limit,
		"offset": offset,
	})
}

// -- Groups get -------------------------------------------------------

type GroupsGetHandler struct{}

func (h *GroupsGetHandler) ServeHTTP(reqCtx request.RequestContext) {
	caller := auth.Authenticate(reqCtx, "groups:read")
	if caller == nil {
		return
	}
	if caller.OrgDB == nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, "org db not available")
		return
	}
	groupID := groupIDFromPath(reqCtx.GetRequest().URL.Path)
	if !caller.CanActOnID("groups:read", groupID) {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusNotFound, "group not found")
		return
	}
	g, err := fetchGroup(caller.OrgDB, caller.ApplicationID, groupID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.ErrorResponse(reqCtx.GetWriter(), http.StatusNotFound, "group not found")
			return
		}
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(reqCtx.GetWriter(), http.StatusOK, g)
}

// -- Groups create ----------------------------------------------------

type GroupsCreateHandler struct{}

func (h *GroupsCreateHandler) ServeHTTP(reqCtx request.RequestContext) {
	caller := auth.Authenticate(reqCtx, "groups:write")
	if caller == nil {
		return
	}
	if caller.OrgDB == nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, "org db not available")
		return
	}
	var body createGroupBody
	if err := reqCtx.BindJSON(&body); err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusBadRequest, "invalid JSON body")
		return
	}
	body.Title = strings.TrimSpace(body.Title)
	if body.Title == "" {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusBadRequest, "title is required")
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
	if orgID == "" {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, "could not resolve application organisation")
		return
	}

	groupID := newUUID()

	if _, err := caller.OrgDB.Exec(
		`INSERT INTO user_groups (id, title, group_description, organization_id, is_active)
		 VALUES (?, ?, ?, ?, TRUE)`,
		groupID, body.Title, body.GroupDescription, orgID,
	); err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusBadRequest, err.Error())
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
	if _, err := caller.OrgDB.Exec(
		`INSERT INTO application_group_acl (id, application_id, group_id, roles, grantable_roles, is_active)
		 VALUES (?, ?, ?, ?, ?, TRUE)`,
		newUUID(), caller.ApplicationID, groupID, string(rolesJSON), string(grantableJSON),
	); err != nil {
		// Compensate: remove the group we just created in the per-org DB.
		_, _ = caller.OrgDB.Exec(`DELETE FROM user_groups WHERE id = ?`, groupID)
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, err.Error())
		return
	}

	g, err := fetchGroup(caller.OrgDB, caller.ApplicationID, groupID)
	if err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(reqCtx.GetWriter(), http.StatusCreated, g)
}

// -- Groups patch -----------------------------------------------------

type GroupsPatchHandler struct{}

func (h *GroupsPatchHandler) ServeHTTP(reqCtx request.RequestContext) {
	caller := auth.Authenticate(reqCtx, "groups:write")
	if caller == nil {
		return
	}
	groupID := groupIDFromPath(reqCtx.GetRequest().URL.Path)
	if !caller.CanActOnID("groups:write", groupID) || !groupOnACL(caller.OrgDB, caller.ApplicationID, groupID) {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusNotFound, "group not found")
		return
	}
	var body patchGroupBody
	if err := reqCtx.BindJSON(&body); err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusBadRequest, "invalid JSON body")
		return
	}
	sets := []string{}
	args := []interface{}{}
	if body.Title != nil {
		sets = append(sets, "title = ?")
		args = append(args, strings.TrimSpace(*body.Title))
	}
	if body.GroupDescription != nil {
		sets = append(sets, "group_description = ?")
		args = append(args, *body.GroupDescription)
	}
	if body.IsActive != nil {
		sets = append(sets, "is_active = ?")
		args = append(args, *body.IsActive)
	}
	if len(sets) == 0 {
		g, _ := fetchGroup(caller.OrgDB, caller.ApplicationID, groupID)
		response.SuccessResponse(reqCtx.GetWriter(), http.StatusOK, g)
		return
	}
	args = append(args, groupID)
	if _, err := caller.OrgDB.Exec(
		`UPDATE user_groups SET `+strings.Join(sets, ", ")+` WHERE id = ?`,
		args...,
	); err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusBadRequest, err.Error())
		return
	}
	g, _ := fetchGroup(caller.OrgDB, caller.ApplicationID, groupID)
	response.SuccessResponse(reqCtx.GetWriter(), http.StatusOK, g)
}

// -- Groups delete (soft) --------------------------------------------

type GroupsDeleteHandler struct{}

func (h *GroupsDeleteHandler) ServeHTTP(reqCtx request.RequestContext) {
	caller := auth.Authenticate(reqCtx, "groups:delete")
	if caller == nil {
		return
	}
	groupID := groupIDFromPath(reqCtx.GetRequest().URL.Path)
	if !caller.CanActOnID("groups:delete", groupID) || !groupOnACL(caller.OrgDB, caller.ApplicationID, groupID) {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusNotFound, "group not found")
		return
	}
	if _, err := caller.OrgDB.Exec(`UPDATE user_groups SET is_active = FALSE WHERE id = ?`, groupID); err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := caller.OrgDB.Exec(
		`UPDATE application_group_acl SET is_active = FALSE WHERE application_id = ? AND group_id = ?`,
		caller.ApplicationID, groupID,
	); err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(reqCtx.GetWriter(), http.StatusOK, map[string]string{"status": "deleted"})
}

// -- helpers ---------------------------------------------------------

func fetchGroup(orgDB *sql.DB, appID, groupID string) (publicGroup, error) {
	// Query roles from per-org DB.
	var rolesRaw []byte
	err := orgDB.QueryRow(
		`SELECT roles FROM application_group_acl
		 WHERE application_id = ? AND group_id = ? AND is_active = TRUE
		 LIMIT 1`,
		appID, groupID,
	).Scan(&rolesRaw)
	if err != nil {
		return publicGroup{}, err
	}

	// Query group metadata from per-org DB.
	var g publicGroup
	if err := orgDB.QueryRow(
		`SELECT id, title, group_description, organization_id, is_active
		 FROM user_groups WHERE id = ? LIMIT 1`,
		groupID,
	).Scan(&g.ID, &g.Title, &g.GroupDescription, &g.OrganizationID, &g.IsActive); err != nil {
		return publicGroup{}, err
	}

	_ = json.Unmarshal(rolesRaw, &g.Roles)
	if g.Roles == nil {
		g.Roles = []string{}
	}
	return g, nil
}

func groupOnACL(orgDB *sql.DB, appID, groupID string) bool {
	if orgDB == nil || groupID == "" {
		return false
	}
	var exists int
	err := orgDB.QueryRow(
		`SELECT 1 FROM application_group_acl
		 WHERE application_id = ? AND group_id = ? AND is_active = TRUE
		 LIMIT 1`, appID, groupID,
	).Scan(&exists)
	return err == nil
}

func groupIDFromPath(path string) string {
	segs := strings.Split(strings.Trim(path, "/"), "/")
	for i := 0; i+1 < len(segs); i++ {
		if segs[i] == "groups" {
			return strings.TrimSpace(segs[i+1])
		}
	}
	return ""
}

func groupIDFromPathBetween(path, start, end string) string {
	segs := strings.Split(strings.Trim(path, "/"), "/")
	for i := 0; i+2 < len(segs); i++ {
		if segs[i] == start && segs[i+2] == end {
			return strings.TrimSpace(segs[i+1])
		}
	}
	return ""
}
