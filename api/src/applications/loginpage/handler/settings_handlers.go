package handler

import (
	"encoding/json"
	"net/http"

	"github.com/a-digi/coco-iam/src/applications/loginpage"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// GetSettingsHandler serves GET /api/v1/applications/{id}/login-settings.
// Returns the saved settings plus a `configured` flag that the admin UI
// uses to render a red dot on the "Settings" tab when the login page is
// not ready to accept credentials.
type GetSettingsHandler struct{}

type settingsResponse struct {
	Settings   loginpage.Settings `json:"settings"`
	Configured bool               `json:"configured"`
	// Slug trio for the admin "Open login page" / "Copy link" URL
	// builder. Empty when the parent chain is missing (shouldn't
	// happen in practice). Populated only on the GET path.
	OrganizationSlug string `json:"organization_slug,omitempty"`
	WorkspaceSlug    string `json:"workspace_slug,omitempty"`
	ClientID         string `json:"client_id,omitempty"`
}

func (h *GetSettingsHandler) ServeHTTP(reqCtx request.RequestContext) {
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
	s, err := svc.LoadSettings(appID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp := settingsResponse{
		Settings:   s,
		Configured: s.IsConfigured(),
	}
	// Best-effort enrichment with the slug trio — failure here is not
	// fatal, the admin just won't get an "Open login page" URL.
	if slugs, err := svc.Store.LookupSlugsByAppID(appID); err == nil {
		resp.OrganizationSlug = slugs.OrganizationSlug
		resp.WorkspaceSlug = slugs.WorkspaceSlug
		resp.ClientID = slugs.ClientID
	}
	response.SuccessResponse(w, http.StatusOK, resp)
}

// UpdateSettingsHandler serves PATCH /api/v1/applications/{id}/login-settings.
type UpdateSettingsHandler struct{}

func (h *UpdateSettingsHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	svc := resolveService(reqCtx)
	if svc == nil {
		return
	}
	appID := appIDFromPath(reqCtx)
	if appID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "missing application id")
		return
	}
	var in loginpage.Settings
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	defer r.Body.Close()
	in.ApplicationID = appID
	fresh, err := svc.SaveSettings(in)
	if err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, settingsResponse{
		Settings:   fresh,
		Configured: fresh.IsConfigured(),
	})
}
