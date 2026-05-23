// Package avatar — handlers for upload / delete / public serve of
// the admin user avatar. Scopes on the admin endpoints match the
// existing /me routes (admin:me); the public serve path sits
// outside /api/v1 under /p/admin-avatars/<admin_user_id> and takes
// no auth.
package avatar

import (
	"errors"
	"io"
	"net/http"
	"path"
	"strings"

	profile_repo "github.com/a-digi/coco-iam/src/admin/users/profile/repository"
	jwt_token "github.com/a-digi/coco-iam/src/auth/security/jwt"
	"github.com/a-digi/coco-oauth/oauth"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// ContextBagKeyStore is the DI key under which the shared FileStore
// is registered by main.go.
const ContextBagKeyStore = "admin.users.profile.avatar.store"

// MaxUploadBytes caps the upload size at 2 MB so an admin can't fill
// the disk or stall the server with a slow PUT. Multipart parsing
// also caps memory separately below.
const MaxUploadBytes int64 = 2 * 1024 * 1024

// UploadHandler serves POST /api/v1/admin/users/me/avatar.
// Multipart/form-data, field name `file`. Replaces any existing
// avatar atomically.
type UploadHandler struct{}

type uploadResponse struct {
	AvatarAssetID string `json:"avatar_asset_id"`
	AvatarURL     string `json:"avatar_url"`
}

// @Summary     Upload admin user avatar
// @Tags        admin-me
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} interface{}
// @Failure     400,401,403,500 {object} map[string]interface{}
// @Router      /admin/users/me/avatar [post]
func (h *UploadHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	ctx := reqCtx.GetDI()

	userID, ok := subjectFromBearer(r.Header.Get("Authorization"))
	if !ok {
		response.ErrorResponse(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	store := resolveStore(ctx)
	if store == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "avatar store not available")
		return
	}
	manager := ctx.GetDatabaseManager()
	if manager == nil || manager.Connector == nil || manager.Connector.DB == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "database manager not available")
		return
	}

	// Hard cap on the whole body. Multipart forms can be much
	// larger than the file alone (boundary + form fields) so the
	// 2MB cap is applied at the request level, not at the file.
	r.Body = http.MaxBytesReader(w, r.Body, MaxUploadBytes)

	// 1 MB of metadata in RAM is plenty; the file bytes stream to
	// the filestore from the multipart reader.
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		response.ErrorResponse(w, http.StatusRequestEntityTooLarge, "upload exceeds "+humanSize(MaxUploadBytes))
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "missing 'file' field")
		return
	}
	defer file.Close()

	ext := extensionFromFilename(header.Filename)
	if ext == "" {
		response.ErrorResponse(w, http.StatusUnsupportedMediaType, "filename must have a recognisable extension")
		return
	}
	if _, allowed := AllowedExtensions[ext]; !allowed {
		response.ErrorResponse(w, http.StatusUnsupportedMediaType, "unsupported image type")
		return
	}

	// Read the old asset id so we can delete the file AFTER the
	// new one is in place — the serve endpoint will stop seeing
	// the stale bytes as soon as the DB column changes.
	repo := profile_repo.New(manager.Connector.DB)
	prev, _ := repo.FindByAdminUserID(userID)
	prevAssetID := ""
	if prev != nil {
		prevAssetID = prev.AvatarAssetID
	}

	assetID, err := store.Save(userID, file, ext)
	if err != nil {
		if errors.Is(err, ErrInvalidExtension) {
			response.ErrorResponse(w, http.StatusUnsupportedMediaType, "unsupported image type")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to save avatar")
		return
	}
	if err := repo.UpdateAvatarAssetID(userID, assetID); err != nil {
		// The file is on disk but the DB didn't update — leave
		// the file in place and surface the error; a retry will
		// overwrite it cleanly.
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to persist avatar id")
		return
	}
	// Best-effort cleanup of the prior file, but only if the
	// extension changed (same extension means Save already
	// overwrote it in place).
	if prevAssetID != "" && prevAssetID != assetID {
		_ = store.Delete(prevAssetID)
	}

	response.SuccessResponse(w, http.StatusOK, uploadResponse{
		AvatarAssetID: assetID,
		AvatarURL:     "/p/admin-avatars/" + userID,
	})
}

// DeleteHandler serves DELETE /api/v1/admin/users/me/avatar.
// Idempotent — "no avatar to delete" is a 200 success, not a 404.
type DeleteHandler struct{}

