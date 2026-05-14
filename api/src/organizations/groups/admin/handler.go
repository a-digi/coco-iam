package admin

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/a-digi/coco-iam/src/organizations/acl"
	"github.com/a-digi/coco-iam/src/organizations/groups/entity"
	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	users_entity "github.com/a-digi/coco-iam/src/organizations/users/entity"
	"github.com/a-digi/coco-iam/src/orgrouter"
	"github.com/a-digi/coco-lift/resource/uri"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
	"github.com/google/uuid"
)

// CustomOrgGroupsHandler handles all CRUD for organization groups routed to
// the per-org users.db. Dispatches by HTTP method.
func CustomOrgGroupsHandler(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	switch r.Method {
	case http.MethodGet:
		orgGroupsGet(reqCtx, w, r)
	case http.MethodPost:
		orgGroupsCreate(reqCtx, w, r)
	case http.MethodPatch, http.MethodPut:
		orgGroupsUpdate(reqCtx, w, r)
	case http.MethodDelete:
		orgGroupsDelete(reqCtx, w, r)
	default:
		response.ErrorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// CustomOrgGroupMembersHandler handles all CRUD for org group members.
func CustomOrgGroupMembersHandler(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	switch r.Method {
	case http.MethodGet:
		orgGroupMembersGet(reqCtx, w, r)
	case http.MethodPost:
		orgGroupMembersCreate(reqCtx, w, r)
	case http.MethodPatch, http.MethodPut:
		orgGroupMembersUpdate(reqCtx, w, r)
	case http.MethodDelete:
		orgGroupMembersDelete(reqCtx, w, r)
	default:
		response.ErrorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// CustomOrgGroupAclHandler handles all CRUD for org group ACL, enforcing
// OrganizationScopeValidator before write operations.
func CustomOrgGroupAclHandler(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	switch r.Method {
	case http.MethodGet:
		orgGroupAclGet(reqCtx, w, r)
	case http.MethodPost:
		orgGroupAclCreate(reqCtx, w, r)
	case http.MethodPatch, http.MethodPut:
		orgGroupAclUpdate(reqCtx, w, r)
	case http.MethodDelete:
		orgGroupAclDelete(reqCtx, w, r)
	default:
		response.ErrorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// --- org groups ----------------------------------------------------------

func orgGroupsGet(reqCtx request.RequestContext, w http.ResponseWriter, r *http.Request) {
	reg := resolveRegistry(reqCtx, w)
	if reg == nil {
		return
	}
	_, id := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if id != "" {
		orgDB, err := findGroupOrgDB(reg, id)
		if err != nil {
			response.ErrorResponse(w, http.StatusNotFound, "group not found")
			return
		}
		var g entity.OrganizationGroup
		if err := orgDB.QueryRow(
			`SELECT id, title, group_description, organization_id, created_at, is_active
			 FROM user_groups WHERE id = ? LIMIT 1`, id,
		).Scan(&g.ID, &g.Title, &g.GroupDescription, &g.OrganizationID, &g.CreatedAt, &g.IsActive); err != nil {
			if err == sql.ErrNoRows {
				response.ErrorResponse(w, http.StatusNotFound, "group not found")
				return
			}
			response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		response.SuccessResponse(w, http.StatusOK, g)
		return
	}

	orgID := extractOrgIDFilter(r)
	if orgID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "organization_id filter is required")
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
		`SELECT id, title, group_description, organization_id, created_at, is_active
		 FROM user_groups WHERE organization_id = ?
		 ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		orgID, limit, offset,
	)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	out := []entity.OrganizationGroup{}
	for rows.Next() {
		var g entity.OrganizationGroup
		if err := rows.Scan(&g.ID, &g.Title, &g.GroupDescription, &g.OrganizationID, &g.CreatedAt, &g.IsActive); err != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		out = append(out, g)
	}
	response.SuccessResponse(w, http.StatusOK, out)
}

func orgGroupsCreate(reqCtx request.RequestContext, w http.ResponseWriter, r *http.Request) {
	reg := resolveRegistry(reqCtx, w)
	if reg == nil {
		return
	}
	var body entity.OrganizationGroup
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(body.Title) == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "title is required")
		return
	}
	if strings.TrimSpace(body.OrganizationID) == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "organization_id is required")
		return
	}
	orgDB, err := orgrouter.ForOrg(reg, body.OrganizationID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to open org db: "+err.Error())
		return
	}
	id := uuid.New().String()
	if _, err := orgDB.Exec(
		`INSERT INTO user_groups (id, title, group_description, organization_id, is_active)
		 VALUES (?, ?, ?, ?, TRUE)`,
		id, body.Title, body.GroupDescription, body.OrganizationID,
	); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	var g entity.OrganizationGroup
	_ = orgDB.QueryRow(
		`SELECT id, title, group_description, organization_id, created_at, is_active
		 FROM user_groups WHERE id = ? LIMIT 1`, id,
	).Scan(&g.ID, &g.Title, &g.GroupDescription, &g.OrganizationID, &g.CreatedAt, &g.IsActive)
	response.SuccessResponse(w, http.StatusCreated, g)
}

func orgGroupsUpdate(reqCtx request.RequestContext, w http.ResponseWriter, r *http.Request) {
	reg := resolveRegistry(reqCtx, w)
	if reg == nil {
		return
	}
	_, id := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if id == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "group id missing from path")
		return
	}
	orgDB, err := findGroupOrgDB(reg, id)
	if err != nil {
		response.ErrorResponse(w, http.StatusNotFound, "group not found")
		return
	}
	var body struct {
		Title            *string `json:"title,omitempty"`
		GroupDescription *string `json:"group_description,omitempty"`
		IsActive         *bool   `json:"is_active,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON body")
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
	if len(sets) > 0 {
		args = append(args, id)
		if _, err := orgDB.Exec(
			`UPDATE user_groups SET `+strings.Join(sets, ", ")+` WHERE id = ?`, args...,
		); err != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	var g entity.OrganizationGroup
	_ = orgDB.QueryRow(
		`SELECT id, title, group_description, organization_id, created_at, is_active
		 FROM user_groups WHERE id = ? LIMIT 1`, id,
	).Scan(&g.ID, &g.Title, &g.GroupDescription, &g.OrganizationID, &g.CreatedAt, &g.IsActive)
	response.SuccessResponse(w, http.StatusOK, g)
}

func orgGroupsDelete(reqCtx request.RequestContext, w http.ResponseWriter, r *http.Request) {
	reg := resolveRegistry(reqCtx, w)
	if reg == nil {
		return
	}
	_, id := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if id == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "group id missing from path")
		return
	}
	orgDB, err := findGroupOrgDB(reg, id)
	if err != nil {
		response.ErrorResponse(w, http.StatusNotFound, "group not found")
		return
	}
	if _, err := orgDB.Exec(`UPDATE user_groups SET is_active = FALSE WHERE id = ?`, id); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// --- org group members ---------------------------------------------------

func orgGroupMembersGet(reqCtx request.RequestContext, w http.ResponseWriter, r *http.Request) {
	reg := resolveRegistry(reqCtx, w)
	if reg == nil {
		return
	}
	_, id := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if id != "" {
		orgDB, err := findGroupMemberOrgDB(reg, id)
		if err != nil {
			response.ErrorResponse(w, http.StatusNotFound, "member not found")
			return
		}
		var m entity.OrganizationGroupMember
		var uID, uUsername, uEmail sql.NullString
		var uIsActive sql.NullBool
		if err := orgDB.QueryRow(
			`SELECT m.id, m.user_id, m.group_id, m.created_at, m.is_active,
			        u.id, u.username, u.email, u.is_active
			 FROM user_group_members m
			 LEFT JOIN users u ON u.id = m.user_id
			 WHERE m.id = ? LIMIT 1`, id,
		).Scan(&m.ID, &m.UserID, &m.GroupID, &m.CreatedAt, &m.IsActive,
			&uID, &uUsername, &uEmail, &uIsActive); err != nil {
			if err == sql.ErrNoRows {
				response.ErrorResponse(w, http.StatusNotFound, "member not found")
				return
			}
			response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		if uID.Valid {
			m.User = &users_entity.User{
				ID:       uID.String,
				Username: uUsername.String,
				Email:    uEmail.String,
				IsActive: uIsActive.Bool,
			}
		}
		response.SuccessResponse(w, http.StatusOK, m)
		return
	}

	groupID := strings.TrimSpace(r.URL.Query().Get("filter[@exact:group_id]"))
	if groupID == "" {
		groupID = strings.TrimSpace(r.URL.Query().Get("group_id"))
	}
	if groupID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "group_id filter is required")
		return
	}
	orgDB, err := findGroupOrgDB(reg, groupID)
	if err != nil {
		response.ErrorResponse(w, http.StatusNotFound, "group not found")
		return
	}
	limit := parseLimitParam(r.URL.Query().Get("limit"), 50)
	page := parsePageParam(r.URL.Query().Get("page"), 1)
	offset := (page - 1) * limit

	rows, err := orgDB.Query(
		`SELECT m.id, m.user_id, m.group_id, m.created_at, m.is_active,
		        u.id, u.username, u.email, u.is_active
		 FROM user_group_members m
		 LEFT JOIN users u ON u.id = m.user_id
		 WHERE m.group_id = ?
		 ORDER BY m.created_at DESC LIMIT ? OFFSET ?`,
		groupID, limit, offset,
	)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	out := []entity.OrganizationGroupMember{}
	for rows.Next() {
		var m entity.OrganizationGroupMember
		var uID, uUsername, uEmail sql.NullString
		var uIsActive sql.NullBool
		if err := rows.Scan(&m.ID, &m.UserID, &m.GroupID, &m.CreatedAt, &m.IsActive,
			&uID, &uUsername, &uEmail, &uIsActive); err != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		if uID.Valid {
			m.User = &users_entity.User{
				ID:       uID.String,
				Username: uUsername.String,
				Email:    uEmail.String,
				IsActive: uIsActive.Bool,
			}
		}
		out = append(out, m)
	}
	response.SuccessResponse(w, http.StatusOK, out)
}

func orgGroupMembersCreate(reqCtx request.RequestContext, w http.ResponseWriter, r *http.Request) {
	reg := resolveRegistry(reqCtx, w)
	if reg == nil {
		return
	}
	var body entity.OrganizationGroupMember
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(body.GroupID) == "" || strings.TrimSpace(body.UserID) == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "group_id and user_id are required")
		return
	}
	orgDB, err := findGroupOrgDB(reg, body.GroupID)
	if err != nil {
		response.ErrorResponse(w, http.StatusNotFound, "group not found")
		return
	}
	id := uuid.New().String()
	if _, err := orgDB.Exec(
		`INSERT INTO user_group_members (id, user_id, group_id, is_active) VALUES (?, ?, ?, TRUE)`,
		id, body.UserID, body.GroupID,
	); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	var m entity.OrganizationGroupMember
	_ = orgDB.QueryRow(
		`SELECT id, user_id, group_id, created_at, is_active
		 FROM user_group_members WHERE id = ? LIMIT 1`, id,
	).Scan(&m.ID, &m.UserID, &m.GroupID, &m.CreatedAt, &m.IsActive)
	response.SuccessResponse(w, http.StatusCreated, m)
}

func orgGroupMembersUpdate(reqCtx request.RequestContext, w http.ResponseWriter, r *http.Request) {
	reg := resolveRegistry(reqCtx, w)
	if reg == nil {
		return
	}
	_, id := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if id == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "member id missing from path")
		return
	}
	orgDB, err := findGroupMemberOrgDB(reg, id)
	if err != nil {
		response.ErrorResponse(w, http.StatusNotFound, "member not found")
		return
	}
	var body struct {
		IsActive *bool `json:"is_active,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.IsActive != nil {
		if _, err := orgDB.Exec(
			`UPDATE user_group_members SET is_active = ? WHERE id = ?`, *body.IsActive, id,
		); err != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	var m entity.OrganizationGroupMember
	_ = orgDB.QueryRow(
		`SELECT id, user_id, group_id, created_at, is_active
		 FROM user_group_members WHERE id = ? LIMIT 1`, id,
	).Scan(&m.ID, &m.UserID, &m.GroupID, &m.CreatedAt, &m.IsActive)
	response.SuccessResponse(w, http.StatusOK, m)
}

func orgGroupMembersDelete(reqCtx request.RequestContext, w http.ResponseWriter, r *http.Request) {
	reg := resolveRegistry(reqCtx, w)
	if reg == nil {
		return
	}
	_, id := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if id == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "member id missing from path")
		return
	}
	orgDB, err := findGroupMemberOrgDB(reg, id)
	if err != nil {
		response.ErrorResponse(w, http.StatusNotFound, "member not found")
		return
	}
	if _, err := orgDB.Exec(`UPDATE user_group_members SET is_active = FALSE WHERE id = ?`, id); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// --- org group ACL -------------------------------------------------------

func orgGroupAclGet(reqCtx request.RequestContext, w http.ResponseWriter, r *http.Request) {
	reg := resolveRegistry(reqCtx, w)
	if reg == nil {
		return
	}
	_, id := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if id != "" {
		orgDB, err := findGroupAclOrgDB(reg, id)
		if err != nil {
			response.ErrorResponse(w, http.StatusNotFound, "acl entry not found")
			return
		}
		var a entity.OrganizationGroupAcl
		var rolesRaw0 string
		if err := orgDB.QueryRow(
			`SELECT id, group_id, roles, created_at, is_active
			 FROM organization_group_acl WHERE id = ? LIMIT 1`, id,
		).Scan(&a.ID, &a.GroupID, &rolesRaw0, &a.CreatedAt, &a.IsActive); err != nil {
			if err == sql.ErrNoRows {
				response.ErrorResponse(w, http.StatusNotFound, "acl entry not found")
				return
			}
			response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		a.Roles = json.RawMessage(rolesRaw0)
		response.SuccessResponse(w, http.StatusOK, a)
		return
	}

	groupID := strings.TrimSpace(r.URL.Query().Get("filter[@exact:group_id]"))
	if groupID == "" {
		groupID = strings.TrimSpace(r.URL.Query().Get("group_id"))
	}
	if groupID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "group_id filter is required")
		return
	}
	orgDB, err := findGroupOrgDB(reg, groupID)
	if err != nil {
		response.ErrorResponse(w, http.StatusNotFound, "group not found")
		return
	}
	rows, err := orgDB.Query(
		`SELECT id, group_id, roles, created_at, is_active
		 FROM organization_group_acl WHERE group_id = ?`, groupID,
	)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	out := []entity.OrganizationGroupAcl{}
	for rows.Next() {
		var a entity.OrganizationGroupAcl
		var rolesRaw string
		if err := rows.Scan(&a.ID, &a.GroupID, &rolesRaw, &a.CreatedAt, &a.IsActive); err != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		a.Roles = json.RawMessage(rolesRaw)
		out = append(out, a)
	}
	response.SuccessResponse(w, http.StatusOK, out)
}

func orgGroupAclCreate(reqCtx request.RequestContext, w http.ResponseWriter, r *http.Request) {
	reg := resolveRegistry(reqCtx, w)
	if reg == nil {
		return
	}
	var body entity.OrganizationGroupAcl
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := (&acl.OrganizationScopeValidator{}).BeforeExecution(reqCtx, &body); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(body.GroupID) == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "group_id is required")
		return
	}
	orgDB, err := findGroupOrgDB(reg, body.GroupID)
	if err != nil {
		response.ErrorResponse(w, http.StatusNotFound, "group not found")
		return
	}
	if body.Roles == nil {
		body.Roles = json.RawMessage("[]")
	}
	id := uuid.New().String()
	if _, err := orgDB.Exec(
		`INSERT INTO organization_group_acl (id, group_id, roles, is_active) VALUES (?, ?, ?, TRUE)`,
		id, body.GroupID, string(body.Roles),
	); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	var a entity.OrganizationGroupAcl
	var rolesRawCreate string
	_ = orgDB.QueryRow(
		`SELECT id, group_id, roles, created_at, is_active
		 FROM organization_group_acl WHERE id = ? LIMIT 1`, id,
	).Scan(&a.ID, &a.GroupID, &rolesRawCreate, &a.CreatedAt, &a.IsActive)
	a.Roles = json.RawMessage(rolesRawCreate)
	response.SuccessResponse(w, http.StatusCreated, a)
}

func orgGroupAclUpdate(reqCtx request.RequestContext, w http.ResponseWriter, r *http.Request) {
	reg := resolveRegistry(reqCtx, w)
	if reg == nil {
		return
	}
	_, id := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if id == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "acl id missing from path")
		return
	}
	orgDB, err := findGroupAclOrgDB(reg, id)
	if err != nil {
		response.ErrorResponse(w, http.StatusNotFound, "acl entry not found")
		return
	}
	var body entity.OrganizationGroupAcl
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
			`UPDATE organization_group_acl SET roles = ? WHERE id = ?`, string(body.Roles), id,
		); err != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	var a entity.OrganizationGroupAcl
	var rolesRawUpdate string
	_ = orgDB.QueryRow(
		`SELECT id, group_id, roles, created_at, is_active
		 FROM organization_group_acl WHERE id = ? LIMIT 1`, id,
	).Scan(&a.ID, &a.GroupID, &rolesRawUpdate, &a.CreatedAt, &a.IsActive)
	a.Roles = json.RawMessage(rolesRawUpdate)
	response.SuccessResponse(w, http.StatusOK, a)
}

func orgGroupAclDelete(reqCtx request.RequestContext, w http.ResponseWriter, r *http.Request) {
	reg := resolveRegistry(reqCtx, w)
	if reg == nil {
		return
	}
	_, id := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if id == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "acl id missing from path")
		return
	}
	orgDB, err := findGroupAclOrgDB(reg, id)
	if err != nil {
		response.ErrorResponse(w, http.StatusNotFound, "acl entry not found")
		return
	}
	if _, err := orgDB.Exec(
		`UPDATE organization_group_acl SET is_active = FALSE WHERE id = ?`, id,
	); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// --- helpers -------------------------------------------------------------

func resolveRegistry(reqCtx request.RequestContext, w http.ResponseWriter) *dbregistry.OrgUserDBRegistry {
	ctx := reqCtx.GetDI()
	bag, ok := ctx.(interface {
		Get(string) (interface{}, bool)
	})
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "di context not available")
		return nil
	}
	raw, ok := bag.Get(dbregistry.ContextBagKey)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "org user db registry not available")
		return nil
	}
	reg, _ := raw.(*dbregistry.OrgUserDBRegistry)
	if reg == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "org user db registry not available")
		return nil
	}
	return reg
}

// findGroupOrgDB scans KnownOrgIDs to find which org DB holds the given group.
func findGroupOrgDB(reg *dbregistry.OrgUserDBRegistry, groupID string) (*sql.DB, error) {
	for _, orgID := range reg.KnownOrgIDs() {
		odb, err := orgrouter.ForOrg(reg, orgID)
		if err != nil {
			continue
		}
		var found string
		if odb.QueryRow(`SELECT id FROM user_groups WHERE id = ? LIMIT 1`, groupID).Scan(&found) == nil {
			return odb, nil
		}
	}
	return nil, fmt.Errorf("group %q: org not found", groupID)
}

// findGroupMemberOrgDB finds the org DB that holds the given member row.
func findGroupMemberOrgDB(reg *dbregistry.OrgUserDBRegistry, memberID string) (*sql.DB, error) {
	for _, orgID := range reg.KnownOrgIDs() {
		odb, err := orgrouter.ForOrg(reg, orgID)
		if err != nil {
			continue
		}
		var found string
		if odb.QueryRow(`SELECT id FROM user_group_members WHERE id = ? LIMIT 1`, memberID).Scan(&found) == nil {
			return odb, nil
		}
	}
	return nil, fmt.Errorf("group member %q: org not found", memberID)
}

// findGroupAclOrgDB finds the org DB that holds the given ACL row.
func findGroupAclOrgDB(reg *dbregistry.OrgUserDBRegistry, aclID string) (*sql.DB, error) {
	for _, orgID := range reg.KnownOrgIDs() {
		odb, err := orgrouter.ForOrg(reg, orgID)
		if err != nil {
			continue
		}
		var found string
		if odb.QueryRow(`SELECT id FROM organization_group_acl WHERE id = ? LIMIT 1`, aclID).Scan(&found) == nil {
			return odb, nil
		}
	}
	return nil, fmt.Errorf("group acl %q: org not found", aclID)
}

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
