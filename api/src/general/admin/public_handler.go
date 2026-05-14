package admin

import (
	"database/sql"
	"net/http"

	"github.com/a-digi/coco-iam/src/general"
	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-iam/src/orgrouter"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// PublicGeneralSettingsHandler serves GET /api/v1/public/general.
// Requires an ?application_id= query parameter to identify the
// organization whose branding should be returned. Responds 400 when the
// parameter is absent and 404 when the application cannot be resolved.
type PublicGeneralSettingsHandler struct{}

func (h *PublicGeneralSettingsHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	applicationID := r.URL.Query().Get("application_id")
	if applicationID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "application_id query parameter is required")
		return
	}

	orgDB := orgDBForApp(reqCtx, applicationID)
	if orgDB == nil {
		response.ErrorResponse(w, http.StatusNotFound, "application not found")
		return
	}

	snap, err := general.NewStoreFromDB(orgDB).Snapshot()
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, snap)
}

// orgDBForApp scans per-org DBs to find the one that owns applicationID
// and returns it, or nil on any failure.
func orgDBForApp(reqCtx request.RequestContext, applicationID string) *sql.DB {
	diCtx := reqCtx.GetDI()
	bag, ok := diCtx.(bagGetter)
	if !ok {
		return nil
	}
	regRaw, ok := bag.Get(dbregistry.ContextBagKey)
	if !ok {
		return nil
	}
	reg, ok := regRaw.(*dbregistry.OrgUserDBRegistry)
	if !ok {
		return nil
	}
	orgDB, _, err := orgrouter.OrgDBForApp(reg, applicationID)
	if err != nil {
		return nil
	}
	return orgDB
}
