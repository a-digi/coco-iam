package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/a-digi/coco-iam/src/applications/oauthproviders/entity"
	"github.com/a-digi/coco-iam/src/applications/oauthproviders/repository"
	"github.com/a-digi/coco-iam/src/auth/crypto/secretbox"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// adminView is the wire shape returned to the admin UI. Secrets
// are NEVER returned in plaintext — only the mask string. Admins
// rotate the secret by re-entering it in the edit flow.
type adminView struct {
	ID                string   `json:"id"`
	ApplicationID     string   `json:"application_id"`
	Provider          string   `json:"provider"`
	ClientID          string   `json:"client_id"`
	ClientSecretMask  string   `json:"client_secret_mask"`
	DiscoveryURL      string   `json:"discovery_url,omitempty"`
	AuthorizeURL      string   `json:"authorize_url,omitempty"`
	TokenURL          string   `json:"token_url,omitempty"`
	UserinfoURL       string   `json:"userinfo_url,omitempty"`
	Scopes            []string `json:"scopes"`
	AllowLogin        bool     `json:"allow_login"`
	AllowRegistration bool     `json:"allow_registration"`
	IsActive          bool     `json:"is_active"`
	CreatedAt         string   `json:"created_at"`
	UpdatedAt         string   `json:"updated_at"`
}

func toView(cfg entity.ProviderConfig) adminView {
	return adminView{
		ID:                cfg.ID,
		ApplicationID:     cfg.ApplicationID,
		Provider:          string(cfg.Provider),
		ClientID:          cfg.ClientID,
		ClientSecretMask:  secretbox.MaskSecret(),
		DiscoveryURL:      cfg.DiscoveryURL,
		AuthorizeURL:      cfg.AuthorizeURL,
		TokenURL:          cfg.TokenURL,
		UserinfoURL:       cfg.UserinfoURL,
		Scopes:            cfg.Scopes,
		AllowLogin:        cfg.AllowLogin,
		AllowRegistration: cfg.AllowRegistration,
		IsActive:          cfg.IsActive,
		CreatedAt:         cfg.CreatedAt,
		UpdatedAt:         cfg.UpdatedAt,
	}
}

// -- List ----------------------------------------------------------

// ListHandler serves GET /api/v1/applications/{id}/oauth-providers.
type ListHandler struct{}

type listResponse struct {
	Providers []adminView `json:"providers"`
}

func (h *ListHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	appID := appIDFromPath(reqCtx)
	if appID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "missing application id")
		return
	}
	repo, ok := openRepo(reqCtx)
	if !ok {
		return
	}
	rows, err := repo.ListForApp(appID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := listResponse{Providers: make([]adminView, 0, len(rows))}
	for _, r := range rows {
		out.Providers = append(out.Providers, toView(r))
	}
	response.SuccessResponse(w, http.StatusOK, out)
}

// -- Create --------------------------------------------------------

// CreateHandler serves POST /api/v1/applications/{id}/oauth-providers.
type CreateHandler struct{}

type createPayload struct {
	Provider          string   `json:"provider"`
	ClientID          string   `json:"client_id"`
	ClientSecret      string   `json:"client_secret"`
	DiscoveryURL      string   `json:"discovery_url"`
	AuthorizeURL      string   `json:"authorize_url"`
	TokenURL          string   `json:"token_url"`
	UserinfoURL       string   `json:"userinfo_url"`
	Scopes            []string `json:"scopes"`
	AllowLogin        bool     `json:"allow_login"`
	AllowRegistration bool     `json:"allow_registration"`
}

