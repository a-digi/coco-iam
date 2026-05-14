package admin

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/a-digi/coco-iam/src/oauthserver/entity"
	"github.com/a-digi/coco-iam/src/oauthserver/sqlstore"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
	"github.com/google/uuid"
)

// clientView is the wire shape returned to the admin UI. The
// secret NEVER appears here — the create handler returns it
// once alongside this view in a one-time envelope, then
// future reads mask it.
type clientView struct {
	ID                string   `json:"id"`
	ApplicationID     string   `json:"application_id"`
	ClientID          string   `json:"client_id"`
	ClientSecretMask  string   `json:"client_secret_mask"`
	Type              string   `json:"client_type"`
	DisplayName       string   `json:"display_name"`
	RedirectURIs      []string `json:"redirect_uris"`
	AllowedScopes     []string `json:"allowed_scopes"`
	RequireConsent    bool     `json:"require_consent"`
	AccessTokenTTL    int      `json:"access_token_ttl"`
	RefreshTokenTTL   int      `json:"refresh_token_ttl"`
	IsActive          bool     `json:"is_active"`
	CreatedAt         string   `json:"created_at"`
	UpdatedAt         string   `json:"updated_at"`
}

// secretMask is the placeholder returned to the admin UI in
// place of the plaintext secret. Admins trigger a rotation to
// receive a fresh plaintext once.
const secretMask = "••••••••"

func toView(c entity.Client) clientView {
	return clientView{
		ID:               c.ID,
		ApplicationID:    c.ApplicationID,
		ClientID:         c.ClientID,
		ClientSecretMask: secretMask,
		Type:             string(c.Type),
		DisplayName:      c.DisplayName,
		RedirectURIs:     c.RedirectURIs,
		AllowedScopes:    c.AllowedScopes,
		RequireConsent:   c.RequireConsent,
		AccessTokenTTL:   c.AccessTokenTTL,
		RefreshTokenTTL:  c.RefreshTokenTTL,
		IsActive:         c.IsActive,
		CreatedAt:        c.CreatedAt,
		UpdatedAt:        c.UpdatedAt,
	}
}

// -- List ----------------------------------------------------------

type ListHandler struct{}

type listResponse struct {
	Clients []clientView `json:"clients"`
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
	rows, err := repo.ListForApp(context.Background(), appID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := listResponse{Clients: make([]clientView, 0, len(rows))}
	for _, r := range rows {
		out.Clients = append(out.Clients, toView(r))
	}
	response.SuccessResponse(w, http.StatusOK, out)
}

// -- Create --------------------------------------------------------

type CreateHandler struct{}

type createPayload struct {
	ClientID        string   `json:"client_id"`
	ClientType      string   `json:"client_type"`
	DisplayName     string   `json:"display_name"`
	RedirectURIs    []string `json:"redirect_uris"`
	AllowedScopes   []string `json:"allowed_scopes"`
	RequireConsent  bool     `json:"require_consent"`
	AccessTokenTTL  int      `json:"access_token_ttl"`
	RefreshTokenTTL int      `json:"refresh_token_ttl"`
	IsActive        bool     `json:"is_active"`
}

// createResponse wraps the one-time plaintext secret alongside
// the persisted row view. Admins copy `client_secret` here;
// future GETs only return `client_secret_mask`.
type createResponse struct {
	Client       clientView `json:"client"`
	ClientSecret string     `json:"client_secret,omitempty"`
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
	if strings.TrimSpace(body.ClientID) == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "client_id is required")
		return
	}
	clientType := entity.ClientType(strings.TrimSpace(body.ClientType))
	if clientType != entity.ClientTypePublic && clientType != entity.ClientTypeConfidential {
		response.ErrorResponse(w, http.StatusBadRequest, "client_type must be 'public' or 'confidential'")
		return
	}
	if len(body.RedirectURIs) == 0 {
		response.ErrorResponse(w, http.StatusBadRequest, "at least one redirect_uri is required")
		return
	}

	// Generate the plaintext secret server-side; admin copies it
	// once. Confidential clients only.
	var plaintext string
	if clientType == entity.ClientTypeConfidential {
		plaintext = randomSecret()
	}

	repo, ok := openRepo(reqCtx)
	if !ok {
		return
	}
	got, err := repo.Insert(context.Background(), uuid.New().String(), sqlstore.InsertInput{
		ApplicationID:   appID,
		ClientID:        strings.TrimSpace(body.ClientID),
		ClientSecret:    plaintext,
		Type:            clientType,
		DisplayName:     strings.TrimSpace(body.DisplayName),
		RedirectURIs:    trimURIs(body.RedirectURIs),
		AllowedScopes:   trimScopes(body.AllowedScopes),
		RequireConsent:  body.RequireConsent,
		AccessTokenTTL:  nonZero(body.AccessTokenTTL, 3600),
		RefreshTokenTTL: nonZero(body.RefreshTokenTTL, 1209600),
		IsActive:        body.IsActive,
	})
	if err != nil {
		if errors.Is(err, entity.ErrDuplicateClient) {
			response.ErrorResponse(w, http.StatusConflict, "client_id already registered for this application")
			return
		}
		var oe *entity.OAuthError
		if errors.As(err, &oe) {
			response.ErrorResponse(w, oe.Status, oe.Description)
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusCreated, createResponse{
		Client:       toView(*got),
		ClientSecret: plaintext,
	})
}

// -- Update --------------------------------------------------------

type UpdateHandler struct{}

