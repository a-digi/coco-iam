package admin

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/a-digi/coco-iam/src/organizations/acl"
	users_entity "github.com/a-digi/coco-iam/src/organizations/users/entity"
	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-iam/src/orgrouter"
	"github.com/a-digi/coco-lift/resource/uri"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
	"github.com/google/uuid"
)

// CustomOrgUserAclHandler handles all CRUD for organization_user_acl routed to
// the per-org users.db. Dispatches by HTTP method.
func CustomOrgUserAclHandler(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	switch r.Method {
	case http.MethodGet:
		orgUserAclGet(reqCtx, w, r)
	case http.MethodPost:
		orgUserAclCreate(reqCtx, w, r)
	case http.MethodPatch, http.MethodPut:
		orgUserAclUpdate(reqCtx, w, r)
	case http.MethodDelete:
		orgUserAclDelete(reqCtx, w, r)
	default:
		response.ErrorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func orgUserAclGet(reqCtx request.RequestContext, w http.ResponseWriter, r *http.Request) {
	reg := resolveOrgUserRegistry(reqCtx.GetDI())
	if reg == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "org user db registry not available")
		return
	}

	_, id := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if id != "" {
		orgDB, err := findUserAclOrgDB(reg, id)
		if err != nil {
			response.ErrorResponse(w, http.StatusNotFound, "acl entry not found")
			return
		}
		var a users_entity.OrganizationUserAcl
		var rolesRaw string
		if err := orgDB.QueryRow(
			`SELECT id, user_id, roles, created_at, is_active
			 FROM organization_user_acl WHERE id = ? LIMIT 1`, id,
		).Scan(&a.ID, &a.UserID, &rolesRaw, &a.CreatedAt, &a.IsActive); err != nil {
			if err == sql.ErrNoRows {
				response.ErrorResponse(w, http.StatusNotFound, "acl entry not found")
				return
			}
			response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		a.Roles = json.RawMessage(rolesRaw)
		response.SuccessResponse(w, http.StatusOK, a)
		return
	}

	userID := strings.TrimSpace(r.URL.Query().Get("filter[@exact:user_id]"))
	if userID == "" {
		userID = strings.TrimSpace(r.URL.Query().Get("user_id"))
	}
	if userID != "" {
		orgDB, _, err := orgrouter.OrgDBFor(reg, userID)
		if err != nil {
			response.ErrorResponse(w, http.StatusNotFound, "user not found")
			return
		}
		view := users_entity.UserScopeView{
			UserID:     userID,
			FromGroups: []users_entity.GroupScopeGrant{},
			FromApps:   []users_entity.AppScopeGrant{},
		}

		// Source 1: direct org-level grant.
		var directID, directCreatedAt, directRolesRaw string
		var directIsActive bool
		err = orgDB.QueryRow(
			`SELECT id, roles, created_at, is_active
			 FROM organization_user_acl WHERE user_id = ? AND is_active = TRUE LIMIT 1`,
			userID,
		).Scan(&directID, &directRolesRaw, &directCreatedAt, &directIsActive)
		if err == nil {
			view.Direct = &users_entity.OrgDirectGrant{
				ID:        directID,
				Roles:     json.RawMessage(directRolesRaw),
				IsActive:  directIsActive,
				CreatedAt: directCreatedAt,
			}
		}

		// Source 2: group inheritance.
		groupRows, err := orgDB.Query(
			`SELECT ug.id, ug.title, oga.roles, oga.is_active
			 FROM organization_group_acl oga
			 JOIN user_group_members ugm ON ugm.group_id = oga.group_id
			 JOIN user_groups ug ON ug.id = oga.group_id
			 WHERE ugm.user_id = ? AND ugm.is_active = TRUE AND oga.is_active = TRUE`,
			userID,
		)
		if err == nil {
			defer groupRows.Close()
			for groupRows.Next() {
				var g users_entity.GroupScopeGrant
				var rolesRaw string
				if groupRows.Scan(&g.GroupID, &g.GroupName, &rolesRaw, &g.IsActive) == nil {
					g.Roles = json.RawMessage(rolesRaw)
					view.FromGroups = append(view.FromGroups, g)
				}
			}
		}

		// Source 3: application grants.
		appRows, err := orgDB.Query(
			`SELECT a.id, a.client_id, acl.roles, acl.is_active
			 FROM application_user_acl acl
			 JOIN applications a ON a.id = acl.application_id
			 WHERE acl.user_id = ? AND acl.is_active = TRUE`,
			userID,
		)
		if err == nil {
			defer appRows.Close()
			for appRows.Next() {
				var a users_entity.AppScopeGrant
				var rolesRaw string
				if appRows.Scan(&a.ApplicationID, &a.ClientID, &rolesRaw, &a.IsActive) == nil {
					a.Roles = json.RawMessage(rolesRaw)
					view.FromApps = append(view.FromApps, a)
				}
			}
		}

		// Build effective_roles: deduplicated union across all three sources.
		seen := make(map[string]struct{})
		addRoles := func(raw json.RawMessage) {
			var roles []string
			if json.Unmarshal(raw, &roles) != nil {
				return
			}
			for _, r := range roles {
				if _, ok := seen[r]; !ok {
					seen[r] = struct{}{}
					view.EffectiveRoles = append(view.EffectiveRoles, r)
				}
			}
		}
		if view.Direct != nil {
			addRoles(view.Direct.Roles)
		}
		for _, g := range view.FromGroups {
			addRoles(g.Roles)
		}
		for _, a := range view.FromApps {
			addRoles(a.Roles)
		}
		if view.EffectiveRoles == nil {
			view.EffectiveRoles = []string{}
		}

		response.SuccessResponse(w, http.StatusOK, view)
		return
	}

	orgID := extractOrgIDFilter(r)
	if orgID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "user_id or organization_id filter is required")
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
		`SELECT id, user_id, roles, created_at, is_active
		 FROM organization_user_acl
		 ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		limit, offset,
	)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	out := []users_entity.OrganizationUserAcl{}
	for rows.Next() {
		var a users_entity.OrganizationUserAcl
		var rolesRaw string
		if err := rows.Scan(&a.ID, &a.UserID, &rolesRaw, &a.CreatedAt, &a.IsActive); err != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		a.Roles = json.RawMessage(rolesRaw)
		out = append(out, a)
	}
	response.SuccessResponse(w, http.StatusOK, out)
}

func orgUserAclCreate(reqCtx request.RequestContext, w http.ResponseWriter, r *http.Request) {
	reg := resolveOrgUserRegistry(reqCtx.GetDI())
	if reg == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "org user db registry not available")
		return
	}

	var body users_entity.OrganizationUserAcl
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(body.UserID) == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "user_id is required")
		return
	}
	if err := (&acl.OrganizationScopeValidator{}).BeforeExecution(reqCtx, &body); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	orgDB, _, err := orgrouter.OrgDBFor(reg, body.UserID)
	if err != nil {
		response.ErrorResponse(w, http.StatusNotFound, "user not found in any organization")
		return
	}

	if body.Roles == nil {
		body.Roles = json.RawMessage("[]")
	}
	id := uuid.New().String()
	if _, err := orgDB.Exec(
		`INSERT INTO organization_user_acl (id, user_id, roles, is_active) VALUES (?, ?, ?, TRUE)
		 ON CONFLICT(user_id) DO UPDATE SET roles = excluded.roles, is_active = TRUE`,
		id, body.UserID, string(body.Roles),
	); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	var a users_entity.OrganizationUserAcl
	var rolesRaw string
	if err := orgDB.QueryRow(
		`SELECT id, user_id, roles, created_at, is_active
		 FROM organization_user_acl WHERE user_id = ? LIMIT 1`, body.UserID,
	).Scan(&a.ID, &a.UserID, &rolesRaw, &a.CreatedAt, &a.IsActive); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.Roles = json.RawMessage(rolesRaw)
	response.SuccessResponse(w, http.StatusCreated, a)
}

func orgUserAclUpdate(reqCtx request.RequestContext, w http.ResponseWriter, r *http.Request) {
	reg := resolveOrgUserRegistry(reqCtx.GetDI())
	if reg == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "org user db registry not available")
		return
	}

	_, id := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if id == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "acl id missing from path")
		return
	}
	orgDB, err := findUserAclOrgDB(reg, id)
	if err != nil {
		response.ErrorResponse(w, http.StatusNotFound, "acl entry not found")
		return
	}

	var body users_entity.OrganizationUserAcl
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := (&acl.OrganizationScopeValidator{}).BeforeExecution(reqCtx, &body); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	if body.Roles != nil {
		if _, err := orgDB.Exec(
			`UPDATE organization_user_acl SET roles = ? WHERE id = ?`, string(body.Roles), id,
		); err != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	var a users_entity.OrganizationUserAcl
	var rolesRaw string
	if err := orgDB.QueryRow(
		`SELECT id, user_id, roles, created_at, is_active
		 FROM organization_user_acl WHERE id = ? LIMIT 1`, id,
	).Scan(&a.ID, &a.UserID, &rolesRaw, &a.CreatedAt, &a.IsActive); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.Roles = json.RawMessage(rolesRaw)
	response.SuccessResponse(w, http.StatusOK, a)
}

func orgUserAclDelete(reqCtx request.RequestContext, w http.ResponseWriter, r *http.Request) {
	reg := resolveOrgUserRegistry(reqCtx.GetDI())
	if reg == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "org user db registry not available")
		return
	}

	_, id := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if id == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "acl id missing from path")
		return
	}
	orgDB, err := findUserAclOrgDB(reg, id)
	if err != nil {
		response.ErrorResponse(w, http.StatusNotFound, "acl entry not found")
		return
	}
	if _, err := orgDB.Exec(
		`UPDATE organization_user_acl SET is_active = FALSE WHERE id = ?`, id,
	); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// findUserAclOrgDB scans KnownOrgIDs to find which org DB holds the given ACL row.
func findUserAclOrgDB(reg *dbregistry.OrgUserDBRegistry, aclID string) (*sql.DB, error) {
	for _, orgID := range reg.KnownOrgIDs() {
		odb, err := orgrouter.ForOrg(reg, orgID)
		if err != nil {
			continue
		}
		var found string
		if odb.QueryRow(`SELECT id FROM organization_user_acl WHERE id = ? LIMIT 1`, aclID).Scan(&found) == nil {
			return odb, nil
		}
	}
	return nil, fmt.Errorf("acl entry %q: org not found", aclID)
}
