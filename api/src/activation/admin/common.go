package admin

import (
	"net/http"

	"github.com/a-digi/coco-iam/src/activation"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

type bagGetter interface {
	Get(key string) (interface{}, bool)
}

// resolveActivationService fetches the Service from the DI bag. Returns
// nil after writing an error response when the service isn't wired —
// callers should early-return.
func resolveActivationService(reqCtx request.RequestContext) *activation.Service {
	w := reqCtx.GetWriter()
	ctx := reqCtx.GetDI()
	bag, ok := ctx.(bagGetter)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "DI context has unexpected type")
		return nil
	}
	raw, ok := bag.Get(activation.ContextBagKeyService)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "activation service not available")
		return nil
	}
	svc, ok := raw.(*activation.Service)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "activation service has unexpected type")
		return nil
	}
	return svc
}
