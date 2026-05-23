package admin

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/a-digi/coco-lift/resource/uri"
	"github.com/a-digi/coco-iam/src/applications/acl"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// ApplicationScopesImportHandler serves
// `POST /api/v1/applications/{res:applications}/{id}/import` and accepts a
// scope tree in the same format the export handler returns. Scopes are
// upserted: existing rows keyed on `(application_id, scope_id)` have their
// description updated; new rows are inserted.
type ApplicationScopesImportHandler struct{}

type importedResult struct {
	Inserted int `json:"inserted"`
	Updated  int `json:"updated"`
	Skipped  int `json:"skipped"`
	Total    int `json:"total"`
}

// @Summary     Import application scopes
// @Tags        applications
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Application ID"
// @Router      /applications/applications/{id}/scopes/import [post]
func (h *ApplicationScopesImportHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	_, appID := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if appID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "application id is required")
		return
	}

	_, reg, ok := resolveAppAclDBs(reqCtx, w)
	if !ok {
		return
	}

	var tree []ExportedScope
	if err := json.NewDecoder(r.Body).Decode(&tree); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	defer r.Body.Close()

	// Verify the application exists by resolving its per-org DB.
	orgDB, _, err := appOrgDB(reg, appID)
	if err != nil {
		response.ErrorResponse(w, http.StatusNotFound, fmt.Sprintf("application %q not found", appID))
		return
	}

	flat := flattenScopeTree(tree)

	// Validate every id up-front so we either import everything or nothing.
	for _, f := range flat {
		if !acl.ScopeIDFormat.MatchString(f.ID) {
			response.ErrorResponse(w, http.StatusBadRequest,
				fmt.Sprintf("scope_id %q is invalid — only letters, underscores and colon separators are allowed", f.ID))
			return
		}
	}

	var result importedResult
	result.Total = len(flat)

	for _, f := range flat {
		var existingID string
		err := orgDB.QueryRow(
			`SELECT id FROM application_scopes WHERE application_id = ? AND scope_id = ?`,
			appID, f.ID,
		).Scan(&existingID)
		switch {
		case err == nil:
			// Update description only — keep the existing row's UUID and timestamps.
			if _, err := orgDB.Exec(
				`UPDATE application_scopes SET description = ?, is_active = TRUE WHERE id = ?`,
				f.Description, existingID,
			); err != nil {
				response.ErrorResponse(w, http.StatusInternalServerError,
					fmt.Sprintf("failed to update scope %q: %s", f.ID, err.Error()))
				return
			}
			result.Updated++
		default:
			newID, uuidErr := newUUID()
			if uuidErr != nil {
				response.ErrorResponse(w, http.StatusInternalServerError, "failed to generate id: "+uuidErr.Error())
				return
			}
			if _, err := orgDB.Exec(
				`INSERT INTO application_scopes (id, application_id, scope_id, description, is_active) VALUES (?, ?, ?, ?, TRUE)`,
				newID, appID, f.ID, f.Description,
			); err != nil {
				response.ErrorResponse(w, http.StatusInternalServerError,
					fmt.Sprintf("failed to insert scope %q: %s", f.ID, err.Error()))
				return
			}
			result.Inserted++
		}
	}

	response.SuccessResponse(w, http.StatusOK, result)
}

// flattenScopeTree walks the tree and returns every node with a non-empty
// `id`, preserving the order a simple pre-order traversal produces.
func flattenScopeTree(tree []ExportedScope) []ExportedScope {
	var out []ExportedScope
	var walk func(nodes []ExportedScope)
	walk = func(nodes []ExportedScope) {
		for _, n := range nodes {
			if n.ID != "" {
				out = append(out, ExportedScope{ID: n.ID, Description: n.Description})
			}
			if len(n.Scopes) > 0 {
				// n.Scopes is a slice of *ExportedScope in the exported shape
				// but the JSON decoder will unmarshal into []*ExportedScope
				// only if the target is a pointer slice. Here we decode into
				// values with Scopes as []*ExportedScope.
				var children []ExportedScope
				for _, c := range n.Scopes {
					if c != nil {
						children = append(children, *c)
					}
				}
				walk(children)
			}
		}
	}
	walk(tree)
	return out
}

func newUUID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	hx := hex.EncodeToString(buf)
	return fmt.Sprintf("%s-%s-%s-%s-%s", hx[:8], hx[8:12], hx[12:16], hx[16:20], hx[20:32]), nil
}
