package admin

import (
	"encoding/json"
	"net/http"

	"github.com/a-digi/coco-iam/config"
	"github.com/a-digi/coco-iam/src/activation"
	user_acl_repo "github.com/a-digi/coco-iam/src/admin/acl/repository"
	"github.com/a-digi/coco-iam/src/auth/oauth"
	oauth_lib "github.com/a-digi/coco-oauth/oauth"
	oauth_model "github.com/a-digi/coco-oauth/oauth/model"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// ChangePasswordHandler serves POST /api/v1/auth/change-password. Used by
// admins who logged in via the temp password and now need to set their
// real one. Guarded by the `system:pwd_reset_required` scope — the
// scope security layer only issues that scope to users whose
// must_change_password flag is TRUE.
type ChangePasswordHandler struct{}

// ServeHTTP forces a password change for the authenticated admin.
//
//	@Summary		Force password change
//	@Description	For admins with must_change_password=true. Accepts a new password and re-issues a full JWT.
//	@Tags			activation
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		activation_entity.ChangePasswordRequest	true	"New password"
//	@Success		200		{object}	activation_entity.ChangePasswordSuccess
//	@Failure		400		{object}	response.ErrorBody
//	@Failure		401		{object}	response.ErrorBody
//	@Router			/auth/change-password [post]

type changePasswordRequest struct {
	NewPassword string `json:"new_password"`
}

type changePasswordResponse struct {
	Token *oauth_model.TokenResponse `json:"token,omitempty"`
}

func (h *ChangePasswordHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	ctx := reqCtx.GetDI()

	var req changePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	defer r.Body.Close()
	if req.NewPassword == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "new_password is required")
		return
	}

	// Extract the user id from the Bearer token. The security layer has
	// already validated signature + scope — we just need the subject.
	cfgBytes, err := config.ReadConfigFile("config.json")
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "auth config error")
		return
	}
	cfg, err := oauth_lib.LoadAuthConfigFromBytes(cfgBytes)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "auth config error")
		return
	}
	validator, err := oauth_lib.NewValidatorFromConfig(cfg)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "auth validator error")
		return
	}
	token, err := oauth_lib.ExtractBearer(r.Header.Get("Authorization"))
	if err != nil {
		response.ErrorResponse(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	userID, _, _, err := validator.Validate(token)
	if err != nil || userID == "" {
		response.ErrorResponse(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	svc := resolveActivationService(reqCtx)
	if svc == nil {
		return
	}
	// Forced-change always applies to admins today — regular users don't
	// have a login path yet. If that changes we'll pass the user_type via
	// a claim.
	if err := svc.ChangePasswordForUser(activation.UserTypeAdmin, userID, req.NewPassword); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	// Mint a fresh JWT with the user's real scopes — the must-change scope
	// is intentionally dropped.
	manager := ctx.GetDatabaseManager()
	if manager == nil {
		response.SuccessResponse(w, http.StatusOK, changePasswordResponse{})
		return
	}
	scopes, serr := user_acl_repo.NewUserAclRepository(manager).FindUserScopes(userID)
	if serr != nil || len(scopes) == 0 {
		response.SuccessResponse(w, http.StatusOK, changePasswordResponse{})
		return
	}
	tok, tErr := oauth.IssueToken(cfg, userID, scopes)
	if tErr != nil {
		response.SuccessResponse(w, http.StatusOK, changePasswordResponse{})
		return
	}
	response.SuccessResponse(w, http.StatusOK, changePasswordResponse{Token: &tok})
}
