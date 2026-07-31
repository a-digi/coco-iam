package templates

import (
	"errors"
	"net/http"

	"github.com/a-digi/coco-lift/resource/uri"
	template "github.com/a-digi/coco-notification/template"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// AdminMailTemplatesDeleteHandler serves DELETE /api/v1/admin/mail/templates/{id}.
type AdminMailTemplatesDeleteHandler struct{}

// @Summary     Delete a mail template
// @Tags        admin-mail-templates
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Template ID"
// @Success     200 {object} interface{}
// @Failure     400,401,403,500 {object} map[string]interface{}
// @Router      /admin/mail/templates/{id} [delete]
func (h *AdminMailTemplatesDeleteHandler) ServeHTTP(reqCtx request.RequestContext) {
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

	if err := repo.Delete(value); err != nil {
		if errors.Is(err, template.ErrNotFound) {
			response.ErrorResponse(w, http.StatusNotFound, "template not found")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, map[string]string{"id": value, "status": "deleted"})
}
