package accounts

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	mailaccounts "github.com/a-digi/coco-notification/mailer"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// AdminMailAccountsCreateHandler serves POST /api/v1/admin/mail/accounts.
type AdminMailAccountsCreateHandler struct{}

type createRequest struct {
	Name      string `json:"name"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	FromName  string `json:"from_name"`
	FromEmail string `json:"from_email"`
	UseTLS    bool   `json:"use_tls"`
	IsActive  bool   `json:"is_active"`
}

// @Summary     Create a mail account
// @Tags        admin-mail
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       body body interface{} true "Request body"
// @Success     200 {object} interface{}
// @Failure     400,401,403,500 {object} map[string]interface{}
// @Router      /admin/mail/accounts [post]
func (h *AdminMailAccountsCreateHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	defer r.Body.Close()

	req.Name = strings.TrimSpace(req.Name)
	req.Host = strings.TrimSpace(req.Host)
	req.FromEmail = strings.TrimSpace(req.FromEmail)

	if req.Name == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "name is required")
		return
	}
	if !mailaccounts.NameFormat.MatchString(req.Name) {
		response.ErrorResponse(w, http.StatusBadRequest,
			"name must match "+mailaccounts.NameFormat.String()+" (lowercase letters, digits, _, -, start with a letter)")
		return
	}
	if req.Host == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "host is required")
		return
	}
	if req.Port < 1 || req.Port > 65535 {
		response.ErrorResponse(w, http.StatusBadRequest, "port must be between 1 and 65535")
		return
	}
	if err := validateEmail(req.FromEmail); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	store := resolveStore(reqCtx)
	if store == nil {
		return
	}

	created, err := store.Create(mailaccounts.Account{
		Name:      req.Name,
		Host:      req.Host,
		Port:      req.Port,
		Username:  req.Username,
		Password:  req.Password,
		FromName:  req.FromName,
		FromEmail: req.FromEmail,
		UseTLS:    req.UseTLS,
		IsActive:  req.IsActive,
	})
	if err != nil {
		if errors.Is(err, mailaccounts.ErrDuplicateName) {
			response.ErrorResponse(w, http.StatusConflict, "an account with that name already exists")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	redacted := created.Redacted()
	response.SuccessResponse(w, http.StatusCreated, redacted)
}

func validateEmail(email string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return nil
	}
	if !strings.Contains(email, "@") || strings.HasPrefix(email, "@") || strings.HasSuffix(email, "@") {
		return fmt.Errorf("from_email %q is not a valid email address", email)
	}
	return nil
}
