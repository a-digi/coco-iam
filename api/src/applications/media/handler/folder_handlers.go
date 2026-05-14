package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/a-digi/coco-server/server/media"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// CreateFolderHandler serves POST /.../media/folders.
// Body: {parent_id?, slug}.
type CreateFolderHandler struct{}

type createFolderRequest struct {
	ParentID *string `json:"parent_id"`
	Slug     string  `json:"slug"`
}

func (h *CreateFolderHandler) ServeHTTP(reqCtx request.RequestContext) {
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
	var req createFolderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	defer r.Body.Close()

	// Validate parent belongs to this application if present — keeps
	// one app from mounting a folder under another app's tree.
	if req.ParentID != nil && *req.ParentID != "" {
		parent, err := svc.Store.GetFolder(*req.ParentID)
		if err != nil || parent.OwnerID != appID {
			response.ErrorResponse(w, http.StatusBadRequest, "parent folder not found")
			return
		}
	}

	folder, err := svc.Store.InsertFolder(appID, req.ParentID, req.Slug)
	if err != nil {
		switch {
		case errors.Is(err, media.ErrSlugTaken):
			response.ErrorResponse(w, http.StatusConflict, err.Error())
		case errors.Is(err, media.ErrTooDeep):
			response.ErrorResponse(w, http.StatusConflict, err.Error())
		default:
			response.ErrorResponse(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	response.SuccessResponse(w, http.StatusCreated, folder)
}

// DeleteFolderHandler serves DELETE /.../media/folders/{id:<folderId>}?recursive=<bool>.
type DeleteFolderHandler struct{}

func (h *DeleteFolderHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	svc := resolveService(reqCtx)
	if svc == nil {
		return
	}
	// Two `{id:...}` segments in the URL (the app id and the folder id).
	// The path-var helper only reads the first one; grab the second
	// straight from the URL path.
	folderID := segmentAfter(reqCtx.GetRequest().URL.Path, "folders")
	// The segment is `{id:<value>}` — strip the wrapper.
	folderID = unwrapIDSegment(folderID)
	if folderID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "missing folder id")
		return
	}
	// Resolve the app id from the URL's outer `{id:...}` segment
	// (the outer `res:applications` + app-level `{id:}` segment).
	appID := appIDFromOuterURL(reqCtx.GetRequest().URL.Path)
	if appID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "missing application id")
		return
	}
	recursive := queryParam(reqCtx, "recursive") == "true"
	if err := svc.DeleteFolder(appID, folderID, recursive); err != nil {
		switch {
		case errors.Is(err, media.ErrNotFound):
			response.ErrorResponse(w, http.StatusNotFound, err.Error())
		case errors.Is(err, media.ErrNotEmpty):
			response.ErrorResponse(w, http.StatusConflict, err.Error())
		default:
			response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	response.SuccessResponse(w, http.StatusOK, map[string]interface{}{"ok": true})
}
