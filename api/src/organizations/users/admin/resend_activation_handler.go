package admin

import (
	"context"
	"errors"
	"net/http"

	"github.com/a-digi/coco-lift/resource/uri"
	"github.com/a-digi/coco-iam/src/activation"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// ResendOrgUserActivationHandler serves
// POST /{res:organization_users}/{id}/resend-activation.
// Resend is allowed when the user has never activated (activation_pending) OR
// when the account is inactive (is_active = false). It is blocked only when
// the user has both activated and is still active — a fully operational account.
// Callers must hold organizations:users:write scope (enforced by the route layer).
type ResendOrgUserActivationHandler struct{}

// @Summary     Resend organization user activation email
// @Tags        org-users
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "User ID"
// @Router      /organizations/organization_users/{id}/resend-activation [post]
func (h *ResendOrgUserActivationHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	key, userID := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if key != "id" || userID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "user id is required")
		return
	}

	mainDB, reg, ok := resolveDBs(reqCtx, w)
	if !ok {
		return
	}

	orgDB, orgID, err := orgDBForUser(mainDB, reg, userID)
	if err != nil {
		response.ErrorResponse(w, http.StatusNotFound, "user not found")
		return
	}

	user, err := fetchOrgUser(orgDB, orgID, userID)
	if err != nil {
		response.ErrorResponse(w, http.StatusNotFound, "user not found")
		return
	}

	svc := resolveActivationService(reqCtx.GetDI())
	if svc == nil {
		return
	}

	activated, err := svc.IsActivated(activation.UserTypeUser, userID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Block only when the account is both activated and still active —
	// a fully operational account that needs no resend.
	if activated && user.IsActive {
		response.ErrorResponse(w, http.StatusUnprocessableEntity, "user has already been activated")
		return
	}

	res, err := svc.Resend(context.Background(), activation.UserTypeUser, userID)
	if err != nil {
		switch {
		case errors.Is(err, activation.ErrCooldown):
			response.ErrorResponse(w, http.StatusTooManyRequests, err.Error())
		default:
			response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	response.SuccessResponse(w, http.StatusAccepted, map[string]string{
		"status":     "sent",
		"expires_at": res.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}
