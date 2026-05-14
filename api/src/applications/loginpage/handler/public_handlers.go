package handler

import (
	"errors"
	"net/http"

	"github.com/a-digi/coco-iam/src/applications/loginpage"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// PublicGetConfigHandler serves
// GET /api/v1/public/applications/login-template?org={org}&ws={ws}&app={app}.
// All three params are the admin-chosen slugs (organization.organization_id,
// workspace.workspace_id, applications.client_id) — never UUIDs.
// Returns the render-only PublicLoginConfig — no secrets, no redirect URL.
type PublicGetConfigHandler struct{}

func (h *PublicGetConfigHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	svc := resolveService(reqCtx)
	if svc == nil {
		return
	}
	orgSlug := queryParam(reqCtx, "org")
	wsSlug := queryParam(reqCtx, "ws")
	clientID := queryParam(reqCtx, "app")
	if orgSlug == "" || wsSlug == "" || clientID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "missing org, ws, or app query parameter")
		return
	}
	cfg, err := svc.GetPublicConfig(orgSlug, wsSlug, clientID, publicAssetURL, publicMediaURL)
	if err != nil {
		if errors.Is(err, loginpage.ErrNotFound) {
			response.ErrorResponse(w, http.StatusNotFound, "not found")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, cfg)
}

// publicAssetURL is the URL-builder given to the service. The asset
// file-server lives OUTSIDE /api/v1/ at /p/applications/assets/{id}
// so images are hot-linkable without hitting the versioned-API auth /
// CORS machinery. The short /p/ prefix distinguishes "plain" public
// media from the JSON API surface.
func publicAssetURL(assetID string) string {
	if assetID == "" {
		return ""
	}
	return "/p/applications/assets/" + assetID
}

// publicMediaURL builds the slug-based public URL for a root-folder
// media file. Lives OUTSIDE /api/v1/ at
// /p/media/<org>/<ws>/<app>/<filename>, with the slug trio so the
// admin-chosen identifiers flow all the way to the browser — no UUIDs
// in the public surface.
func publicMediaURL(orgSlug, wsSlug, clientID, filename string) string {
	if orgSlug == "" || wsSlug == "" || clientID == "" || filename == "" {
		return ""
	}
	return "/p/media/" + orgSlug + "/" + wsSlug + "/" + clientID + "/" + filename
}

// PublicServeAssetHandler serves GET /api/v1/public/applications/assets/{assetId}.
type PublicServeAssetHandler struct{}

func (h *PublicServeAssetHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	svc := resolveService(reqCtx)
	if svc == nil {
		return
	}
	id := segmentAfter(r.URL.Path, "assets")
	if id == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "missing asset id")
		return
	}
	data, mime, err := svc.ReadAsset(id)
	if err != nil {
		response.ErrorResponse(w, http.StatusNotFound, "not found")
		return
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
