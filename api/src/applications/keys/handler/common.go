package handler

import (
	"net/http"
	"strings"

	"github.com/a-digi/coco-lift/resource/uri"
	"github.com/a-digi/coco-iam/src/applications/keys"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

type bagGetter interface {
	Get(key string) (interface{}, bool)
}

// resolveService resolves the shared keys service from the DI bag.
// On failure it writes the error directly to the response and
// returns nil so the caller bails early.
func resolveService(reqCtx request.RequestContext) *keys.Service {
	w := reqCtx.GetWriter()
	ctx := reqCtx.GetDI()
	bag, ok := ctx.(bagGetter)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "DI context has unexpected type")
		return nil
	}
	raw, ok := bag.Get(keys.ContextBagKeyService)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "keys service not available")
		return nil
	}
	svc, ok := raw.(*keys.Service)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "keys service has unexpected type")
		return nil
	}
	return svc
}

// appIDFromPath pulls the `{id:<value>}` segment out of the URL.
func appIDFromPath(reqCtx request.RequestContext) string {
	key, value := uri.ExtractKeyAndValueFromURI(reqCtx.GetRequest().URL.Path)
	if key != "id" {
		return ""
	}
	return strings.TrimSpace(value)
}

// segmentAfter returns the URL segment that immediately follows
// `marker`. Used by the public JWKS handler which is not pattern
// matched via `{id:…}`.
func segmentAfter(path, marker string) string {
	segs := strings.Split(strings.Trim(path, "/"), "/")
	for i, s := range segs {
		if s == marker && i+1 < len(segs) {
			return strings.TrimSpace(segs[i+1])
		}
	}
	return ""
}
