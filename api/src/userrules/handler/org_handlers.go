package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	uri "github.com/a-digi/coco-lift/resource/uri"
	"github.com/a-digi/coco-iam/src/userrules"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// OrgUserRulesGetHandler serves GET /api/v1/organizations/{id}/user-rules.
type OrgUserRulesGetHandler struct{}

func (h *OrgUserRulesGetHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	store := resolveStore(reqCtx)
	if store == nil {
		return
	}
	orgID := orgIDFromPath(reqCtx)
	if orgID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "missing organization id")
		return
	}
	rs, err := store.GetForOrg(orgID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, rs)
}

// OrgUserRulesUpdateHandler serves PATCH /api/v1/organizations/{id}/user-rules.
type OrgUserRulesUpdateHandler struct{}

func (h *OrgUserRulesUpdateHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	store := resolveStore(reqCtx)
	if store == nil {
		return
	}
	orgID := orgIDFromPath(reqCtx)
	if orgID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "missing organization id")
		return
	}

	var rs userrules.RuleSet
	if err := json.NewDecoder(r.Body).Decode(&rs); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	defer r.Body.Close()

	if err := store.UpsertForOrg(orgID, rs); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, rs)
}

// AccountUserRulesHandler serves GET /api/v1/account/user-rules. It
// resolves the rule set that applies to the authenticated caller —
// admin rules for admins, the user's org rules for everyone else.
type AccountUserRulesHandler struct{}

func (h *AccountUserRulesHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	store := resolveStore(reqCtx)
	if store == nil {
		return
	}
	userID := userIDFromRequest(reqCtx)
	if userID == "" {
		return
	}
	rs, err := store.GetForUser(userID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, rs)
}

// orgIDFromPath extracts the organization id from the request URL.
// The URL carries an `{id:<value>}` segment (the same scheme the rest
// of the resource API uses); we parse the `<value>` out via the
// shared URI helper instead of reading the raw `GetPathVariable("id")`
// which returns the literal `{id:<value>}` string.
func orgIDFromPath(reqCtx request.RequestContext) string {
	key, value := uri.ExtractKeyAndValueFromURI(reqCtx.GetRequest().URL.Path)
	if key != "id" {
		return ""
	}
	return strings.TrimSpace(value)
}
