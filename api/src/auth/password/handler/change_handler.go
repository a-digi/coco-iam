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

// ChangeHandler serves POST /api/v1/account/password/change. Step 2:
// verify the current password (again — defense in depth) and write
// the new one. Password-format errors surface verbatim so the UI can
// show a useful message (too short, same as old). Everything else
// collapses to the generic failure string.
type ChangeHandler struct{}

type changeRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type changeResponse struct {
	Ok bool `json:"ok"`
}

func (h *ChangeHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	userID := userIDFromRequest(reqCtx)
	if userID == "" {
		return
	}

	var req changeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, genericFailureMsg)
		return
	}
	defer r.Body.Close()
	if strings.TrimSpace(req.CurrentPassword) == "" || req.NewPassword == "" {
		response.ErrorResponse(w, http.StatusBadRequest, genericFailureMsg)
		return
	}

	svc := resolveService(reqCtx)
	if svc == nil {
		return
	}

	if err := svc.Change(userID, req.CurrentPassword, req.NewPassword); err != nil {
		if errors.Is(err, password.ErrChangeFailed) {
			response.ErrorResponse(w, http.StatusBadRequest, genericFailureMsg)
			return
		}
		// Password-format errors are fine to surface — they're not
		// security-sensitive and the UI wants a precise message.
		response.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	response.SuccessResponse(w, http.StatusOK, changeResponse{Ok: true})
}
