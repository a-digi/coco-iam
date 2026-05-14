package admin

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/a-digi/coco-iam/src/userrules"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// VerifyHandler serves POST /api/v1/activation/verify. It is the first
// step of the two-step activation UX: the frontend posts the token
// from the URL plus the email the user typed; we confirm both match a
// pending row. Only on success does the UI reveal the password fields.
//
// The response is intentionally minimal — success is just {ok: true}.
// Every failure (bad token, expired, consumed, email mismatch,
// malformed body) collapses to the same generic 400 so this endpoint
// can't be used as a probe.
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

	svc := resolveActivationService(reqCtx)
	if svc == nil {
		return
	}

	if err := svc.Verify(req.Token, req.Email); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, genericFailureMsg)
		return
	}

	response.SuccessResponse(w, http.StatusOK, verifyResponse{
		Ok:    true,
		Rules: svc.RulesForToken(req.Token),
	})
}
