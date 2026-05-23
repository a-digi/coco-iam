package renew

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/a-digi/coco-iam/config"
	"github.com/a-digi/coco-iam/src/auth/oauth"
	oauth_lib "github.com/a-digi/coco-oauth/oauth"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

type TokenRenewHandler struct{}

type TokenRenewRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// @Summary     Renew OAuth token
// @Tags        auth
// @Produce     json
// @Success     200 {object} interface{}
// @Failure     400,401,403,500 {object} map[string]interface{}
// @Router      /admin/oauth/renew [post]
func (h *TokenRenewHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	var req TokenRenewRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(&req); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.RefreshToken == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "refresh_token is required")
		return
	}

	cfgBytes, err := config.ReadConfigFile("config.json")
	if err != nil {
		msg := "oauth error #1: " + err.Error()
		response.ErrorResponse(w, http.StatusInternalServerError, msg)
		return
	}

	cfg, err := oauth_lib.LoadAuthConfigFromBytes(cfgBytes)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "token validator error")
		return
	}

	validator, err := oauth_lib.NewValidatorFromConfig(cfg)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "token validator error")
		return
	}

	sub, scopes, expiry, err := validator.Validate(req.RefreshToken)
	if err != nil || sub == "" {
		response.ErrorResponse(w, http.StatusUnauthorized, "invalid refresh token: "+err.Error())
		return
	}

	if expiry.Add(15 * time.Minute).Before(time.Now()) {
		response.ErrorResponse(w, http.StatusUnauthorized, "refresh token expired")
		return
	}

	tokenResp, err := oauth.IssueToken(cfg, sub, scopes)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "token signing failed")
		return
	}

	response.SuccessResponse(w, http.StatusOK, tokenResp)
}
