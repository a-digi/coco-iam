package handler

import (
	"io"
	"net/http"

	"github.com/a-digi/coco-iam/src/applications/loginpage"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// UploadAssetHandler serves POST /api/v1/applications/{id}/login-template/assets.
// Multipart upload with a single `file` field.
type UploadAssetHandler struct{}

func (h *UploadAssetHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	svc := resolveService(reqCtx)
	if svc == nil {
		return
	}
	appID := appIDFromPath(reqCtx)
	if appID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "missing application id")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, loginpage.AssetCapBytes+4096)
	if err := r.ParseMultipartForm(loginpage.AssetCapBytes + 4096); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid multipart payload")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "file field is required")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "could not read file")
		return
	}
	claimed := ""
	if header != nil {
		claimed = header.Header.Get("Content-Type")
	}
	// `kind` tags the asset for the admin UI (background / logo / other).
	// Unknown or missing values fall back to AssetKindOther inside the service.
	kind := loginpage.AssetKind(r.FormValue("kind"))
	asset, err := svc.StoreAsset(appID, data, claimed, kind)
	if err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusCreated, asset)
}

// ListAssetsHandler serves GET /api/v1/applications/{id}/login-template/assets.
// Returns every asset registered for the application.
type ListAssetsHandler struct{}

func (h *ListAssetsHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	svc := resolveService(reqCtx)
	if svc == nil {
		return
	}
	appID := appIDFromPath(reqCtx)
	if appID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "missing application id")
		return
	}
	assets, err := svc.Store.ListAssets(appID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if assets == nil {
		assets = []loginpage.Asset{}
	}
	response.SuccessResponse(w, http.StatusOK, assets)
}

// DeleteAssetHandler serves DELETE /api/v1/applications/{id}/login-template/assets/{assetId}.
type DeleteAssetHandler struct{}

func (h *DeleteAssetHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	svc := resolveService(reqCtx)
	if svc == nil {
		return
	}
	appID := appIDFromPath(reqCtx)
	if appID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "missing application id")
		return
	}
	assetID := segmentAfter(r.URL.Path, "assets")
	if assetID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "missing asset id")
		return
	}
	// Ownership check — don't allow one app's admin to delete
	// another app's asset by guessing IDs.
	a, err := svc.Store.FindAsset(assetID)
	if err != nil || a.ApplicationID != appID {
		response.ErrorResponse(w, http.StatusNotFound, "asset not found")
		return
	}
	if err := svc.DeleteAsset(assetID); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, map[string]interface{}{"ok": true})
}
