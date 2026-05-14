package handler

import (
	"errors"
	"io"
	"net/http"

	"github.com/a-digi/coco-server/server/media"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// UploadFileHandler serves POST /.../media/files (multipart).
// Form fields:
//   - folder_id (optional) — destination folder; missing = app root
//   - file                — the upload (required)
type UploadFileHandler struct{}

func (h *UploadFileHandler) ServeHTTP(reqCtx request.RequestContext) {
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
	r.Body = http.MaxBytesReader(w, r.Body, media.MaxUploadBytes+4096)
	if err := r.ParseMultipartForm(media.MaxUploadBytes + 4096); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid multipart payload")
		return
	}
	var folderID *string
	if fid := r.FormValue("folder_id"); fid != "" {
		// Validate the folder belongs to this app to prevent
		// cross-app writes via forged folder_id.
		if f, err := svc.Store.GetFolder(fid); err != nil || f.OwnerID != appID {
			response.ErrorResponse(w, http.StatusBadRequest, "folder not found")
			return
		}
		folderID = &fid
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
	filename := ""
	claimed := ""
	if header != nil {
		filename = header.Filename
		claimed = header.Header.Get("Content-Type")
	}
	row, err := svc.StoreUpload(appID, folderID, filename, claimed, data)
	if err != nil {
		switch {
		case errors.Is(err, media.ErrMimeNotAllowed),
			errors.Is(err, media.ErrTooLarge),
			errors.Is(err, media.ErrNameTaken):
			response.ErrorResponse(w, http.StatusBadRequest, err.Error())
		default:
			response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	response.SuccessResponse(w, http.StatusCreated, row)
}

// DeleteFileHandler serves DELETE /.../media/files/{id:<fileId>}.
type DeleteFileHandler struct{}

func (h *DeleteFileHandler) ServeHTTP(reqCtx request.RequestContext) {
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
	fileID := unwrapIDSegment(segmentAfter(reqCtx.GetRequest().URL.Path, "files"))
	if fileID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "missing file id")
		return
	}
	if err := svc.DeleteFile(appID, fileID); err != nil {
		if errors.Is(err, media.ErrNotFound) {
			response.ErrorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, map[string]interface{}{"ok": true})
}
