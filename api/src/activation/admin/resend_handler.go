package admin

import (
	"context"
	"errors"
	"net/http"

	"github.com/a-digi/coco-iam/src/activation"
	"github.com/a-digi/coco-lift/resource/uri"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// ResendAdminHandler serves POST /admin/admin_users/{id}/resend-activation.
type ResendAdminHandler struct{}

// ResendUserHandler serves POST /admin/users/{id}/resend-activation.
type ResendUserHandler struct{}

// @Summary     Resend admin activation email
// @Description Re-sends the activation email for an inactive admin user.
// @Tags        activation
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Admin user ID"
// @Success     202 {object} interface{}
// @Failure     400,429,500 {object} map[string]interface{}
// @Router      /admin/activation/admin/{id}/resend [post]
func (h *ResendAdminHandler) ServeHTTP(reqCtx request.RequestContext) {
	resend(reqCtx, activation.UserTypeAdmin)
}

// @Summary     Resend org user activation email
// @Description Re-sends the activation email for an inactive organization user.
// @Tags        activation
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Org user ID"
// @Success     202 {object} interface{}
// @Failure     400,429,500 {object} map[string]interface{}
// @Router      /admin/activation/user/{id}/resend [post]
func (h *ResendUserHandler) ServeHTTP(reqCtx request.RequestContext) {
	resend(reqCtx, activation.UserTypeUser)
}

func resend(reqCtx request.RequestContext, userType activation.UserType) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	key, value := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if key != "id" || value == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "user id is required")
		return
	}
	svc := resolveActivationService(reqCtx)
	if svc == nil {
		return
	}
	res, err := svc.Resend(context.Background(), userType, value)
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
