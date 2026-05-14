package admin

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/a-digi/coco-iam/config/di"
	iam_mail "github.com/a-digi/coco-iam/src/mail"
	"github.com/a-digi/coco-iam/src/mail/store"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// AdminMailListHandler serves GET /api/v1/admin/mail/outbound.
// Query parameters:
//
//	status=queued,sent,dead_lettered  (comma-separated)
//	to=<substring>                    (matches to_json LIKE)
//	limit=<n>                         (max 500, default 50)
//	offset=<n>
type AdminMailListHandler struct{}

type listResponse struct {
	Items  []store.Row `json:"items"`
	Total  int         `json:"total"`
	Limit  int         `json:"limit"`
	Offset int         `json:"offset"`
}

func (h *AdminMailListHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	ctx := reqCtx.GetDI()

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

	q := r.URL.Query()
	filter := store.ListFilter{ToLike: strings.TrimSpace(q.Get("to"))}
	if s := q.Get("status"); s != "" {
		for _, part := range strings.Split(s, ",") {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				filter.Statuses = append(filter.Statuses, trimmed)
			}
		}
	}
	filter.Limit = parseIntOr(q.Get("limit"), 50)
	filter.Offset = parseIntOr(q.Get("offset"), 0)

	total, err := st.Count(filter)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	rows, err := st.List(filter)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.SuccessResponse(w, http.StatusOK, listResponse{
		Items:  rows,
		Total:  total,
		Limit:  filter.Limit,
		Offset: filter.Offset,
	})
}

func parseIntOr(s string, def int) int {
	if s == "" {
		return def
	}
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return def
}
