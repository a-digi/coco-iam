package admin

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	acl_entity "github.com/a-digi/coco-iam/src/admin/acl/entity"
	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-iam/src/orgrouter"
	"github.com/a-digi/coco-lift/resource/uri"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
	"github.com/google/uuid"
)

// CustomOrgUserBaseAclHandler handles all CRUD for user_acl routed to the
// per-org users.db. Dispatches by HTTP method.
func CustomOrgUserBaseAclHandler(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	switch r.Method {
	case http.MethodGet:
		orgUserBaseAclGet(reqCtx, w, r)
	case http.MethodPost:
		orgUserBaseAclCreate(reqCtx, w, r)
	case http.MethodPatch, http.MethodPut:
		orgUserBaseAclUpdate(reqCtx, w, r)
	case http.MethodDelete:
		orgUserBaseAclDelete(reqCtx, w, r)
	default:
		response.ErrorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func orgUserBaseAclGet(reqCtx request.RequestContext, w http.ResponseWriter, r *http.Request) {
	reg := resolveOrgUserRegistry(reqCtx.GetDI())
	if reg == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "org user db registry not available")
		return
	}

	_, id := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if id != "" {
		orgDB, err := findUserBaseAclOrgDB(reg, id)
		if err != nil {
			response.ErrorResponse(w, http.StatusNotFound, "acl entry not found")
			return
		}
		var a acl_entity.UserAcl
		if err := orgDB.QueryRow(
			`SELECT id, user_id, roles, created_at, is_active
			 FROM user_acl WHERE id = ? LIMIT 1`, id,
		).Scan(&a.ID, &a.UserID, &a.Roles, &a.CreatedAt, &a.IsActive); err != nil {
			if err == sql.ErrNoRows {
				response.ErrorResponse(w, http.StatusNotFound, "acl entry not found")
				return
			}
			response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
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
		rows, err := orgDB.Query(
			`SELECT id, user_id, roles, created_at, is_active
			 FROM user_acl WHERE user_id = ?`, userID,
		)
		if err != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		defer rows.Close()
		out := []acl_entity.UserAcl{}
		for rows.Next() {
			var a acl_entity.UserAcl
			if err := rows.Scan(&a.ID, &a.UserID, &a.Roles, &a.CreatedAt, &a.IsActive); err != nil {
				response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			out = append(out, a)
		}
		response.SuccessResponse(w, http.StatusOK, out)
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
		 FROM user_acl
		 ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		limit, offset,
	)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	out := []acl_entity.UserAcl{}
	for rows.Next() {
		var a acl_entity.UserAcl
		if err := rows.Scan(&a.ID, &a.UserID, &a.Roles, &a.CreatedAt, &a.IsActive); err != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		out = append(out, a)
	}
	response.SuccessResponse(w, http.StatusOK, out)
}

func orgUserBaseAclCreate(reqCtx request.RequestContext, w http.ResponseWriter, r *http.Request) {
	reg := resolveOrgUserRegistry(reqCtx.GetDI())
	if reg == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "org user db registry not available")
		return
	}

	var body acl_entity.UserAcl
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(body.UserID) == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "user_id is required")
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
		`INSERT INTO user_acl (id, user_id, roles, is_active) VALUES (?, ?, ?, TRUE)`,
		id, body.UserID, string(body.Roles),
	); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	var a acl_entity.UserAcl
	if err := orgDB.QueryRow(
		`SELECT id, user_id, roles, created_at, is_active
		 FROM user_acl WHERE id = ? LIMIT 1`, id,
	).Scan(&a.ID, &a.UserID, &a.Roles, &a.CreatedAt, &a.IsActive); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusCreated, a)
}

func orgUserBaseAclUpdate(reqCtx request.RequestContext, w http.ResponseWriter, r *http.Request) {
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
	orgDB, err := findUserBaseAclOrgDB(reg, id)
	if err != nil {
		response.ErrorResponse(w, http.StatusNotFound, "acl entry not found")
		return
	}

	var body acl_entity.UserAcl
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if body.Roles != nil {
		if _, err := orgDB.Exec(
			`UPDATE user_acl SET roles = ? WHERE id = ?`, string(body.Roles), id,
		); err != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	var a acl_entity.UserAcl
	if err := orgDB.QueryRow(
		`SELECT id, user_id, roles, created_at, is_active
		 FROM user_acl WHERE id = ? LIMIT 1`, id,
	).Scan(&a.ID, &a.UserID, &a.Roles, &a.CreatedAt, &a.IsActive); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, a)
}

func orgUserBaseAclDelete(reqCtx request.RequestContext, w http.ResponseWriter, r *http.Request) {
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
	orgDB, err := findUserBaseAclOrgDB(reg, id)
	if err != nil {
		response.ErrorResponse(w, http.StatusNotFound, "acl entry not found")
		return
	}
	if _, err := orgDB.Exec(
		`UPDATE user_acl SET is_active = FALSE WHERE id = ?`, id,
	); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// findUserBaseAclOrgDB scans KnownOrgIDs to find which org DB holds the given user_acl row.
func findUserBaseAclOrgDB(reg *dbregistry.OrgUserDBRegistry, aclID string) (*sql.DB, error) {
	for _, orgID := range reg.KnownOrgIDs() {
		odb, err := orgrouter.ForOrg(reg, orgID)
		if err != nil {
			continue
		}
		var found string
		if odb.QueryRow(`SELECT id FROM user_acl WHERE id = ? LIMIT 1`, aclID).Scan(&found) == nil {
			return odb, nil
		}
	}
	return nil, fmt.Errorf("acl entry %q: org not found", aclID)
}
