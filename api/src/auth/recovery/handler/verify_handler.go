package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/a-digi/coco-iam/src/userrules"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// VerifyHandler serves POST /api/v1/recovery/verify. First step of the
// two-step UX: frontend posts the token from the URL plus the email
// the user typed. On success we echo the applicable rule set so the
// UI can pre-validate the password in the next step. On any failure
// the response collapses to the generic 400.
type VerifyHandler struct{}

type verifyRequest struct {
	Token string `json:"token"`
	Email string `json:"email"`
}

type verifyResponse struct {
	Ok    bool              `json:"ok"`
	Rules userrules.RuleSet `json:"rules"`
}

func (h *VerifyHandler) ServeHTTP(reqCtx request.RequestContext) {
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

	svc := resolveService(reqCtx)
	if svc == nil {
		return
	}

	rules, err := svc.Verify(req.Token, req.Email)
	if err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, genericFailureMsg)
		return
	}

	response.SuccessResponse(w, http.StatusOK, verifyResponse{
		Ok:    true,
		Rules: rules,
	})
}
