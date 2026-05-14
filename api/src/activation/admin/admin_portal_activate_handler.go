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
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// AdminPortalActivateHandler serves POST /api/v1/activation/a/activate.
// Admin-only: checks admin_activations exclusively and always issues a
// JWT on success. Never scans per-org databases.
type AdminPortalActivateHandler struct{}

func (h *AdminPortalActivateHandler) ServeHTTP(reqCtx request.RequestContext) {
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
		response.ErrorResponse(w, http.StatusBadRequest, genericFailureMsg)
		return
	}

	svc := resolveActivationService(reqCtx)
	if svc == nil {
		return
	}

	result, err := svc.ActivateAdmin(req.Token, req.Email, req.NewPassword)
	if err != nil {
		switch {
		case errors.Is(err, activation.ErrActivationFailed),
			errors.Is(err, activation.ErrNotFound),
			errors.Is(err, activation.ErrExpired),
			errors.Is(err, activation.ErrAlreadyUsed):
			response.ErrorResponse(w, http.StatusBadRequest, genericFailureMsg)
		default:
			response.ErrorResponse(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	out := activateResponse{RedirectURL: result.RedirectURL}

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

	response.SuccessResponse(w, http.StatusOK, out)
}
