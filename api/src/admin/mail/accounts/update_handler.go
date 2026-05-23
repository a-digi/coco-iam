package accounts

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/a-digi/coco-lift/resource/uri"
	mailaccounts "github.com/a-digi/coco-iam/src/mail/accounts"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// AdminMailAccountsUpdateHandler serves PATCH /api/v1/admin/mail/accounts/{id}.
// Name is immutable; the active flag is not patchable here — use the
// dedicated /activate endpoint.
type AdminMailAccountsUpdateHandler struct{}

type updateRequest struct {
	Host      *string `json:"host,omitempty"`
	Port      *int    `json:"port,omitempty"`
	Username  *string `json:"username,omitempty"`
	Password  *string `json:"password,omitempty"`
	FromName  *string `json:"from_name,omitempty"`
	FromEmail *string `json:"from_email,omitempty"`
	UseTLS    *bool   `json:"use_tls,omitempty"`
	// Accepted-but-ignored unless it matches the stored name — prevents
	// silent renames via a PATCH.
	Name *string `json:"name,omitempty"`
}

// @Summary     Update a mail account
// @Tags        admin-mail
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Account ID"
// @Param       body body interface{} true "Request body"
// @Success     200 {object} interface{}
// @Failure     400,401,403,500 {object} map[string]interface{}
// @Router      /admin/mail/accounts/{id} [patch]
func (h *AdminMailAccountsUpdateHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	key, value := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if key != "id" || value == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "account id is required")
		return
	}

	var req updateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	defer r.Body.Close()

	store := resolveStore(reqCtx)
	if store == nil {
		return
	}

	if req.Name != nil {
		existing, err := store.Get(value)
		if err != nil {
			if errors.Is(err, mailaccounts.ErrNotFound) {
				response.ErrorResponse(w, http.StatusNotFound, "account not found")
				return
			}
			response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		if *req.Name != existing.Name {
			response.ErrorResponse(w, http.StatusBadRequest, "account name is immutable")
			return
		}
	}

	if req.Port != nil && (*req.Port < 1 || *req.Port > 65535) {
		response.ErrorResponse(w, http.StatusBadRequest, "port must be between 1 and 65535")
		return
	}
	if req.FromEmail != nil {
		if err := validateEmail(*req.FromEmail); err != nil {
			response.ErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	patch := mailaccounts.Patch{
		Host:      req.Host,
		Port:      req.Port,
		Username:  req.Username,
		Password:  req.Password,
		FromName:  req.FromName,
		FromEmail: req.FromEmail,
		UseTLS:    req.UseTLS,
	}
	updated, err := store.Update(value, patch)
	if err != nil {
		if errors.Is(err, mailaccounts.ErrNotFound) {
			response.ErrorResponse(w, http.StatusNotFound, "account not found")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	redacted := updated.Redacted()
	response.SuccessResponse(w, http.StatusOK, redacted)
}
