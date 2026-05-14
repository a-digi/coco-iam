// Package slugmedia wraps the existing media file-server so the
// public /p/media/** surface accepts the three-level slug form
// /p/media/<org_id>/<workspace_id>/<client_id>/<filename> in addition
// to the legacy UUID form /p/media/<owner_uuid>/<path> the admin
// MediaBrowser still uses. Requests with a leading UUID segment are
// passed through unchanged; slug-trio requests are resolved via
// loginpage.Store.FindBySlugs and rewritten to the UUID form before
// delegation.
package slugmedia

import (
	"net/http"
	"regexp"
	"strings"

	app_loginpage "github.com/a-digi/coco-iam/src/applications/loginpage"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/routing"
)

var uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// Handler dispatches /p/media/** to a wrapped file-server, resolving
// the slug trio to a UUID when the first segment isn't already a UUID.
//
// Implementation note: the file-server delegate reads
// `reqCtx.GetURI().GetPathRemainder()` (populated by the router for
// the `/p/media/**` pattern), NOT `r.URL.Path`. So we rewrite the
// remainder in place; a rewrite of `r.URL.Path` alone would be
// invisible to the delegate and trigger a 404.
type Handler struct {
	Store    *app_loginpage.Store
	Delegate routing.HandlerInterface
}

func (h *Handler) ServeHTTP(reqCtx request.RequestContext) {
	if h.Delegate == nil {
		reqCtx.GetWriter().WriteHeader(http.StatusInternalServerError)
		return
	}

	uri := reqCtx.GetURI()
	remainder := uri.GetPathRemainder()
	// Fallback for routers that didn't set the remainder (shouldn't
	// happen for `/p/media/**` but keeps the handler defensive).
	if remainder == "" {
		remainder = strings.TrimPrefix(reqCtx.GetRequest().URL.Path, "/p/media/")
	}

	parts := strings.SplitN(remainder, "/", 5)

	// Legacy UUID form (admin MediaBrowser) — first segment is a UUID,
	// pass through unchanged. Also covers the degenerate cases where
	// the path is too short to carry a slug trio + filename.
	if len(parts) < 4 || uuidRegex.MatchString(parts[0]) {
		h.Delegate.ServeHTTP(reqCtx)
		return
	}
	if h.Store == nil {
		h.Delegate.ServeHTTP(reqCtx)
		return
	}

	info, err := h.Store.FindBySlugs(parts[0], parts[1], parts[2])
	if err != nil {
		http.NotFound(reqCtx.GetWriter(), reqCtx.GetRequest())
		return
	}

	rest := strings.Join(parts[3:], "/")
	// The delegate's Resolver expects `<ownerID>/<folder>/.../<filename>`
	// — overwrite the remainder with that shape and let the file-server
	// do the actual disk lookup. Keep r.URL.Path in sync so downstream
	// logging / debugging reads the same rewritten path.
	uri.SetPathRemainder(info.ID + "/" + rest)
	reqCtx.GetRequest().URL.Path = "/p/media/" + info.ID + "/" + rest

	h.Delegate.ServeHTTP(reqCtx)
}
