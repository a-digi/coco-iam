package acl

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/a-digi/coco-iam/config"

	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

type AclScopesHandler struct{}

// @Summary     List ACL scopes
// @Tags        admin-acl
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} interface{}
// @Failure     400,401,403,500 {object} map[string]interface{}
// @Router      /admin/acl/scopes [get]
func (h *AclScopesHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()

	entries, err := config.ConfigFS.ReadDir("scopes")
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to list scopes directory")
		return
	}

	var combined []interface{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		bytes, err := config.ReadConfigFile("scopes/" + entry.Name())
		if err != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, "failed to read scopes file: "+entry.Name())
			return
		}

		var parsed []interface{}
		if err := json.Unmarshal(bytes, &parsed); err != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, "failed to parse scopes file: "+entry.Name())
			return
		}

		combined = append(combined, parsed...)
	}

	response.SuccessResponse(w, http.StatusOK, combined)
}
