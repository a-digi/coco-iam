// Package handler serves the admin + public HTTP surfaces for the
// per-application password recovery feature.
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/a-digi/coco-iam/src/applications/recoverypage"
	uri "github.com/a-digi/coco-lift/resource/uri"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

type bagGetter interface {
	Get(key string) (interface{}, bool)
}

func resolveService(reqCtx request.RequestContext) *recoverypage.Service {
	w := reqCtx.GetWriter()
	ctx := reqCtx.GetDI()
	bag, ok := ctx.(bagGetter)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "DI context has unexpected type")
		return nil
	}
	raw, ok := bag.Get(recoverypage.ContextBagKeyService)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "recoverypage service not available")
		return nil
	}
	svc, _ := raw.(*recoverypage.Service)
	if svc == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "recoverypage service has unexpected type")
	}
	return svc
}

func appIDFromPath(reqCtx request.RequestContext) string {
	key, value := uri.ExtractKeyAndValueFromURI(reqCtx.GetRequest().URL.Path)
	if key != "id" {
		return ""
	}
	return strings.TrimSpace(value)
}

func queryParam(reqCtx request.RequestContext, key string) string {
	return strings.TrimSpace(reqCtx.GetRequest().URL.Query().Get(key))
}

// -- public -------------------------------------------------------------

// PublicRequestHandler serves POST
// /api/v1/public/applications/recover/request?org=…&ws=…&app=….
// Always returns 200 — the service is silent on all branches so the
// endpoint can't be used to enumerate users on the ACL.
type PublicRequestHandler struct{}

type requestBody struct {
	Email string `json:"email"`
}

// @Summary     Request password recovery
// @Tags        app-recovery
// @Accept      json
// @Produce     json
// @Router      /public/applications/recover/request [post]
func (h *PublicRequestHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	svc := resolveService(reqCtx)
	if svc == nil {
		return
	}
	orgSlug := queryParam(reqCtx, "org")
	wsSlug := queryParam(reqCtx, "ws")
	clientID := queryParam(reqCtx, "app")
	if orgSlug == "" || wsSlug == "" || clientID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "missing org, ws, or app query parameter")
		return
	}
	var body requestBody
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	defer r.Body.Close()

	svc.StartRecovery(context.Background(), orgSlug, wsSlug, clientID, body.Email)
	response.SuccessResponse(w, http.StatusOK, map[string]bool{"ok": true})
}

// PublicResetHandler serves POST
// /api/v1/public/applications/recover/reset?org=…&ws=…&app=….
type PublicResetHandler struct{}

type resetBody struct {
	Token    string `json:"token"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// @Summary     Reset password via recovery token
// @Tags        app-recovery
// @Accept      json
// @Produce     json
// @Router      /public/applications/recover/reset [post]
func (h *PublicResetHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	svc := resolveService(reqCtx)
	if svc == nil {
		return
	}
	orgSlug := queryParam(reqCtx, "org")
	wsSlug := queryParam(reqCtx, "ws")
	clientID := queryParam(reqCtx, "app")
	if orgSlug == "" || wsSlug == "" || clientID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "missing org, ws, or app query parameter")
		return
	}
	var body resetBody
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	defer r.Body.Close()
	if body.Token == "" || body.Email == "" || body.Password == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "token, email, and password are required")
		return
	}

	err := svc.CompleteRecovery(orgSlug, wsSlug, clientID, body.Token, body.Email, body.Password)
	if err != nil {
		// ErrRecoveryFailed collapses every auth failure to a generic
		// 400. Rule violations surface verbatim (still 400).
		if errors.Is(err, recoverypage.ErrRecoveryFailed) {
			response.ErrorResponse(w, http.StatusBadRequest, "recovery failed")
			return
		}
		response.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, map[string]bool{"ok": true})
}
