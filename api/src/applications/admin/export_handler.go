// Package admin hosts custom HTTP handlers for application-level operations
// that don't fit the generic ApiResourceHandler — currently scope export and
// import.
package admin

import (
	"net/http"
	"sort"
	"strings"

	"github.com/a-digi/coco-lift/resource/uri"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// ExportedScope is the same on-wire shape used by `scopes/*.json` catalogs.
// `id` is the scope identifier (e.g. "docs:read"), `scopes` is the optional
// child list built by grouping ids on their `:` delimiters.
type ExportedScope struct {
	ID          string           `json:"id"`
	Description string           `json:"description"`
	Scopes      []*ExportedScope `json:"scopes,omitempty"`
}

// flatScope is the flat DB-row representation used between the handler and
// buildScopeTree.
type flatScope struct {
	id   string
	desc string
}

// ApplicationScopesExportHandler serves
// `GET /api/v1/applications/{res:applications}/{id}/export` and returns the
// application's scope tree in the same JSON format as `admin.json`.
type ApplicationScopesExportHandler struct{}

// @Summary     Export application scopes
// @Tags        applications
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Application ID"
// @Router      /applications/applications/{id}/scopes/export [get]
func (h *ApplicationScopesExportHandler) ServeHTTP(reqCtx request.RequestContext) {
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

	orgDB, _, err := appOrgDB(reg, appID)
	if err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	rows, err := orgDB.Query(
		`SELECT scope_id, description FROM application_scopes WHERE application_id = ? AND is_active = TRUE ORDER BY scope_id ASC`,
		appID,
	)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to load scopes: "+err.Error())
		return
	}
	defer rows.Close()

	var flat []flatScope
	for rows.Next() {
		var id, desc string
		if err := rows.Scan(&id, &desc); err != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, "failed to read scope row: "+err.Error())
			return
		}
		flat = append(flat, flatScope{id: id, desc: desc})
	}

	tree := buildScopeTree(flat)
	response.SuccessResponse(w, http.StatusOK, tree)
}

// buildScopeTree groups a flat list of scope ids into a hierarchical tree by
// their `:` delimiters, matching the shape of `admin.json`. A scope with id
// `a:b:c` becomes a child of `a:b`, which becomes a child of `a`. Missing
// intermediate nodes are synthesised with an empty description.
func buildScopeTree(flat []flatScope) []*ExportedScope {
	byID := map[string]*ExportedScope{}
	for _, f := range flat {
		byID[f.id] = &ExportedScope{ID: f.id, Description: f.desc}
	}

	// Synthesise missing parent nodes (e.g. `a:b:c` requires `a:b` and `a`).
	for _, f := range flat {
		segments := strings.Split(f.id, ":")
		for i := 1; i < len(segments); i++ {
			parent := strings.Join(segments[:i], ":")
			if _, ok := byID[parent]; !ok {
				byID[parent] = &ExportedScope{ID: parent, Description: ""}
			}
		}
	}

	// Order ids by depth then lexicographically so parents attach before children.
	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(a, b int) bool {
		da := strings.Count(ids[a], ":")
		db := strings.Count(ids[b], ":")
		if da != db {
			return da < db
		}
		return ids[a] < ids[b]
	})

	var roots []*ExportedScope
	for _, id := range ids {
		node := byID[id]
		segments := strings.Split(id, ":")
		if len(segments) == 1 {
			roots = append(roots, node)
			continue
		}
		parentID := strings.Join(segments[:len(segments)-1], ":")
		if parent, ok := byID[parentID]; ok {
			parent.Scopes = append(parent.Scopes, node)
		} else {
			roots = append(roots, node)
		}
	}
	return roots
}
