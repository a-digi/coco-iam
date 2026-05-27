package profilefields

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	userprofile "github.com/a-digi/coco-iam/src/applications/userprofile"
	profile_entity "github.com/a-digi/coco-iam/src/organizations/profile/entity"
	"github.com/a-digi/coco-server/server/media"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

type putValidationResponse struct {
	Status string                   `json:"status"`
	Errors []userprofile.FieldError `json:"errors,omitempty"`
}

// PutProfileFieldsResponse is the envelope returned by
// PUT /a/{orgSlug}/{wsSlug}/{appSlug}/profile/fields.
type PutProfileFieldsResponse struct {
	Fields []userprofile.FieldWithValue `json:"fields"`
}

// PutProfileFieldsHandler serves
//
//	PUT /a/{orgSlug}/{wsSlug}/{appSlug}/profile/fields
//
// Replaces the authenticated user's full profile. Accepts
// multipart/form-data: a required "fields" JSON part plus one optional
// file part per file-type field being set (named by field name). File
// fields omitted from the request keep their existing stored value; set
// a file field to null in the "fields" JSON to explicitly clear it.
// All required fields (including required file fields with no prior
// value) must be satisfied. Files are scanned for malware before storage.
type PutProfileFieldsHandler struct {
	Slugs        SlugResolver
	Keys         KeyLoader
	Users        UserOrgReader
	FullFields   FullFieldLoader
	Reader       ProfileReader
	Saver        ProfileSaver
	Scanner      userprofile.Scanner
	VirusScanner userprofile.VirusScanner
	Store        userprofile.FileStore
	Files        userprofile.FileRepo
	Now          func() time.Time
}

