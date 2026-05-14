package userprofile

import (
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"time"

	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// PatchMeHandler serves
//
//	PATCH /a/{orgSlug}/{wsSlug}/{appSlug}/profile/me
//
// Same auth model + slug resolution as GetMeHandler. Body is a JSON
// object with a `profile_data` field; the handler merges it onto the
// current profile_data via MergeProfileData (pure), rejects unknown
// and file-type keys, and applies the result via ProfileWriter.
//
// Thin orchestrator: parse → auth → read current → merge → write.
// All decision logic lives in authenticateUser + MergeProfileData.
// A nil Now field falls back to time.Now so callers don't have to
// set it unless they want deterministic behaviour.
type PatchMeHandler struct {
	Slugs    SlugResolver
	Keys     KeyLoader
	Users    UserOrgReader
	Profiles ProfileReader
	Writer   ProfileWriter
	Now      func() time.Time
}

type patchMeRequest struct {
	ProfileData map[string]any `json:"profile_data"`
}

type patchMeValidationResponse struct {
	Status string       `json:"status"`
	Errors []FieldError `json:"errors,omitempty"`
}

func (h *PatchMeHandler) ServeHTTP(reqCtx request.RequestContext) {
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

	loadKey := LoadPublicKeyFunc(func(kid string) (*rsa.PublicKey, error) {
		return h.Keys.LoadPublicKey(appID, kid)
	})
	userOrg := UserOrgLookupFunc(h.Users.UserOrg)

	nowFn := time.Now
	if h.Now != nil {
		nowFn = h.Now
	}

	userID, authErr := authenticateUser(
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

	var body patchMeRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.ProfileData == nil {
		// Empty patch is a no-op; return the current response shape
		// so clients can treat PATCH as always returning the current
		// state.
		fields, data, err := h.Profiles.LoadProfile(orgID, userID)
		if err != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, "failed to load profile")
			return
		}
		response.SuccessResponse(w, http.StatusOK, meResponse{Fields: BuildResponse(fields, data)})
		return
	}

	fields, current, err := h.Profiles.LoadProfile(orgID, userID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to load profile")
		return
	}

	merged, fieldErrs := MergeProfileData(fields, current, body.ProfileData)
	if len(fieldErrs) > 0 {
		// SuccessResponse hardcodes 200, so write the 422 directly.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error":   true,
			"message": patchMeValidationResponse{Status: "validation_failed", Errors: fieldErrs},
		})
		return
	}

	// Apply the CLEANED values MergeProfileData produced, not the
	// raw patch — the helper trims strings, coerces numbers, sorts
	// multi-select arrays, and strips duplicates. Writing the raw
	// body would store un-normalised values. A key missing from
	// `merged` means the patch explicitly cleared it; the writer
	// receives nil and the adapter deletes the key from profile_data.
	for key := range body.ProfileData {
		var value any
		if v, ok := merged[key]; ok {
			value = v
		}
		if err := h.Writer.UpdateFieldValue(orgID, userID, key, value); err != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, "failed to persist profile")
			return
		}
	}

	fields, data, err := h.Profiles.LoadProfile(orgID, userID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to load profile")
		return
	}
	response.SuccessResponse(w, http.StatusOK, meResponse{Fields: BuildResponse(fields, data)})
}
