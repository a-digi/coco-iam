package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/a-digi/coco-iam/config"
	"github.com/a-digi/coco-iam/src/activation"
	user_acl_repo "github.com/a-digi/coco-iam/src/admin/acl/repository"
	"github.com/a-digi/coco-iam/src/auth/oauth"
	oauth_lib "github.com/a-digi/coco-oauth/oauth"
	oauth_model "github.com/a-digi/coco-oauth/oauth/model"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// ActivateHandler serves POST /api/v1/activation/activate. Public — the
// supplied token + email pair is the authenticator. Every authentication
// failure (bad token, expired, consumed, email mismatch) collapses to the
// same generic error message so attackers can't probe which part failed.
type ActivateHandler struct{}

// genericFailureMsg is the only error string returned for any
// authentication-related activation failure. Kept deliberately vague.
const genericFailureMsg = "Something went wrong. The activation link may be invalid, expired, or already used."

type activateRequest struct {
	Token       string `json:"token"`
	Email       string `json:"email"`
	NewPassword string `json:"new_password"`
}

type activateResponse struct {
	Token *oauth_model.TokenResponse `json:"token,omitempty"`
	// RedirectURL is the per-app login path the admin selected at
	// user-create time (empty when none). The frontend uses it as the
	// "Go to login" button target; empty → fall back to /login.
	RedirectURL string `json:"redirect_url,omitempty"`
}

// @Summary     Activate account
// @Description Consumes a one-time activation token and sets the user password. Returns a JWT on success for admin users.
// @Tags        activation
// @Accept      json
// @Produce     json
// @Param       body body activation_entity.ActivateRequest true "Activation payload"
// @Success     200 {object} activation_entity.ActivateSuccess
// @Failure     400,500 {object} response.ErrorBody
// @Router      /activation/activate [post]
func (h *ActivateHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	ctx := reqCtx.GetDI()

	var req activateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, genericFailureMsg)
		return
	}
	defer r.Body.Close()
	req.Token = strings.TrimSpace(req.Token)
	req.Email = strings.TrimSpace(req.Email)
	if req.Token == "" || req.Email == "" || req.NewPassword == "" {
		// Missing fields look indistinguishable from a malformed attempt.
		response.ErrorResponse(w, http.StatusBadRequest, genericFailureMsg)
		return
	}

	svc := resolveActivationService(reqCtx)
	if svc == nil {
		return
	}

	result, err := svc.Activate(req.Token, req.Email, req.NewPassword)
	if err != nil {
		// Password-format problems are the only non-security-sensitive
		// failure — let them through with a precise message for UX. All
		// other failures (token + email checks) funnel into the generic
		// message.
		switch {
		case errors.Is(err, activation.ErrActivationFailed),
			errors.Is(err, activation.ErrNotFound),
			errors.Is(err, activation.ErrExpired),
			errors.Is(err, activation.ErrAlreadyUsed):
			response.ErrorResponse(w, http.StatusBadRequest, genericFailureMsg)
		default:
			// New-password length / empty errors surface as-is.
			response.ErrorResponse(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	out := activateResponse{RedirectURL: result.RedirectURL}

	// Admin users get an immediate JWT with their real scopes. Regular
	// users have no login path yet — the response carries no user data
	// by design (we don't echo back the resolved email/username).
	if result.UserType == activation.UserTypeAdmin {
		manager := ctx.GetDatabaseManager()
		if manager != nil {
			aclRepo := user_acl_repo.NewUserAclRepository(manager)
			scopes, scopeErr := aclRepo.FindUserScopes(result.UserID)
			if scopeErr == nil && len(scopes) > 0 {
				cfgBytes, cErr := config.ReadConfigFile("config.json")
				if cErr == nil {
					cfg, lerr := oauth_lib.LoadAuthConfigFromBytes(cfgBytes)
					if lerr == nil {
						if tok, tErr := oauth.IssueToken(cfg, result.UserID, scopes); tErr == nil {
							out.Token = &tok
						}
					}
				}
			}
		}
	}

	response.SuccessResponse(w, http.StatusOK, out)
}
