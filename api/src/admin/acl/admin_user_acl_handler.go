package acl

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	acl_entity "github.com/a-digi/coco-iam/src/admin/acl/entity"
	"github.com/a-digi/coco-iam/src/admin/acl/repository/persistent"
	"github.com/a-digi/coco-iam/src/admin/acl/repository/query"
	"github.com/a-digi/coco-lift/resource/uri"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
	"github.com/google/uuid"
)

func resolveDB(reqCtx request.RequestContext, w http.ResponseWriter) (*sql.DB, bool) {
	mgr := reqCtx.GetDI().GetDatabaseManager()
	if mgr == nil || mgr.Connector == nil || mgr.Connector.DB == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "database not available")
		return nil, false
	}
	return mgr.Connector.DB, true
}

func CustomAdminUserAclCreate(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	type createBody struct {
		UserID   string          `json:"user_id"`
		Roles    json.RawMessage `json:"roles"`
		IsActive *bool           `json:"is_active"`
	}

	var body createBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(body.UserID) == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "user_id is required")
		return
	}
	if body.Roles == nil {
		response.ErrorResponse(w, http.StatusBadRequest, "roles is required")
		return
	}
	var rolesSlice []string
	if err := json.Unmarshal(body.Roles, &rolesSlice); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "roles must be a JSON array of strings")
		return
	}

	db, ok := resolveDB(reqCtx, w)
	if !ok {
		return
	}
	queryRepo := query.NewAdminAclQueryRepo(db)
	persistentRepo := persistent.NewAdminAclPersistentRepo(db)

	exists, err := queryRepo.AdminUserExists(body.UserID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to verify admin user")
		return
	}
	if !exists {
		response.ErrorResponse(w, http.StatusNotFound, "admin user not found")
		return
	}

	isActive := true
	if body.IsActive != nil {
		isActive = *body.IsActive
	}

	// If an entry already exists for this user, update it instead of inserting.
	existing, err := queryRepo.FindByUserID(body.UserID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to check existing acl entry")
		return
	}
	if len(existing) > 0 {
		if err := persistentRepo.UpdateRoles(existing[0].ID, rolesSlice, isActive); err != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, "failed to update acl entry")
			return
		}
		updated, err := queryRepo.FindByID(existing[0].ID)
		if err != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, "failed to fetch updated entry")
			return
		}
		response.SuccessResponse(w, http.StatusOK, updated)
		return
	}

	rolesJSON, _ := json.Marshal(rolesSlice)
	entry := &acl_entity.AdminAcl{
		ID:       uuid.New().String(),
		UserID:   body.UserID,
		Roles:    json.RawMessage(rolesJSON),
		IsActive: isActive,
	}
	if err := persistentRepo.Insert(entry); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to create acl entry")
		return
	}

	created, err := queryRepo.FindByID(entry.ID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to fetch created entry")
		return
	}
	response.SuccessResponse(w, http.StatusCreated, created)
}

func CustomAdminUserAclGet(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	db, ok := resolveDB(reqCtx, w)
	if !ok {
		return
	}
	queryRepo := query.NewAdminAclQueryRepo(db)

	_, id := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if id != "" {
		entry, err := queryRepo.FindByID(id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				response.ErrorResponse(w, http.StatusNotFound, "acl entry not found")
				return
			}
			response.ErrorResponse(w, http.StatusInternalServerError, "failed to fetch acl entry")
			return
		}
		response.SuccessResponse(w, http.StatusOK, entry)
		return
	}

	userID := strings.TrimSpace(r.URL.Query().Get("filter[@exact:user_id]"))
	if userID == "" {
		userID = strings.TrimSpace(r.URL.Query().Get("user_id"))
	}
	if userID != "" {
		entries, err := queryRepo.FindByUserID(userID)
		if err != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, "failed to fetch acl entries")
			return
		}
		if entries == nil {
			entries = []*acl_entity.AdminAcl{}
		}
		response.SuccessResponse(w, http.StatusOK, entries)
		return
	}

	entries, err := queryRepo.List()
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to fetch acl entries")
		return
	}
	if entries == nil {
		entries = []*acl_entity.AdminAcl{}
	}
	response.SuccessResponse(w, http.StatusOK, entries)
}

func CustomAdminUserAclUpdate(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	_, id := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if id == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "acl id missing from path")
		return
	}

	db, ok := resolveDB(reqCtx, w)
	if !ok {
		return
	}
	queryRepo := query.NewAdminAclQueryRepo(db)
	persistentRepo := persistent.NewAdminAclPersistentRepo(db)

	existing, err := queryRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.ErrorResponse(w, http.StatusNotFound, "acl entry not found")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to fetch acl entry")
		return
	}

	type updateBody struct {
		Roles    []string `json:"roles"`
		IsActive *bool    `json:"is_active"`
	}

	var body updateBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	isActive := existing.IsActive
	if body.IsActive != nil {
		isActive = *body.IsActive
	}
	roles := body.Roles
	if roles == nil {
		var parsed []string
		if err := json.Unmarshal(existing.Roles, &parsed); err != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, "stored roles data is corrupt")
			return
		}
		roles = parsed
	}

	if err := persistentRepo.UpdateRoles(id, roles, isActive); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to update acl entry")
		return
	}

	updated, err := queryRepo.FindByID(id)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to fetch updated entry")
		return
	}
	response.SuccessResponse(w, http.StatusOK, updated)
}

func CustomAdminUserAclDelete(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	_, id := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if id == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "acl id missing from path")
		return
	}

	db, ok := resolveDB(reqCtx, w)
	if !ok {
		return
	}
	queryRepo := query.NewAdminAclQueryRepo(db)
	persistentRepo := persistent.NewAdminAclPersistentRepo(db)

	_, err := queryRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.ErrorResponse(w, http.StatusNotFound, "acl entry not found")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to fetch acl entry")
		return
	}

	if err := persistentRepo.Delete(id); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to delete acl entry")
		return
	}
	response.SuccessResponse(w, http.StatusOK, map[string]string{"status": "deleted"})
}
