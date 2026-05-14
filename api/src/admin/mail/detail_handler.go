package admin

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/a-digi/coco-iam/config/di"
	"github.com/a-digi/coco-lift/resource/uri"
	iam_mail "github.com/a-digi/coco-iam/src/mail"
	"github.com/a-digi/coco-iam/src/mail/store"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// AdminMailDetailHandler serves GET /api/v1/admin/mail/outbound/{id}.
// Returns the mail_outbound row. Body bytes live on the queue's payload
// file and can be fetched via the existing /admin/queue/tasks/{id}/payload
// endpoint — the IDs are aligned.
type AdminMailDetailHandler struct{}

func (h *AdminMailDetailHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	ctx := reqCtx.GetDI()

	key, value := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if key != "id" || value == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "mail id is required")
		return
	}

	bag, ok := ctx.(*di.ContextBag)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "DI context has unexpected type")
		return
	}
	raw, ok := bag.Get(iam_mail.ContextBagKeyMailStore)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "mail store not available")
		return
	}
	st, ok := raw.(*store.Store)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "mail store has unexpected type")
		return
	}

	row, err := st.Get(value)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.ErrorResponse(w, http.StatusNotFound, "mail row not found")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, row)
}
