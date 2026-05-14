// Package handler serves the per-application login-page HTTP surface.
package handler

import (
	"net/http"
	"strings"

	uri "github.com/a-digi/coco-lift/resource/uri"
	"github.com/a-digi/coco-iam/src/applications/loginpage"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

type bagGetter interface {
	Get(key string) (interface{}, bool)
}

func resolveService(reqCtx request.RequestContext) *loginpage.Service {
	w := reqCtx.GetWriter()
	ctx := reqCtx.GetDI()
	bag, ok := ctx.(bagGetter)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "DI context has unexpected type")
		return nil
	}
	raw, ok := bag.Get(loginpage.ContextBagKeyService)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "loginpage service not available")
		return nil
	}
	svc, ok := raw.(*loginpage.Service)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "loginpage service has unexpected type")
		return nil
	}
	return svc
}

// appIDFromPath reads the `{id:<value>}` segment. Returns "" if absent.
func appIDFromPath(reqCtx request.RequestContext) string {
	key, value := uri.ExtractKeyAndValueFromURI(reqCtx.GetRequest().URL.Path)
	if key != "id" {
		return ""
	}
	return strings.TrimSpace(value)
}

// wsIDFromPath reads the `{id:<value>}` segment for workspace endpoints.
func wsIDFromPath(reqCtx request.RequestContext) string {
	return appIDFromPath(reqCtx)
}

// segmentAfter returns the first segment that follows `marker` in the path.
func segmentAfter(path, marker string) string {
	segs := strings.Split(strings.Trim(path, "/"), "/")
	for i, s := range segs {
		if s == marker && i+1 < len(segs) {
			return strings.TrimSpace(segs[i+1])
		}
	}
	return ""
}

// queryParam returns the trimmed value of a query parameter, or "".
func queryParam(reqCtx request.RequestContext, key string) string {
	return strings.TrimSpace(reqCtx.GetRequest().URL.Query().Get(key))
}
