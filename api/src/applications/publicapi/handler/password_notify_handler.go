package handler

import (
	"encoding/json"
	"net/http"

	"github.com/a-digi/coco-iam/src/applications/publicapi/auth"
	orgpwnotify "github.com/a-digi/coco-iam/src/organizations/users/passwordnotify"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// PasswordNotificationGetHandler serves GET .../me/password-notification.
// Returns the authenticated org user's notification day preferences.
type PasswordNotificationGetHandler struct{}

func (h *PasswordNotificationGetHandler) ServeHTTP(reqCtx request.RequestContext) {
	caller := auth.Authenticate(reqCtx, "user:me")
	if caller == nil {
		return
	}
	orgDB := caller.OrgDB
	if orgDB == nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, "org database unavailable")
		return
	}

	repo := orgpwnotify.NewPrefsRepository(orgDB)
	days, err := repo.Get(caller.UserID)
	if err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, err.Error())
		return
	}

	response.SuccessResponse(reqCtx.GetWriter(), http.StatusOK, map[string]interface{}{
		"notify_days": days,
	})
}

// PasswordNotificationPutHandler serves PUT .../me/password-notification.
// Replaces the authenticated org user's notification day preferences.
type PasswordNotificationPutHandler struct{}

type orgNotifyPrefsBody struct {
	NotifyDays []int `json:"notify_days"`
}

func (h *PasswordNotificationPutHandler) ServeHTTP(reqCtx request.RequestContext) {
	caller := auth.Authenticate(reqCtx, "user:me")
	if caller == nil {
		return
	}
	orgDB := caller.OrgDB
	if orgDB == nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, "org database unavailable")
		return
	}

	var body orgNotifyPrefsBody
	dec := json.NewDecoder(reqCtx.GetRequest().Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	defer reqCtx.GetRequest().Body.Close()

	if len(body.NotifyDays) > 10 {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusBadRequest, "notify_days: maximum 10 items")
		return
	}
	for _, d := range body.NotifyDays {
		if d <= 0 {
			response.ErrorResponse(reqCtx.GetWriter(), http.StatusBadRequest, "notify_days: each value must be greater than 0")
			return
		}
	}
	if body.NotifyDays == nil {
		body.NotifyDays = []int{}
	}

	repo := orgpwnotify.NewPrefsRepository(orgDB)
	if err := repo.Upsert(caller.UserID, body.NotifyDays); err != nil {
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusInternalServerError, err.Error())
		return
	}

	response.SuccessResponse(reqCtx.GetWriter(), http.StatusOK, map[string]interface{}{
		"notify_days": body.NotifyDays,
	})
}
