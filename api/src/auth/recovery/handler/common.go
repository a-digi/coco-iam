// Package handler hosts the three public HTTP entry points into the
// recovery flow. Kept separate from the service layer so the core
// stays transport-agnostic.
package handler

import (
	"net/http"

	"github.com/a-digi/coco-iam/src/auth/recovery"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// genericFailureMsg — single error surface for every auth-related
// failure on verify/reset. Intentionally vague so attackers can't
// use the endpoint as an oracle.
const genericFailureMsg = "Something went wrong. The reset link may be invalid, expired, or already used."

type bagGetter interface {
	Get(key string) (interface{}, bool)
}

// resolveService fetches the Service from the DI bag. Writes a 500
// and returns nil when the service isn't wired — callers early-return.
func resolveService(reqCtx request.RequestContext) *recovery.Service {
	w := reqCtx.GetWriter()
	ctx := reqCtx.GetDI()
	bag, ok := ctx.(bagGetter)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "DI context has unexpected type")
		return nil
	}
	raw, ok := bag.Get(recovery.ContextBagKeyService)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "recovery service not available")
		return nil
	}
	svc, ok := raw.(*recovery.Service)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "recovery service has unexpected type")
		return nil
	}
	return svc
}
