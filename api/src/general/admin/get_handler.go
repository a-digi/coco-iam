package admin

import (
	"net/http"

	"github.com/a-digi/coco-iam/src/general"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// AdminGeneralSettingsGetHandler serves GET /api/v1/admin/settings/general.
// Returns the four global branding fields from the main DB app_settings table.
type AdminGeneralSettingsGetHandler struct{}

func (h *AdminGeneralSettingsGetHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	store := resolveGlobalStore(reqCtx, w)
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

func resolveGlobalStore(reqCtx request.RequestContext, w http.ResponseWriter) *general.Store {
	ctx := reqCtx.GetDI()
	bag, ok := ctx.(bagGetter)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "DI context has unexpected type")
		return nil
	}
	raw, ok := bag.Get(general.ContextBagKeyStore)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "global settings store not available")
		return nil
	}
	store, ok := raw.(*general.Store)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "global settings store has unexpected type")
		return nil
	}
	return store
}
