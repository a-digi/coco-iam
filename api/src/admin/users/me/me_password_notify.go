package me

import (
	"encoding/json"
	"net/http"

	adminpwnotify "github.com/a-digi/coco-iam/src/admin/users/passwordnotify"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// MePasswordNotificationGetHandler serves GET /admin/users/me/password-notification.
type MePasswordNotificationGetHandler struct{}

// @Summary     Get password notification preferences
// @Tags        admin-me
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} interface{}
// @Failure     400,401,403,500 {object} map[string]interface{}
// @Router      /admin/users/me/password-notification [get]
func (h *MePasswordNotificationGetHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	ctx := reqCtx.GetDI()

	userID, ok := subjectFromBearer(r.Header.Get("Authorization"))
	if !ok {
		response.ErrorResponse(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	manager := ctx.GetDatabaseManager()
	if manager == nil || manager.Connector == nil || manager.Connector.DB == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "database manager not available")
		return
	}

	repo := adminpwnotify.NewPrefsRepository(manager.Connector.DB)
	days, err := repo.Get(userID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.SuccessResponse(w, http.StatusOK, map[string]interface{}{
		"notify_days": days,
	})
}

// MePasswordNotificationPutHandler serves PUT /admin/users/me/password-notification.
type MePasswordNotificationPutHandler struct{}

type notifyPrefsBody struct {
	NotifyDays []int `json:"notify_days"`
}

// @Summary     Update password notification preferences
// @Tags        admin-me
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} interface{}
// @Failure     400,401,403,500 {object} map[string]interface{}
// @Router      /admin/users/me/password-notification [put]
func (h *MePasswordNotificationPutHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	ctx := reqCtx.GetDI()

	userID, ok := subjectFromBearer(r.Header.Get("Authorization"))
	if !ok {
		response.ErrorResponse(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var body notifyPrefsBody
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	defer r.Body.Close()

	if len(body.NotifyDays) > 10 {
		response.ErrorResponse(w, http.StatusBadRequest, "notify_days: maximum 10 items")
		return
	}
	for _, d := range body.NotifyDays {
		if d <= 0 {
			response.ErrorResponse(w, http.StatusBadRequest, "notify_days: each value must be greater than 0")
			return
		}
	}
	if body.NotifyDays == nil {
		body.NotifyDays = []int{}
	}

	manager := ctx.GetDatabaseManager()
	if manager == nil || manager.Connector == nil || manager.Connector.DB == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "database manager not available")
		return
	}

	repo := adminpwnotify.NewPrefsRepository(manager.Connector.DB)
	if err := repo.Upsert(userID, body.NotifyDays); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.SuccessResponse(w, http.StatusOK, map[string]interface{}{
		"notify_days": body.NotifyDays,
	})
}
