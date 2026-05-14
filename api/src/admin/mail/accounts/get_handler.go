package accounts

import (
	"errors"
	"net/http"

	"github.com/a-digi/coco-lift/resource/uri"
	mailaccounts "github.com/a-digi/coco-iam/src/mail/accounts"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// AdminMailAccountsGetHandler serves GET /api/v1/admin/mail/accounts/{id}.
type AdminMailAccountsGetHandler struct{}

func (h *AdminMailAccountsGetHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	key, value := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if key != "id" || value == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "account id is required")
		return
	}
	store := resolveStore(reqCtx)
	if store == nil {
		return
	}
	acc, err := store.Get(value)
	if err != nil {
		if errors.Is(err, mailaccounts.ErrNotFound) {
			response.ErrorResponse(w, http.StatusNotFound, "account not found")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	redacted := acc.Redacted()
	response.SuccessResponse(w, http.StatusOK, redacted)
}
