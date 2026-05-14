// Package handler serves the per-application key management HTTP
// endpoints. Admin endpoints for listing and lifecycle actions live
// here; the public JWKS + renew endpoints are in public_handlers.go
// and authentication/renew.go respectively.
package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/a-digi/coco-iam/config"
	"github.com/a-digi/coco-iam/src/applications/keys"
	oauth_lib "github.com/a-digi/coco-oauth/oauth"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// PrivateKeyScope is the scope a caller must hold for the GET handler
// to include the private PEM bytes in its response. Without it the
// shape is the same; just `private_pem` is the empty string and the
// UI renders a "no access" block in that section.
const PrivateKeyScope = "applications:keys:read_private"

// ListKeysHandler serves GET /api/v1/applications/{id}/keys.
// Returns every non-expired key for the application, newest first.
// Callers decide by status which section to render the entry in.
type ListKeysHandler struct{}

type listResponse struct {
	Keys []keys.Keypair `json:"keys"`
}

func (h *ListKeysHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	svc := resolveService(reqCtx)
	if svc == nil {
		return
	}
	appID := appIDFromPath(reqCtx)
	if appID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "missing application id")
		return
	}
	includePrivate := callerCanReadPrivate(reqCtx)

	// Lazy fallback: an application row may exist without a keypair
	// if something went sideways during its create listener. Heal
	// that on read rather than 500ing.
	if _, err := svc.ActiveRow(appID); err != nil {
		if errors.Is(err, keys.ErrNotFound) {
			if aerr := svc.EnsureActive(appID); aerr != nil {
				response.ErrorResponse(w, http.StatusInternalServerError, aerr.Error())
				return
			}
		} else {
			response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	kps, err := svc.Keypairs(appID, includePrivate)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, listResponse{Keys: kps})
}

// RegenerateKeysHandler serves POST /applications/{id}/keys/regenerate.
// Creates a pending key alongside the active one. 409 if a pending key
// already exists — admins must discard or accept the existing one
// first.
type RegenerateKeysHandler struct{}

func (h *RegenerateKeysHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	svc := resolveService(reqCtx)
	if svc == nil {
		return
	}
	appID := appIDFromPath(reqCtx)
	if appID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "missing application id")
		return
	}
	row, err := svc.GeneratePending(appID)
	if err != nil {
		if errors.Is(err, keys.ErrPendingExists) {
			response.ErrorResponse(w, http.StatusConflict, err.Error())
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	kp, err := svc.Keypair(appID, row.ID, callerCanReadPrivate(reqCtx))
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusCreated, kp)
}

// ActivatePendingHandler serves POST
// /applications/{id}/keys/activate-pending. Promotes the pending key
// to active and demotes the current active to deactivated with a 24h
// expiry.
type ActivatePendingHandler struct{}

func (h *ActivatePendingHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	svc := resolveService(reqCtx)
	if svc == nil {
		return
	}
	appID := appIDFromPath(reqCtx)
	if appID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "missing application id")
		return
	}
	if err := svc.ActivatePending(appID); err != nil {
		if errors.Is(err, keys.ErrNoPending) {
			response.ErrorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, map[string]string{"status": "activated"})
}

// DiscardPendingHandler serves POST
// /applications/{id}/keys/discard-pending.
type DiscardPendingHandler struct{}

func (h *DiscardPendingHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	svc := resolveService(reqCtx)
	if svc == nil {
		return
	}
	appID := appIDFromPath(reqCtx)
	if appID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "missing application id")
		return
	}
	if err := svc.DiscardPending(appID); err != nil {
		if errors.Is(err, keys.ErrNoPending) {
			response.ErrorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, map[string]string{"status": "discarded"})
}

// DeactivateKeyHandler serves POST /applications/{id}/keys/{keyId}/deactivate
// — the "force expire" action. Only deactivated keys can be targeted;
// active/pending/expired keys are rejected with 409 / 404.
type DeactivateKeyHandler struct{}

func (h *DeactivateKeyHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	svc := resolveService(reqCtx)
	if svc == nil {
		return
	}
	r := reqCtx.GetRequest()
	appID := appIDFromPath(reqCtx)
	if appID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "missing application id")
		return
	}
	// Route is `/applications/{id}/keys/{keyId}/deactivate`. The URL
	// uri pattern matcher only returns one `{key:value}` pair per
	// call, so we fish out the second id by hand.
	keyID := segmentBetween(r.URL.Path, "keys", "deactivate")
	if keyID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "missing key id")
		return
	}
	if err := svc.DeactivateCompletely(appID, keyID); err != nil {
		switch {
		case errors.Is(err, keys.ErrNotFound):
			response.ErrorResponse(w, http.StatusNotFound, err.Error())
		case errors.Is(err, keys.ErrNotDeactivated):
			response.ErrorResponse(w, http.StatusConflict, err.Error())
		default:
			response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	response.SuccessResponse(w, http.StatusOK, map[string]string{"status": "expired"})
}

// segmentBetween returns the path segment that sits between two known
// markers. Used by the deactivate handler to locate the `{keyId}` slot
// in `.../keys/<keyId>/deactivate`.
func segmentBetween(path, start, end string) string {
	segs := strings.Split(strings.Trim(path, "/"), "/")
	for i := 0; i+2 < len(segs); i++ {
		if segs[i] == start && segs[i+2] == end {
			return strings.TrimSpace(segs[i+1])
		}
	}
	return ""
}

// callerCanReadPrivate revalidates the bearer token and returns true
// iff the caller holds PrivateKeyScope or super:admin. Failures
// default to false — missing the private PEM is the safe mode.
func callerCanReadPrivate(reqCtx request.RequestContext) bool {
	auth := reqCtx.GetRequest().Header.Get("Authorization")
	token, err := oauth_lib.ExtractBearer(auth)
	if err != nil {
		return false
	}
	cfgBytes, err := config.ReadConfigFile("config.json")
	if err != nil {
		return false
	}
	cfg, err := oauth_lib.LoadAuthConfigFromBytes(cfgBytes)
	if err != nil {
		return false
	}
	validator, err := oauth_lib.NewValidatorFromConfig(cfg)
	if err != nil {
		return false
	}
	_, scopes, _, err := validator.Validate(token)
	if err != nil {
		return false
	}
	for _, s := range scopes {
		if s == PrivateKeyScope || s == "super:admin" {
			return true
		}
	}
	return false
}