func (h *CreateHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	appID := appIDFromPath(reqCtx)
	if appID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "missing application id")
		return
	}
	var body createPayload
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	if err := validateBody(body.Provider, body.ClientID, body.ClientSecret); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	repo, ok := openRepo(reqCtx)
	if !ok {
		return
	}
	cfg, err := repo.Insert(repository.InsertInput{
		ApplicationID:     appID,
		Provider:          entity.Provider(body.Provider),
		ClientID:          strings.TrimSpace(body.ClientID),
		ClientSecret:      body.ClientSecret,
		DiscoveryURL:      strings.TrimSpace(body.DiscoveryURL),
		AuthorizeURL:      strings.TrimSpace(body.AuthorizeURL),
		TokenURL:          strings.TrimSpace(body.TokenURL),
		UserinfoURL:       strings.TrimSpace(body.UserinfoURL),
		Scopes:            trimScopes(body.Scopes),
		AllowLogin:        body.AllowLogin,
		AllowRegistration: body.AllowRegistration,
	})
	if err != nil {
		if errors.Is(err, entity.ErrDuplicateProvider) {
			response.ErrorResponse(w, http.StatusConflict, err.Error())
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusCreated, toView(cfg))
}

// -- Update --------------------------------------------------------

// UpdateHandler serves PATCH /api/v1/applications/{id}/oauth-providers/{providerId}.
type UpdateHandler struct{}

type updatePayload struct {
	ClientID          string   `json:"client_id"`
	ClientSecret      *string  `json:"client_secret,omitempty"`
	DiscoveryURL      string   `json:"discovery_url"`
	AuthorizeURL      string   `json:"authorize_url"`
	TokenURL          string   `json:"token_url"`
	UserinfoURL       string   `json:"userinfo_url"`
	Scopes            []string `json:"scopes"`
	AllowLogin        bool     `json:"allow_login"`
	AllowRegistration bool     `json:"allow_registration"`
	IsActive          bool     `json:"is_active"`
}

func (h *UpdateHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	appID := appIDFromPath(reqCtx)
	providerID := providerIDFromPath(r.URL.Path)
	if appID == "" || providerID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "missing id")
		return
	}
	var body updatePayload
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	if body.ClientSecret != nil && strings.TrimSpace(*body.ClientSecret) == "" {
		// A caller that omits the field entirely leaves the
		// secret untouched; an explicit empty string is an
		// error — admins shouldn't be able to blank the secret.
		response.ErrorResponse(w, http.StatusBadRequest, "client_secret may not be blank")
		return
	}
	if strings.TrimSpace(body.ClientID) == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "client_id is required")
		return
	}

	repo, ok := openRepo(reqCtx)
	if !ok {
		return
	}
	cfg, err := repo.Update(appID, providerID, repository.UpdateInput{
		ClientID:          strings.TrimSpace(body.ClientID),
		ClientSecret:      body.ClientSecret,
		DiscoveryURL:      strings.TrimSpace(body.DiscoveryURL),
		AuthorizeURL:      strings.TrimSpace(body.AuthorizeURL),
		TokenURL:          strings.TrimSpace(body.TokenURL),
		UserinfoURL:       strings.TrimSpace(body.UserinfoURL),
		Scopes:            trimScopes(body.Scopes),
		AllowLogin:        body.AllowLogin,
		AllowRegistration: body.AllowRegistration,
		IsActive:          body.IsActive,
	})
	if err != nil {
		if errors.Is(err, entity.ErrProviderNotFound) {
			response.ErrorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, toView(cfg))
}

// -- Delete --------------------------------------------------------

// DeleteHandler serves DELETE /api/v1/applications/{id}/oauth-providers/{providerId}.
type DeleteHandler struct{}

func (h *DeleteHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	appID := appIDFromPath(reqCtx)
	providerID := providerIDFromPath(r.URL.Path)
	if appID == "" || providerID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "missing id")
		return
	}
	repo, ok := openRepo(reqCtx)
	if !ok {
		return
	}
	if err := repo.Delete(appID, providerID); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, map[string]any{"ok": true})
}

// -- helpers --------------------------------------------------------

func validateBody(provider, clientID, clientSecret string) error {
	if !entity.IsAllowedProvider(provider) {
		return errors.New("unsupported provider")
	}
	if strings.TrimSpace(clientID) == "" {
		return errors.New("client_id is required")
	}
	if strings.TrimSpace(clientSecret) == "" {
		return errors.New("client_secret is required")
	}
	return nil
}

func trimScopes(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if v := strings.TrimSpace(s); v != "" {
			out = append(out, v)
		}
	}
	return out
}
