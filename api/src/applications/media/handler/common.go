// Package handler serves the media subsystem's HTTP surface.
package handler

import (
	"net/http"
	"strings"

	uri "github.com/a-digi/coco-lift/resource/uri"
	"github.com/a-digi/coco-server/server/media"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

type bagGetter interface {
	Get(key string) (interface{}, bool)
}

// resolveService pulls the media.Service out of the DI bag.
// Writes a 500 and returns nil on missing wiring.
func resolveService(reqCtx request.RequestContext) *media.Service {
	w := reqCtx.GetWriter()
	ctx := reqCtx.GetDI()
	bag, ok := ctx.(bagGetter)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "DI context has unexpected type")
		return nil
	}
	raw, ok := bag.Get(media.ContextBagKeyService)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "media service not available")
		return nil
	}
	svc, ok := raw.(*media.Service)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "media service has unexpected type")
		return nil
	}
	return svc
}

// appIDFromPath reads the FIRST `{id:<value>}` segment from the URL
// — our routes put the application id first, before any
// folders/files segment that may also carry `{id:...}`.
func appIDFromPath(reqCtx request.RequestContext) string {
	return appIDFromOuterURL(reqCtx.GetRequest().URL.Path)
}

// appIDFromOuterURL does the same job but takes a raw path string.
func appIDFromOuterURL(path string) string {
	key, value := uri.ExtractKeyAndValueFromURI(path)
	if key != "id" {
		return ""
	}
	return strings.TrimSpace(value)
}

// segmentAfter returns the first non-empty segment following `marker`
// in the URL path. Used to locate the `{id:<folderId>}` (or file id)
// that sits after the sub-resource name.
func segmentAfter(path, marker string) string {
	segs := strings.Split(strings.Trim(path, "/"), "/")
	for i, s := range segs {
		if s == marker && i+1 < len(segs) {
			return strings.TrimSpace(segs[i+1])
		}
	}
	return ""
}

// unwrapIDSegment strips the `{id:<value>}` wrapper the frontend
// inserts into URLs. Returns the raw value, or the input unchanged
// when it's not wrapped.
func unwrapIDSegment(seg string) string {
	if !strings.HasPrefix(seg, "{") || !strings.HasSuffix(seg, "}") {
		return seg
	}
	inner := seg[1 : len(seg)-1]
	parts := strings.SplitN(inner, ":", 2)
	if len(parts) != 2 || parts[0] != "id" {
		return seg
	}
	return strings.TrimSpace(parts[1])
}

// queryParam returns the trimmed value of a query string parameter.
func queryParam(reqCtx request.RequestContext, key string) string {
	return strings.TrimSpace(reqCtx.GetRequest().URL.Query().Get(key))
}
