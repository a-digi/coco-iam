package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/a-digi/coco-iam/src/auth/password"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// VerifyHandler serves POST /api/v1/account/password/verify. Step 1 of
// the UX: confirm the user's current password before unveiling the
// set-new-password fields. Any failure collapses to the generic 400 so
// the endpoint can't be used as a password oracle.
type VerifyHandler struct{}

type verifyRequest struct {
	CurrentPassword string `json:"current_password"`
}

type verifyResponse struct {
	Ok bool `json:"ok"`
}

func (h *VerifyHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	userID := userIDFromRequest(reqCtx)
	if userID == "" {
		return
	}

	var req verifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, genericFailureMsg)
		return
	}
	defer r.Body.Close()
	if strings.TrimSpace(req.CurrentPassword) == "" {
		response.ErrorResponse(w, http.StatusBadRequest, genericFailureMsg)
		return
	}

	svc := resolveService(reqCtx)
	if svc == nil {
		return
	}

	if err := svc.Verify(userID, req.CurrentPassword); err != nil {
		if errors.Is(err, password.ErrChangeFailed) {
			response.ErrorResponse(w, http.StatusBadRequest, genericFailureMsg)
			return
		}
		response.ErrorResponse(w, http.StatusBadRequest, genericFailureMsg)
		return
	}

	response.SuccessResponse(w, http.StatusOK, verifyResponse{Ok: true})
}
