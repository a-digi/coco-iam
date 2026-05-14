package userprofile

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// FileServeHandler serves
//
//	GET /a/{orgSlug}/{wsSlug}/{appSlug}/profile/me/files/{fieldName}
//
// Streams the current asset bytes for the authenticated user's
// file-type field. Assets belong to exactly one (orgID, userID)
// pair — requests that resolve to a different user get 404 (not
// 403) so existence of another user's asset never leaks through.
type FileServeHandler struct {
	Slugs  SlugResolver
	Keys   KeyLoader
	Users  UserOrgReader
	Store  FileStore
	Files  FileRepo
	Now    func() time.Time
}

func (h *FileServeHandler) ServeHTTP(reqCtx request.RequestContext) {
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

	meta, err := h.Files.FindByField(orgID, userID, fieldName)
	if err != nil {
		if errors.Is(err, ErrAssetNotFound) {
			response.NotFoundResponse(w, "file not found")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to resolve file")
		return
	}
	if meta == nil {
		response.NotFoundResponse(w, "file not found")
		return
	}

	data, err := h.Store.Open(orgID, userID, meta.AssetID, meta.Ext)
	if err != nil {
		if errors.Is(err, ErrAssetNotFound) {
			response.NotFoundResponse(w, "file not found")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to read file")
		return
	}

	w.Header().Set("Content-Type", meta.MimeType)
	w.Header().Set("Content-Disposition",
		fmt.Sprintf(`inline; filename="%s"`, meta.Filename))
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