// @Summary     Delete admin user avatar
// @Tags        admin-me
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} interface{}
// @Failure     400,401,403,500 {object} map[string]interface{}
// @Router      /admin/users/me/avatar [delete]
func (h *DeleteHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	ctx := reqCtx.GetDI()

	userID, ok := subjectFromBearer(r.Header.Get("Authorization"))
	if !ok {
		response.ErrorResponse(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	store := resolveStore(ctx)
	if store == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "avatar store not available")
		return
	}
	manager := ctx.GetDatabaseManager()
	if manager == nil || manager.Connector == nil || manager.Connector.DB == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "database manager not available")
		return
	}
	repo := profile_repo.New(manager.Connector.DB)
	prev, err := repo.FindByAdminUserID(userID)
	if err != nil && !errors.Is(err, profile_repo.ErrNotFound) {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Clear the DB first so the public serve path stops returning
	// bytes even if the file-remove fails.
	if err := repo.ClearAvatar(userID); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if prev != nil && prev.AvatarAssetID != "" {
		_ = store.Delete(prev.AvatarAssetID)
	}
	response.SuccessResponse(w, http.StatusOK, map[string]string{"status": "ok"})
}

// PublicServeHandler serves GET /p/admin-avatars/<admin_user_id>.
// Looks up the current avatar_asset_id, streams the bytes with the
// right Content-Type. No auth — avatar images are non-sensitive
// (same category as any signed-in landing page).
type PublicServeHandler struct{}

// @Summary     Serve admin user avatar (public)
// @Description Public endpoint outside /api/v1 base path. No authentication required.
// @Tags        admin-avatars
// @Produce     json
// @Success     200 {object} interface{}
// @Failure     400,401,403,500 {object} map[string]interface{}
// @Router      /p/admin-avatars/{adminUserId} [get]
func (h *PublicServeHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	ctx := reqCtx.GetDI()

	userID := lastPathSegment(r.URL.Path)
	if userID == "" {
		http.NotFound(w, r)
		return
	}
	store := resolveStore(ctx)
	if store == nil {
		http.Error(w, "avatar store not available", http.StatusInternalServerError)
		return
	}
	manager := ctx.GetDatabaseManager()
	if manager == nil || manager.Connector == nil || manager.Connector.DB == nil {
		http.Error(w, "database manager not available", http.StatusInternalServerError)
		return
	}
	repo := profile_repo.New(manager.Connector.DB)
	profile, err := repo.FindByAdminUserID(userID)
	if err != nil || profile == nil || profile.AvatarAssetID == "" {
		http.NotFound(w, r)
		return
	}
	rc, err := store.Open(profile.AvatarAssetID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer rc.Close()

	if ct := contentTypeForExt(ExtensionOf(profile.AvatarAssetID)); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	// Cheap client-side caching — 5 minutes lines up with the
	// typical session rhythm and forces a refresh after the
	// admin uploads a new avatar (file change = different bytes,
	// same URL; the browser may serve stale until the TTL). If
	// snappier refresh becomes important later, switch to an
	// ETag.
	w.Header().Set("Cache-Control", "public, max-age=300")
	_, _ = io.Copy(w, rc)
}

// -- helpers ----------------------------------------------------------

type bagGetter interface {
	Get(key string) (interface{}, bool)
}

func resolveStore(ctx interface{}) *FileStore {
	bag, ok := ctx.(bagGetter)
	if !ok {
		return nil
	}
	raw, ok := bag.Get(ContextBagKeyStore)
	if !ok {
		return nil
	}
	s, _ := raw.(*FileStore)
	return s
}

func subjectFromBearer(header string) (string, bool) {
	if header == "" {
		return "", false
	}
	token, err := oauth.ExtractBearer(header)
	if err != nil {
		return "", false
	}
	sub, err := jwt_token.ParseJWTSubject(token)
	if err != nil || sub == "" {
		return "", false
	}
	return sub, true
}

// extensionFromFilename lowercases and strips the leading dot. Uses
// path.Ext so paths with multiple dots pick the last segment.
func extensionFromFilename(name string) string {
	ext := strings.ToLower(strings.TrimPrefix(path.Ext(name), "."))
	return ext
}

// lastPathSegment returns the last non-empty segment of a URL path.
// Used to pull the admin_user_id out of
// /p/admin-avatars/<admin_user_id>.
func lastPathSegment(p string) string {
	trimmed := strings.Trim(p, "/")
	if trimmed == "" {
		return ""
	}
	idx := strings.LastIndex(trimmed, "/")
	if idx < 0 {
		return trimmed
	}
	return trimmed[idx+1:]
}

// contentTypeForExt returns the MIME for the whitelisted extensions
// only — anything else becomes empty and we let the browser sniff.
func contentTypeForExt(ext string) string {
	switch ext {
	case "png":
		return "image/png"
	case "jpg", "jpeg":
		return "image/jpeg"
	case "webp":
		return "image/webp"
	case "gif":
		return "image/gif"
	}
	return ""
}

func humanSize(n int64) string {
	const mb = 1024 * 1024
	if n >= mb {
		return "2 MB"
	}
	return "limit"
}
