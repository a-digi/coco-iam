package admin

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/a-digi/coco-iam/src/activation"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// AdminPortalVerifyHandler serves POST /api/v1/activation/a/verify.
// Admin-only first step: confirms that the token from the email link
// matches a pending admin activation row. Never scans per-org databases.
// Every failure collapses to the same generic 400.
type AdminPortalVerifyHandler struct{}

// ServeHTTP verifies an admin-only activation token.
//
//	@Summary		Verify admin activation token
//	@Description	Admin-portal first step: confirms token+email match a pending admin activation row.
//	@Tags			activation
//	@Accept			json
//	@Produce		json
//	@Param			body	body		activation_entity.VerifyRequest	true	"Token and email"
//	@Success		200		{object}	activation_entity.VerifySuccess
//	@Failure		400		{object}	response.ErrorBody
//	@Router			/activation/a/verify [post]

func (h *AdminPortalVerifyHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	var req verifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, genericFailureMsg)
		return
	}
	defer r.Body.Close()
	req.Token = strings.TrimSpace(req.Token)
	req.Email = strings.TrimSpace(req.Email)
	if req.Token == "" || req.Email == "" {
		response.ErrorResponse(w, http.StatusBadRequest, genericFailureMsg)
		return
	}

	svc := resolveActivationService(reqCtx)
	if svc == nil {
		return
	}

	if err := svc.VerifyAdmin(req.Token, req.Email); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, genericFailureMsg)
		return
	}

	response.SuccessResponse(w, http.StatusOK, verifyResponse{
		Ok:    true,
		Rules: svc.RulesFor(activation.UserTypeAdmin, ""),
	})
}
