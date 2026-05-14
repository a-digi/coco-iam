package userprofile

import (
	"crypto/rsa"
	"errors"
	"net/http"
	"time"

	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// FileDeleteHandler serves
//
//	DELETE /a/{orgSlug}/{wsSlug}/{appSlug}/profile/me/files/{fieldName}
//
// Removes the current asset for the authenticated user's file-type
// field. Idempotent — deleting when there is nothing to delete
// still returns 200. On a successful delete the field is cleared
// from profile_data.
type FileDeleteHandler struct {
	Slugs  SlugResolver
	Keys   KeyLoader
	Users  UserOrgReader
	Fields FieldConfigReader
	Store  FileStore
	Files  FileRepo
	Writer ProfileWriter
	Now    func() time.Time
}

func (h *FileDeleteHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	orgSlug, wsSlug, appSlug, ok := parseSlugSegments(r.URL.Path)
	if !ok {
		response.ErrorResponse(w, http.StatusUnauthorized, genericUnauthorized)
		return
	}
	fieldName := extractFieldName(r.URL.Path)
	if fieldName == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "missing field name")
		return
	}

	appID, orgID, err := h.Slugs.ResolveSlugs(orgSlug, wsSlug, appSlug)
	if err != nil {
		response.ErrorResponse(w, http.StatusUnauthorized, genericUnauthorized)
		return
	}

	loadKey := LoadPublicKeyFunc(func(kid string) (*rsa.PublicKey, error) {
		return h.Keys.LoadPublicKey(appID, kid)
	})
	userOrg := UserOrgLookupFunc(h.Users.UserOrg)
	nowFn := time.Now
	if h.Now != nil {
		nowFn = h.Now
	}
	userID, authErr := authenticateUser(r.Header.Get("Authorization"), orgID, loadKey, userOrg, nowFn())
	if authErr != nil {
		if authErr.Status == http.StatusInternalServerError {
			response.ErrorResponse(w, http.StatusInternalServerError, genericUnauthorized)
			return
		}
		response.ErrorResponse(w, http.StatusUnauthorized, genericUnauthorized)
		return
	}

	prior, err := h.Files.FindByField(orgID, userID, fieldName)
	if err != nil && !errors.Is(err, ErrAssetNotFound) {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to resolve current file")
		return
	}
	if prior == nil {
		// Idempotent — nothing to delete. Still clear the profile
		// field in case there's a stale asset id hanging around.
		_ = h.Writer.UpdateFieldValue(orgID, userID, fieldName, nil)
		response.SuccessResponse(w, http.StatusOK, map[string]any{"ok": true})
		return
	}

	if err := h.Store.Delete(orgID, userID, prior.AssetID, prior.Ext); err != nil {
		// Disk remove failed: leave the repo row intact so a retry
		// can still find the asset and unlink again. Returning 500
		// signals the caller to retry.
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to delete file")
		return
	}
	if err := h.Files.DeleteByAssetID(orgID, userID, prior.AssetID); err != nil && !errors.Is(err, ErrAssetNotFound) {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to remove file metadata")
		return
	}
	if err := h.Writer.UpdateFieldValue(orgID, userID, fieldName, nil); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to update profile")
		return
	}
	response.SuccessResponse(w, http.StatusOK, map[string]any{"ok": true})
}
