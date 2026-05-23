package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// RequestHandler serves POST /api/v1/recovery/request. Takes just an
// email; the response is always 200 with `{ok:true}` regardless of
// whether the email actually maps to an account. That uniform answer
// is the security property — it prevents account enumeration.
type RequestHandler struct{}

type requestBody struct {
	Email string `json:"email"`
}

type requestResponse struct {
	Ok bool `json:"ok"`
}

// @Summary     Request password recovery
// @Tags        recovery
// @Accept      json
// @Produce     json
// @Router      /recovery/request [post]
func (h *RequestHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	var body requestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		// Malformed body still returns 200 — same external behaviour
		// regardless of whether we can parse.
		response.SuccessResponse(w, http.StatusOK, requestResponse{Ok: true})
		return
	}
	defer r.Body.Close()
	body.Email = strings.TrimSpace(body.Email)

	svc := resolveService(reqCtx)
	if svc == nil {
		return
	}
	// Run in the background-safe way: Request() is intentionally
	// no-op on any internal failure.
	svc.Request(context.Background(), body.Email)

	response.SuccessResponse(w, http.StatusOK, requestResponse{Ok: true})
}
