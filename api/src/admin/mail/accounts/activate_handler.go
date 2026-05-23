package accounts

import (
	"errors"
	"net/http"

	"github.com/a-digi/coco-lift/resource/uri"
	mailaccounts "github.com/a-digi/coco-iam/src/mail/accounts"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// AdminMailAccountsActivateHandler serves
// POST /api/v1/admin/mail/accounts/{id}/activate. Demotes whatever was
// active and promotes the requested id inside a single transaction.
type AdminMailAccountsActivateHandler struct{}

// @Summary     Activate a mail account
// @Tags        admin-mail
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Account ID"
// @Success     200 {object} interface{}
// @Failure     400,401,403,500 {object} map[string]interface{}
// @Router      /admin/mail/accounts/{id}/activate [post]
func (h *AdminMailAccountsActivateHandler) ServeHTTP(reqCtx request.RequestContext) {
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
	if err := store.Activate(value); err != nil {
		if errors.Is(err, mailaccounts.ErrNotFound) {
			response.ErrorResponse(w, http.StatusNotFound, "account not found")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, map[string]string{"id": value, "status": "active"})
}
