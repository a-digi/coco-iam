package admin

import (
	"errors"
	"net/http"

	"github.com/a-digi/coco-iam/src/applications/apicredentials/repository"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// RevokeHandler serves POST
// /api/v1/applications/{id}/api-credentials/{credId}/revoke.
// Soft-revokes: the row stays in the DB with is_active=0 and
// revoked_at set so audit tools can still see who held what.
type RevokeHandler struct{}

func (h *RevokeHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	appID := appIDFromPath(reqCtx)
	credID := credIDFromPath(r.URL.Path)
	if appID == "" || credID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "missing application id or credential id")
		return
	}

	repo, _, ok := openRepoForApp(reqCtx, appID)
	if !ok {
		return
	}

	if err := repo.Revoke(credID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			response.ErrorResponse(w, http.StatusNotFound, "credential not found or already revoked")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, map[string]string{"status": "revoked"})
}