// @Summary     Save user profile fields
// @Description Replaces the user's profile. Accepts multipart/form-data with a
//
//	"fields" JSON part and one file part per file-type field being set.
//	File fields absent from the request keep their existing stored value.
//	Set a file field to null in the "fields" JSON to explicitly clear it.
//	All required fields (including required file fields with no prior value)
//	must be present. Files are scanned for malware before storage.
//
// @Tags        app-profile-me
// @Accept      multipart/form-data
// @Produce     json
// @Param       orgSlug  path      string  true   "Organization slug"
// @Param       wsSlug   path      string  true   "Workspace slug"
// @Param       appSlug  path      string  true   "Application slug"
// @Param       fields   formData  string  true   "JSON object of field values"
// @Param       file     formData  file    false  "One part per file field, named by field name"
// @Success     200      {object}  PutProfileFieldsResponse
// @Failure     400      {object}  response.ErrorBody
// @Failure     401      {object}  response.ErrorBody
// @Failure     413      {object}  response.ErrorBody
// @Failure     415      {object}  response.ErrorBody
// @Failure     422      {object}  response.ErrorBody
// @Failure     503      {object}  response.ErrorBody
// @Failure     500      {object}  response.ErrorBody
// @Router      /a/{orgSlug}/{wsSlug}/{appSlug}/profile/fields [put]
func (h *PutProfileFieldsHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	orgSlug, wsSlug, appSlug, ok := parseSlugSegments(r.URL.Path)
	if !ok {
		response.ErrorResponse(w, http.StatusUnauthorized, genericUnauthorized)
		return
	}

	appID, orgID, err := h.Slugs.ResolveSlugs(orgSlug, wsSlug, appSlug)
	if err != nil {
		response.ErrorResponse(w, http.StatusUnauthorized, genericUnauthorized)
		return
	}

	loadKey := userprofile.LoadPublicKeyFunc(func(kid string) (*rsa.PublicKey, error) {
		return h.Keys.LoadPublicKey(appID, kid)
	})
	userOrg := userprofile.UserOrgLookupFunc(h.Users.UserOrg)
	nowFn := time.Now
	if h.Now != nil {
		nowFn = h.Now
	}

	userID, authErr := userprofile.AuthenticateUser(
		r.Header.Get("Authorization"),
		orgID,
		loadKey,
		userOrg,
		nowFn(),
	)
	if authErr != nil {
		if authErr.Status == http.StatusInternalServerError {
			response.ErrorResponse(w, http.StatusInternalServerError, genericUnauthorized)
			return
		}
		response.ErrorResponse(w, http.StatusUnauthorized, genericUnauthorized)
		return
	}

	const maxFiles = 20
	bodyLimit := int64(maxFiles)*media.MaxUploadBytes + 4096
	r.Body = http.MaxBytesReader(w, r.Body, bodyLimit)
	if err := r.ParseMultipartForm(bodyLimit); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid multipart payload")
		return
	}

	fieldsJSON := r.FormValue("fields")
	if fieldsJSON == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "fields form value is required")
		return
	}
	var jsonBody map[string]any
	if err := json.Unmarshal([]byte(fieldsJSON), &jsonBody); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "fields is not valid JSON")
		return
	}

	allFields, err := h.FullFields.ActiveFieldsFull(orgID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to load field definitions")
		return
	}

	_, currentData, err := h.Reader.LoadProfile(orgID, userID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to load current profile")
		return
	}
	if currentData == nil {
		currentData = map[string]interface{}{}
	}

	// Partition active fields into file vs non-file.
	var nonFileFields []profile_entity.ProfileField
	fileFieldsByName := make(map[string]*profile_entity.ProfileField, len(allFields))
	for i := range allFields {
		f := &allFields[i]
		if f.DataType == profile_entity.DataTypeFile {
			fileFieldsByName[f.Name] = f
		} else {
			nonFileFields = append(nonFileFields, *f)
		}
	}

	// Strip file-field keys from the JSON body before merging.
	// MergeProfileData rejects file-type fields; we handle them separately.
	nonFileBody := make(map[string]any, len(jsonBody))
	for k, v := range jsonBody {
		if _, isFile := fileFieldsByName[k]; !isFile {
			nonFileBody[k] = v
		}
	}
	mergedData, fieldErrs := userprofile.MergeProfileData(nonFileFields, currentData, nonFileBody)

	// Collect and validate file parts from the multipart form.
	type pendingFile struct {
		data     []byte
		mime     string
		ext      string
		filename string
	}
	pendingByName := make(map[string]pendingFile)

	if r.MultipartForm != nil {
		for fieldName, headers := range r.MultipartForm.File {
			if _, isFileField := fileFieldsByName[fieldName]; !isFileField {
				continue
			}
			if len(headers) == 0 {
				continue
			}
			header := headers[0]
			f, openErr := header.Open()
			if openErr != nil {
				response.ErrorResponse(w, http.StatusBadRequest, "could not read file for field: "+fieldName)
				return
			}
			buf, readErr := io.ReadAll(f)
			f.Close()
			if readErr != nil {
				response.ErrorResponse(w, http.StatusBadRequest, "could not read file for field: "+fieldName)
				return
			}

			head := buf
			if len(head) > 512 {
				head = head[:512]
			}
			claimed := header.Header.Get("Content-Type")
			detectedMime, ext, scanErr := h.Scanner.DetectAndValidate(head, claimed)
			if scanErr != nil {
				response.ErrorResponse(w, http.StatusUnsupportedMediaType, scanErr.Error())
				return
			}

			if h.VirusScanner != nil {
				if vErr := h.VirusScanner.Scan(buf); vErr != nil {
					if errors.Is(vErr, userprofile.ErrVirusFound) {
						response.ErrorResponse(w, http.StatusUnprocessableEntity, "file rejected: malware detected")
						return
					}
					response.ErrorResponse(w, http.StatusServiceUnavailable, "virus scanner unavailable")
					return
				}
			}

			field := fileFieldsByName[fieldName]
			allow, capBytes := userprofile.EffectiveLimits(field, detectedMime)
			if !userprofile.MimeAllowed(detectedMime, allow) {
				response.ErrorResponse(w, http.StatusUnsupportedMediaType, "mime type not allowed for field: "+fieldName)
				return
			}
			if capBytes > 0 && int64(len(buf)) > capBytes {
				response.ErrorResponse(w, http.StatusRequestEntityTooLarge, "file exceeds size limit for field: "+fieldName)
				return
			}

			pendingByName[fieldName] = pendingFile{
				data:     buf,
				mime:     detectedMime,
				ext:      ext,
				filename: media.SlugifyFilename(header.Filename),
			}
		}
	}

	// Check required file fields.
	for fieldName, field := range fileFieldsByName {
		if !field.IsRequired {
			continue
		}
		if _, hasNewFile := pendingByName[fieldName]; hasNewFile {
			continue
		}
		val, inBody := jsonBody[fieldName]
		if inBody && val == nil {
			// Explicit null on a required file field → not allowed.
			fieldErrs = append(fieldErrs, userprofile.FieldError{Field: fieldName, Message: "required"})
			continue
		}
		if _, hasExisting := currentData[fieldName]; !hasExisting {
			fieldErrs = append(fieldErrs, userprofile.FieldError{Field: fieldName, Message: "required"})
		}
	}

	if len(fieldErrs) > 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error":   true,
			"message": putValidationResponse{Status: "validation_failed", Errors: fieldErrs},
		})
		return
	}

	// Apply explicit nulls for file fields to mergedData.
	// (mergedData was seeded from currentData by MergeProfileData, so
	// file asset IDs are already carried forward; we only remove the
	// ones the caller explicitly clears.)
	for fieldName := range fileFieldsByName {
		if _, hasNewFile := pendingByName[fieldName]; hasNewFile {
			continue
		}
		val, inBody := jsonBody[fieldName]
		if inBody && val == nil {
			delete(mergedData, fieldName)
		}
	}

	// Persist non-file changes first so the profile row exists before
	// we start writing individual file assets.
	if err := h.Saver.SaveProfile(orgID, userID, mergedData); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to save profile")
		return
	}

	// Write each new file, update profile_data with the asset ID, then
	// delete the prior file for the same field (if any).
	for fieldName, pf := range pendingByName {
		assetID, genErr := newAssetIDLocal()
		if genErr != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, "failed to generate asset id")
			return
		}

		if storeErr := h.Store.Save(orgID, userID, assetID, pf.ext, pf.data); storeErr != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, "failed to store file")
			return
		}

		meta := userprofile.FileMeta{
			AssetID:   assetID,
			UserID:    userID,
			FieldName: fieldName,
			Filename:  pf.filename,
			MimeType:  pf.mime,
			SizeBytes: int64(len(pf.data)),
			Ext:       pf.ext,
		}
		if _, insertErr := h.Files.Insert(orgID, meta); insertErr != nil {
			_ = h.Store.Delete(orgID, userID, assetID, pf.ext)
			response.ErrorResponse(w, http.StatusInternalServerError, "failed to persist file metadata")
			return
		}

		// Find the prior asset before overwriting the profile_data key so
		// we still have the handle for cleanup.
		var priorAssetID, priorExt string
		if prior, findErr := h.Files.FindByField(orgID, userID, fieldName); findErr == nil && prior != nil && prior.AssetID != assetID {
			priorAssetID = prior.AssetID
			priorExt = prior.Ext
		}

		mergedData[fieldName] = assetID
		if saveErr := h.Saver.SaveProfile(orgID, userID, mergedData); saveErr != nil {
			_ = h.Store.Delete(orgID, userID, assetID, pf.ext)
			_ = h.Files.DeleteByAssetID(orgID, userID, assetID)
			response.ErrorResponse(w, http.StatusInternalServerError, "failed to update profile after file write")
			return
		}

		if priorAssetID != "" {
			_ = h.Store.Delete(orgID, userID, priorAssetID, priorExt)
			_ = h.Files.DeleteByAssetID(orgID, userID, priorAssetID)
		}
	}

	// Delete stored files for explicitly null-ed file fields.
	for fieldName := range fileFieldsByName {
		val, inBody := jsonBody[fieldName]
		if !inBody || val != nil {
			continue
		}
		if prior, findErr := h.Files.FindByField(orgID, userID, fieldName); findErr == nil && prior != nil {
			_ = h.Store.Delete(orgID, userID, prior.AssetID, prior.Ext)
			_ = h.Files.DeleteByAssetID(orgID, userID, prior.AssetID)
		}
	}

	// Reload confirmed state for the response.
	fields, finalData, err := h.Reader.LoadProfile(orgID, userID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to reload profile")
		return
	}

	response.SuccessResponse(w, http.StatusOK, PutProfileFieldsResponse{
		Fields: userprofile.BuildResponse(fields, finalData),
	})
}

// newAssetIDLocal generates a 32-char base64url asset ID using
// 24 bytes of crypto/rand. Kept local so the profilefields package
// stays independent of unexported helpers in userprofile.
func newAssetIDLocal() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
