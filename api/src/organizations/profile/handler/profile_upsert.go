package handler

import (
	"net/http"

	"github.com/a-digi/coco-iam/src/auth/scopecheck"
	"github.com/a-digi/coco-iam/src/organizations/profile"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// OrgUserProfileUpsertHandler — PUT /api/v1/organizations/{orgId}/users/{userId}/profile
// Validates the incoming data map against the active schema and stores it.
//
// Access rules:
//   - super:admin → always
//   - organizations:users:profile:write → any user in the org
//   - user:me → only when the JWT sub matches {userId}
type OrgUserProfileUpsertHandler struct{}

type upsertRequest struct {
	ProfileData map[string]interface{} `json:"profile_data"`
}

type upsertResponse struct {
	Status string             `json:"status"`
	Errors []profile.FieldError `json:"errors,omitempty"`
}

func (h *OrgUserProfileUpsertHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	userID := extractToken(r.URL.Path, "userId")
	if userID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "user id is required")
		return
	}

	checker := scopecheck.NewChecker()
	hasOrgWrite, _ := checker.HasAnyScope(r.Header,
		"organizations:users:profile:write",
		"organizations:users:profile",
		"organizations",
	)
	if !hasOrgWrite {
		if callerID(reqCtx) != userID {
			response.ErrorResponse(w, http.StatusForbidden, "forbidden")
			return
		}
	}

	_, repo, err := repositoryFromRequest(reqCtx)
	if err != nil {
		writeErr(w, err)
		return
	}

	fields, err := repo.ListFields(true)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	var body upsertRequest
	if err := decodeJSONBody(reqCtx, &body); err != nil {
		writeErr(w, err)
		return
	}
	if body.ProfileData == nil {
		body.ProfileData = map[string]interface{}{}
	}

	cleaned, errs := profile.Validate(fields, body.ProfileData)
	if len(errs) > 0 {
		response.SuccessResponse(w, http.StatusUnprocessableEntity, upsertResponse{
			Status: "validation_failed",
			Errors: errs,
		})
		return
	}

	if err := repo.UpsertUserProfile(userID, cleaned); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to save profile: "+err.Error())
		return
	}

	response.SuccessResponse(w, http.StatusOK, upsertResponse{Status: "saved"})
}
