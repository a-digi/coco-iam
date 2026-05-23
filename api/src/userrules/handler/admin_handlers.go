package handler

import (
	"encoding/json"
	"net/http"

	"github.com/a-digi/coco-iam/src/userrules"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// AdminUserRulesGetHandler serves GET /api/v1/admin/settings/user-rules.
// Returns the admin-wide rule set, or defaults if unset.
type AdminUserRulesGetHandler struct{}

// @Summary     Get admin user rules
// @Tags        admin-settings
// @Produce     json
// @Security    BearerAuth
// @Router      /admin/settings/user-rules [get]
func (h *AdminUserRulesGetHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	store := resolveStore(reqCtx)
	if store == nil {
		return
	}
	rs, err := store.GetAdmin()
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, rs)
}

// AdminUserRulesUpdateHandler serves PATCH /api/v1/admin/settings/user-rules.
// The body is the full RuleSet; partial updates aren't supported (the
// form always ships the complete object).
type AdminUserRulesUpdateHandler struct{}

// @Summary     Update admin user rules
// @Tags        admin-settings
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Router      /admin/settings/user-rules [patch]
func (h *AdminUserRulesUpdateHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	store := resolveStore(reqCtx)
	if store == nil {
		return
	}

	var rs userrules.RuleSet
	if err := json.NewDecoder(r.Body).Decode(&rs); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	defer r.Body.Close()

	if err := store.UpsertAdmin(rs); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, rs)
}
