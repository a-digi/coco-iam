package templates

import (
	"errors"
	"net/http"

	"github.com/a-digi/coco-lift/resource/uri"
	"github.com/a-digi/coco-iam/src/mail/template"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// AdminMailTemplatesGetHandler serves GET /api/v1/admin/mail/templates/{id}.
type AdminMailTemplatesGetHandler struct{}

func (h *AdminMailTemplatesGetHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	key, value := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if key != "id" || value == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "template id is required")
		return
	}

	repo := resolveRepo(reqCtx)
	if repo == nil {
		return
	}

	row, err := repo.Get(value)
	if err != nil {
		if errors.Is(err, template.ErrNotFound) {
			response.ErrorResponse(w, http.StatusNotFound, "template not found")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, row)
}
