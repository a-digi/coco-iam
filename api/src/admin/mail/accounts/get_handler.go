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

// @Summary     Get a mail account
// @Tags        admin-mail
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Account ID"
// @Success     200 {object} interface{}
// @Failure     400,401,403,500 {object} map[string]interface{}
// @Router      /admin/mail/accounts/{id} [get]
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
