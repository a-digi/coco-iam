package userprofile

import (
	"crypto/rsa"
	"errors"
	"io"
	"net/http"
	"time"

	profile_entity "github.com/a-digi/coco-iam/src/organizations/profile/entity"
	"github.com/a-digi/coco-server/server/media"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// FileUploadHandler serves
//
//	POST /a/{orgSlug}/{wsSlug}/{appSlug}/profile/me/files/{fieldName}
//
// Multipart upload for one file-type profile field. Same auth model
// as the GET / PATCH handlers; the authenticated user's id scopes
// every downstream call so one user can never attach files to
// another user's profile.
//
// Orchestration only — content validation lives in `Scanner`
// (delegates to the media subsystem's DetectAndValidateMime);
// disk writes + metadata rows live in FileStore / FileRepo; the
// final profile_data mutation goes through ProfileWriter. Every
// branching decision (per-field mime allowlist, per-field size cap)
// is handled by the pure `effectiveLimits` helper.
type FileUploadHandler struct {
	Slugs    SlugResolver
	Keys     KeyLoader
	Users    UserOrgReader
	Fields   FieldConfigReader
	Scanner  Scanner
	Store    FileStore
	Files    FileRepo
	Writer   ProfileWriter
	Profiles ProfileReader
	Now      func() time.Time
}

type fileUploadResponse struct {
	AssetID   string `json:"asset_id"`
	Filename  string `json:"filename"`
	MimeType  string `json:"mime_type"`
	SizeBytes int64  `json:"size_bytes"`
}

// @Summary     Upload profile file
// @Tags        app-profile-me
// @Produce     json
// @Param       fieldName path string true "Field Name"
// @Router      /a/{orgSlug}/{wsSlug}/{appSlug}/profile/me/files/{fieldName} [post]
func (h *FileUploadHandler) ServeHTTP(reqCtx request.RequestContext) {
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

	userID, authErr := h.authenticate(r, appID, orgID)
	if authErr != nil {
		if authErr.Status == http.StatusInternalServerError {
			response.ErrorResponse(w, http.StatusInternalServerError, genericUnauthorized)
			return
		}
		response.ErrorResponse(w, http.StatusUnauthorized, genericUnauthorized)
		return
	}

	field, err := h.Fields.FieldByName(orgID, fieldName)
	if err != nil {
		if errors.Is(err, ErrFieldNotFound) {
			response.ErrorResponse(w, http.StatusBadRequest, "unknown field")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to resolve field")
		return
	}
	if !field.IsActive || field.DataType != profile_entity.DataTypeFile {
		response.ErrorResponse(w, http.StatusBadRequest, "field does not accept file uploads")
		return
	}

	// Cap the request body before parsing so an attacker can't
	// exhaust memory via a crafted multipart payload.
	r.Body = http.MaxBytesReader(w, r.Body, media.MaxUploadBytes+4096)
	if err := r.ParseMultipartForm(media.MaxUploadBytes + 4096); err != nil {
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

	head := data
	if len(head) > 512 {
		head = head[:512]
	}
	claimed := ""
	rawFilename := ""
	if header != nil {
		claimed = header.Header.Get("Content-Type")
		rawFilename = header.Filename
	}
	detectedMime, ext, err := h.Scanner.DetectAndValidate(head, claimed)
	if err != nil {
		response.ErrorResponse(w, http.StatusUnsupportedMediaType, err.Error())
		return
	}

	allow, capBytes := effectiveLimits(field, detectedMime)
	if !mimeAllowed(detectedMime, allow) {
		response.ErrorResponse(w, http.StatusUnsupportedMediaType, "mime type not allowed for this field")
		return
	}
	if capBytes > 0 && int64(len(data)) > capBytes {
		response.ErrorResponse(w, http.StatusRequestEntityTooLarge, "file exceeds the field's size limit")
		return
	}

	// Look up the prior asset for this (user, field) BEFORE we mint
	// the new one — afterwards FindByField would return the new
	// row and we'd lose the handle for cleanup.
	var prior *FileMeta
	if p, err := h.Files.FindByField(orgID, userID, fieldName); err == nil {
		prior = p
	} else if !errors.Is(err, ErrAssetNotFound) {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to resolve current file")
		return
	}

	// Mint the asset id up front so FileStore.Save and the repo
	// insert agree on the same key.
	assetID, err := newAssetID()
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to generate asset id")
		return
	}
	if err := h.Store.Save(orgID, userID, assetID, ext, data); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to store file")
		return
	}

	filename := media.SlugifyFilename(rawFilename)
	meta := FileMeta{
		AssetID:   assetID,
		UserID:    userID,
		FieldName: fieldName,
		Filename:  filename,
		MimeType:  detectedMime,
		SizeBytes: int64(len(data)),
		Ext:       ext,
	}
	if _, err := h.Files.Insert(orgID, meta); err != nil {
		_ = h.Store.Delete(orgID, userID, assetID, ext)
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to persist file metadata")
		return
	}

	if err := h.Writer.UpdateFieldValue(orgID, userID, fieldName, assetID); err != nil {
		_ = h.Store.Delete(orgID, userID, assetID, ext)
		_ = h.Files.DeleteByAssetID(orgID, userID, assetID)
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to update profile")
		return
	}

	if prior != nil {
		_ = h.Store.Delete(orgID, userID, prior.AssetID, prior.Ext)
		_ = h.Files.DeleteByAssetID(orgID, userID, prior.AssetID)
	}

	response.SuccessResponse(w, http.StatusOK, fileUploadResponse{
		AssetID:   assetID,
		Filename:  filename,
		MimeType:  detectedMime,
		SizeBytes: int64(len(data)),
	})
}

func (h *FileUploadHandler) authenticate(r *http.Request, appID, orgID string) (string, *AuthError) {
	loadKey := LoadPublicKeyFunc(func(kid string) (*rsa.PublicKey, error) {
		return h.Keys.LoadPublicKey(appID, kid)
	})
	userOrg := UserOrgLookupFunc(h.Users.UserOrg)
	nowFn := time.Now
	if h.Now != nil {
		nowFn = h.Now
	}
	return authenticateUser(r.Header.Get("Authorization"), orgID, loadKey, userOrg, nowFn())
}

// extractFieldName pulls `<name>` out of `/a/…/profile/me/files/<name>`.
func extractFieldName(path string) string {
	parts := splitSegments(path)
	for i, s := range parts {
		if s == "files" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

