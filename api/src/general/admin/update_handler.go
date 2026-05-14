package admin

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/a-digi/coco-iam/src/general"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// AdminGeneralSettingsUpdateHandler serves PATCH /api/v1/admin/settings/general.
// Any field may be omitted; a present string replaces the stored value.
type AdminGeneralSettingsUpdateHandler struct{}

func (h *AdminGeneralSettingsUpdateHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	store := resolveGlobalStore(reqCtx, w)
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