type updatePayload struct {
	ClientSecret    *string  `json:"client_secret,omitempty"`
	DisplayName     string   `json:"display_name"`
	RedirectURIs    []string `json:"redirect_uris"`
	AllowedScopes   []string `json:"allowed_scopes"`
	RequireConsent  bool     `json:"require_consent"`
	AccessTokenTTL  int      `json:"access_token_ttl"`
	RefreshTokenTTL int      `json:"refresh_token_ttl"`
	IsActive        bool     `json:"is_active"`
}

// updateResponse reveals a rotated secret only when one was
// actually generated this call.
type updateResponse struct {
	Client       clientView `json:"client"`
	ClientSecret string     `json:"client_secret,omitempty"`
}

// RotateSecretRequest is the empty body admins POST to the
// rotate sub-endpoint. Kept as a named type so the wire shape
// is explicit even when empty.
type RotateSecretRequest struct{}

func (h *UpdateHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	appID := appIDFromPath(reqCtx)
	rowID := clientIDFromPath(r.URL.Path)
	if appID == "" || rowID == "" {
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
	if len(body.RedirectURIs) == 0 {
		response.ErrorResponse(w, http.StatusBadRequest, "at least one redirect_uri is required")
		return
	}
	repo, ok := openRepo(reqCtx)
	if !ok {
		return
	}
	got, err := repo.Update(context.Background(), appID, rowID, sqlstore.UpdateInput{
		ClientSecret:    body.ClientSecret,
		DisplayName:     strings.TrimSpace(body.DisplayName),
		RedirectURIs:    trimURIs(body.RedirectURIs),
		AllowedScopes:   trimScopes(body.AllowedScopes),
		RequireConsent:  body.RequireConsent,
		AccessTokenTTL:  nonZero(body.AccessTokenTTL, 3600),
		RefreshTokenTTL: nonZero(body.RefreshTokenTTL, 1209600),
		IsActive:        body.IsActive,
	})
	if err != nil {
		if errors.Is(err, entity.ErrClientNotFound) {
			response.ErrorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		var oe *entity.OAuthError
		if errors.As(err, &oe) {
			response.ErrorResponse(w, oe.Status, oe.Description)
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp := updateResponse{Client: toView(*got)}
	if body.ClientSecret != nil {
		resp.ClientSecret = *body.ClientSecret
	}
	response.SuccessResponse(w, http.StatusOK, resp)
}

// RotateHandler serves POST .../oauth-clients/{rowId}/rotate-secret.
// Mints a fresh plaintext server-side and stores its hash,
// returns the plaintext exactly once. Fails for public clients.
type RotateHandler struct{}

func (h *RotateHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	appID := appIDFromPath(reqCtx)
	rowID := clientIDFromPathWithSuffix(r.URL.Path, "rotate-secret")
	if appID == "" || rowID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "missing id")
		return
	}
	repo, ok := openRepo(reqCtx)
	if !ok {
		return
	}
	existing, err := repo.FindByID(context.Background(), appID, rowID)
	if err != nil {
		if errors.Is(err, entity.ErrClientNotFound) {
			response.ErrorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if existing.Type == entity.ClientTypePublic {
		response.ErrorResponse(w, http.StatusBadRequest, "public clients have no secret to rotate")
		return
	}
	plaintext := randomSecret()
	got, err := repo.Update(context.Background(), appID, rowID, sqlstore.UpdateInput{
		ClientSecret:    &plaintext,
		DisplayName:     existing.DisplayName,
		RedirectURIs:    existing.RedirectURIs,
		AllowedScopes:   existing.AllowedScopes,
		RequireConsent:  existing.RequireConsent,
		AccessTokenTTL:  existing.AccessTokenTTL,
		RefreshTokenTTL: existing.RefreshTokenTTL,
		IsActive:        existing.IsActive,
	})
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, updateResponse{
		Client:       toView(*got),
		ClientSecret: plaintext,
	})
}

// -- Delete --------------------------------------------------------

type DeleteHandler struct{}

func (h *DeleteHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	appID := appIDFromPath(reqCtx)
	rowID := clientIDFromPath(r.URL.Path)
	if appID == "" || rowID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "missing id")
		return
	}
	repo, ok := openRepo(reqCtx)
	if !ok {
		return
	}
	if err := repo.Delete(context.Background(), appID, rowID); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, map[string]any{"ok": true})
}

// -- helpers --------------------------------------------------------

// clientIDFromPathWithSuffix pulls the `{rowId}` segment from
//
//	.../oauth-clients/<rowId>/<suffix>
//
// when the rotate-secret (or future action) endpoints are hit.
func clientIDFromPathWithSuffix(path, suffix string) string {
	segs := strings.Split(strings.Trim(path, "/"), "/")
	for i := 0; i+2 < len(segs); i++ {
		if segs[i] == "oauth-clients" && segs[i+2] == suffix {
			return strings.TrimSpace(segs[i+1])
		}
	}
	return ""
}

// randomSecret mints a 32-byte base64url secret for confidential
// clients. Opaque to the library — clients just echo whatever we
// gave them.
func randomSecret() string {
	buf := make([]byte, 32)
	_, _ = rand.Read(buf)
	return base64.RawURLEncoding.EncodeToString(buf)
}

func trimURIs(in []string) []string {
	out := make([]string, 0, len(in))
	for _, u := range in {
		if v := strings.TrimSpace(u); v != "" {
			out = append(out, v)
		}
	}
	return out
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

func nonZero(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
}

// Compile-time pin: RotateSecretRequest must remain trivially
// JSON-decodable (empty bodies accepted by clients that still
// send `{}`).
var _ = RotateSecretRequest{}
