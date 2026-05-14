package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/a-digi/coco-iam/src/auth/recovery"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// ResetHandler serves POST /api/v1/recovery/reset. Completes the flow
// — consumes the token and writes the new password if every check
// passes. Password-format errors (rule violations) surface verbatim
// because they're user-fixable and not security-sensitive. Auth
// failures collapse to the generic message.
type ResetHandler struct{}

type resetRequest struct {
	Token       string `json:"token"`
	Email       string `json:"email"`
	NewPassword string `json:"new_password"`
}

type resetResponse struct {
	Ok bool `json:"ok"`
}

func (h *ResetHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	var req resetRequest
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

	svc := resolveService(reqCtx)
	if svc == nil {
		return
	}

	if err := svc.Reset(req.Token, req.Email, req.NewPassword); err != nil {
		if errors.Is(err, recovery.ErrRecoveryFailed) {
			response.ErrorResponse(w, http.StatusBadRequest, genericFailureMsg)
			return
		}
		// Rule-violation / password-format error — safe to surface
		// because it describes policy, not identity.
		response.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	response.SuccessResponse(w, http.StatusOK, resetResponse{Ok: true})
}
