package handler

import (
	"net/http"

	"github.com/a-digi/coco-iam/src/auth/scopecheck"
	"github.com/a-digi/coco-iam/src/organizations/profile/entity"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// OrgUserProfileGetHandler — GET /api/v1/organizations/{orgId}/users/{userId}/profile
// Returns the org user's profile + active schema.
//
// Access rules:
//   - super:admin → always
//   - organizations:users:profile:read → any user in the org
//   - user:me → only when the JWT sub matches {userId}
type OrgUserProfileGetHandler struct{}

type userProfileResponse struct {
	UserID   string                 `json:"user_id"`
	Fields   []entity.ProfileField  `json:"fields"`
	Data     map[string]interface{} `json:"profile_data"`
	UpdatedAt string                `json:"updated_at,omitempty"`
}

// @Summary     Get organization user profile
// @Tags        org-users
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Organization ID"
// @Param       userId path string true "User ID"
// @Router      /organizations/organizations/{id}/users/{userId}/profile [get]
func (h *OrgUserProfileGetHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	userID := extractToken(r.URL.Path, "userId")
	if userID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "user id is required")
		return
	}

	// Authorise: super:admin OR org-read OR (user:me && caller == userID)
	checker := scopecheck.NewChecker()
	hasOrgRead, _ := checker.HasAnyScope(r.Header,
		"organizations:users:profile:read",
		"organizations:users:profile",
		"organizations",
	)
	if !hasOrgRead {
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
	profile, err := repo.GetUserProfile(userID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	resp := userProfileResponse{
		UserID: userID,
		Fields: fields,
		Data:   map[string]interface{}{},
	}
	if profile != nil {
		resp.Data = profile.ProfileData
		resp.UpdatedAt = profile.UpdatedAt
	}
	response.SuccessResponse(w, http.StatusOK, resp)
}
