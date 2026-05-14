package admin

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/a-digi/coco-iam/src/general"
	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-iam/src/orgrouter"
	"github.com/a-digi/coco-lift/resource/uri"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// OrgGeneralSettingsGetHandler serves
// GET /api/v1/admin/organizations/{id:<org_id>}/settings/general.
// Returns the four branding fields from the org's per-org DB.
type OrgGeneralSettingsGetHandler struct{}

func (h *OrgGeneralSettingsGetHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	store := resolveOrgStore(reqCtx, w)
	if store == nil {
		return
	}
	snap, err := store.Snapshot()
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, snap)
}

type updateRequest struct {
	BaseURL     *string `json:"base_url,omitempty"`
	PageTitle   *string `json:"page_title,omitempty"`
	Description *string `json:"description,omitempty"`
	Robots      *string `json:"robots,omitempty"`
}

// OrgGeneralSettingsUpdateHandler serves
// PATCH /api/v1/admin/organizations/{id:<org_id>}/settings/general.
// Any field may be omitted; a present string replaces the stored value.
type OrgGeneralSettingsUpdateHandler struct{}

func (h *OrgGeneralSettingsUpdateHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	store := resolveOrgStore(reqCtx, w)
	if store == nil {
		return
	}

	var req updateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	defer r.Body.Close()

	updates := map[string]string{}

	if req.BaseURL != nil {
		v := strings.TrimSpace(*req.BaseURL)
		if v != "" {
			if !strings.HasPrefix(v, "http://") && !strings.HasPrefix(v, "https://") {
				response.ErrorResponse(w, http.StatusBadRequest,
					"base_url must start with http:// or https://")
				return
			}
		}
		updates[general.KeyBaseURL] = strings.TrimRight(v, "/")
	}
	if req.PageTitle != nil {
		updates[general.KeyPageTitle] = strings.TrimSpace(*req.PageTitle)
	}
	if req.Description != nil {
		updates[general.KeyDescription] = strings.TrimSpace(*req.Description)
	}
	if req.Robots != nil {
		updates[general.KeyRobots] = strings.TrimSpace(*req.Robots)
	}

	if len(updates) > 0 {
		if err := store.SetMany(updates); err != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	snap, err := store.Snapshot()
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, snap)
}

// resolveOrgStore opens the per-org DB for the org whose ID is in the
// request path and returns a general.Store backed by it.
func resolveOrgStore(reqCtx request.RequestContext, w http.ResponseWriter) *general.Store {
	r := reqCtx.GetRequest()
	_, orgID := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	orgID = strings.TrimSpace(orgID)
	if orgID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "org id missing from path")
		return nil
	}

	ctx := reqCtx.GetDI()
	bag, ok := ctx.(bagGetter)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "DI context has unexpected type")
		return nil
	}
	raw, ok := bag.Get(dbregistry.ContextBagKey)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "org db registry not available")
		return nil
	}
	reg, ok := raw.(*dbregistry.OrgUserDBRegistry)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "org db registry has unexpected type")
		return nil
	}

	orgDB, err := orgrouter.ForOrg(reg, orgID)
	if err != nil {
		response.ErrorResponse(w, http.StatusNotFound, "organization not found")
		return nil
	}
	return general.NewStoreFromDB(orgDB)
}
