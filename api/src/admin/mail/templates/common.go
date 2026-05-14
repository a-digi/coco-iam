// Package templates hosts the admin handlers for the mail_templates CRUD
// surface. Each handler resolves the Repository from the ContextBag and
// translates JSON request / response shapes.
package templates

import (
	"net/http"

	"github.com/a-digi/coco-iam/config/di"
	iam_mail "github.com/a-digi/coco-iam/src/mail"
	"github.com/a-digi/coco-iam/src/mail/template"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// resolveRepo fetches the Repository from DI. On any error it writes the
// response and returns nil — callers should then early-return.
func resolveRepo(reqCtx request.RequestContext) *template.Repository {
	w := reqCtx.GetWriter()
	ctx := reqCtx.GetDI()
	bag, ok := ctx.(*di.ContextBag)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "DI context has unexpected type")
		return nil
	}
	raw, ok := bag.Get(iam_mail.ContextBagKeyTemplateRepository)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "mail template repository not available")
		return nil
	}
	repo, ok := raw.(*template.Repository)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "mail template repository has unexpected type")
		return nil
	}
	return repo
}
