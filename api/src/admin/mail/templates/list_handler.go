package templates

import (
	"net/http"
	"strconv"
	"strings"

	template "github.com/a-digi/coco-notification/template"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// AdminMailTemplatesListHandler serves GET /api/v1/admin/mail/templates.
// Query params:
//
//	name=<substring>
//	description=<substring>
//	limit=<n>   (default 50, max 500)
//	offset=<n>
type AdminMailTemplatesListHandler struct{}

type listResponse struct {
	Items  []template.Template `json:"items"`
	Total  int                 `json:"total"`
	Limit  int                 `json:"limit"`
	Offset int                 `json:"offset"`
}

// @Summary     List mail templates
// @Tags        admin-mail-templates
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} interface{}
// @Failure     400,401,403,500 {object} map[string]interface{}
// @Router      /admin/mail/templates [get]
func (h *AdminMailTemplatesListHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	repo := resolveRepo(reqCtx)
	if repo == nil {
		return
	}

	q := r.URL.Query()
	filter := template.ListFilter{
		NameLike:        strings.TrimSpace(q.Get("name")),
		DescriptionLike: strings.TrimSpace(q.Get("description")),
	}
	filter.Limit = parseIntOr(q.Get("limit"), 50)
	filter.Offset = parseIntOr(q.Get("offset"), 0)

	items, total, err := repo.List(filter)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, listResponse{
		Items:  items,
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
