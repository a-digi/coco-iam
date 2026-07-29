package handler

import (
	"net/http"

	"github.com/a-digi/coco-iam/config/di"
	"github.com/a-digi/coco-iam/src/security/ipsearch"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// resolveSearcher constructs an ipsearch.Searcher from the shared
// geoip.Lookup and ip-attacks.db handle already on ContextBag — cheap
// and stateless (mirrors geoip/handler/common.go's resolveSettingsQuery),
// so there's no need to pre-construct and store a Searcher on
// ContextBag the way geoip.Manager has to be.
func resolveSearcher(reqCtx request.RequestContext) (*ipsearch.Searcher, bool) {
	w := reqCtx.GetWriter()
	bag, ok := reqCtx.GetDI().(*di.ContextBag)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "DI context has unexpected type")
		return nil, false
	}
	handle := bag.GetIPAttacksHandle()
	if handle == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "ip-attacks db handle not available")
		return nil, false
	}
	return ipsearch.NewSearcher(bag.GetGeoIP(), handle), true
}
