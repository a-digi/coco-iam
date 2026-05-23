package accounts

import (
	"net/http"

	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// AdminMailAccountsListHandler serves GET /api/v1/admin/mail/accounts.
type AdminMailAccountsListHandler struct{}

// @Summary     List mail accounts
// @Tags        admin-mail
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} interface{}
// @Failure     400,401,403,500 {object} map[string]interface{}
// @Router      /admin/mail/accounts [get]
func (h *AdminMailAccountsListHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	store := resolveStore(reqCtx)
	if store == nil {
		return
	}
	rows, err := store.List()
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	for i := range rows {
		rows[i] = rows[i].Redacted()
	}
	response.SuccessResponse(w, http.StatusOK, rows)
}
