// Package admin implements CustomApplicationsHandler which routes all CRUD
// for the applications entity to the per-org users DB.
package admin

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/a-digi/coco-iam/src/applications/keys"
	loginlog_dbregistry "github.com/a-digi/coco-iam/src/applications/loginlog/dbregistry"
	"github.com/a-digi/coco-lift/resource/uri"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
	"github.com/google/uuid"
)

// CustomApplicationsHandler routes all CRUD operations for the applications
// entity to the per-org DB (data/db/organization/{orgID}/users.db).
//
//	@Summary		Manage applications (CRUD)
//	@Description	POST creates a new application. GET returns one (by id) or a list (filtered by workspace_id). PATCH/PUT updates. DELETE soft-deletes (sets is_active=false).
//	@Tags			applications
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id				path		string							false	"Application ID"
//	@Param			workspace_id	query		string							false	"Workspace ID filter (list)"
//	@Param			body			body		entity.ApplicationRequest	false	"Application body (POST/PATCH/PUT)"
//	@Success		200		{object}	entity.ApplicationSuccess
//	@Success		201		{object}	entity.ApplicationSuccess
//	@Failure		400		{object}	response.ErrorBody
//	@Failure		404		{object}	response.ErrorBody
//	@Failure		500		{object}	response.ErrorBody
//	@Router			/applications/applications [post]
//	@Router			/applications/applications [get]
//	@Router			/applications/applications/{id} [get]
//	@Router			/applications/applications/{id} [patch]
//	@Router			/applications/applications/{id} [delete]
func CustomApplicationsHandler(reqCtx request.RequestContext) {
	r := reqCtx.GetRequest()
	switch r.Method {
	case http.MethodPost:
		applicationCreate(reqCtx)
	case http.MethodGet:
		applicationGet(reqCtx)
	case http.MethodPatch, http.MethodPut:
		applicationUpdate(reqCtx)
	case http.MethodDelete:
		applicationDelete(reqCtx)
	default:
		response.ErrorResponse(reqCtx.GetWriter(), http.StatusMethodNotAllowed, "method not allowed")
	}
}

type applicationBody struct {
	WorkspaceID        string `json:"workspace_id"`
	ClientID           string `json:"client_id"`
	Title              string `json:"title"`
	Description        string `json:"description"`
	AllowRecovery      *bool  `json:"allow_recovery"`
	AllowRegistration  *bool  `json:"allow_registration"`
	AllowPasswordLogin *bool  `json:"allow_password_login"`
	RegistrationType   string `json:"registration_type"`
	IsActive           *bool  `json:"is_active"`
}

type applicationRow struct {
	ID                 string `json:"id"`
	WorkspaceID        string `json:"workspace_id"`
	ClientID           string `json:"client_id"`
	Title              string `json:"title"`
	Description        string `json:"description"`
	CreatedAt          string `json:"created_at"`
	IsActive           bool   `json:"is_active"`
	AllowRecovery      bool   `json:"allow_recovery"`
	AllowRegistration  bool   `json:"allow_registration"`
	AllowPasswordLogin bool   `json:"allow_password_login"`
	RegistrationType   string `json:"registration_type"`
}

// --- POST ---------------------------------------------------------------

func applicationCreate(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	_, reg, ok := resolveAppAclDBs(reqCtx, w)
	if !ok {
		return
	}

	var body applicationBody
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if body.WorkspaceID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	if body.Title == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "title is required")
		return
	}
	if body.ClientID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "client_id is required")
		return
	}

	// Resolve org by scanning per-org DBs for the workspace.
	orgDB, orgID, err := wsOrgDB(reg, body.WorkspaceID)
	if err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "workspace not found: "+err.Error())
		return
	}

	id := uuid.New().String()
	isActive := true
	if body.IsActive != nil {
		isActive = *body.IsActive
	}
	allowRecovery := true
	if body.AllowRecovery != nil {
		allowRecovery = *body.AllowRecovery
	}
	allowReg := false
	if body.AllowRegistration != nil {
		allowReg = *body.AllowRegistration
	}
	allowPwdLogin := true
	if body.AllowPasswordLogin != nil {
		allowPwdLogin = *body.AllowPasswordLogin
	}
	regType := "legacy"
	if body.RegistrationType != "" {
		regType = body.RegistrationType
	}

	// Best-effort, like ensureKeypair below - a reservation failure
	// must never block application creation itself. See
	// reserveApplicationSlug's own doc comment.
	slug := reserveApplicationSlug(reqCtx, id, orgID, body.Title)
	var slugArg interface{}
	if slug != "" {
		slugArg = slug
	}

	if _, err := orgDB.Exec(
		`INSERT INTO applications
		   (id, workspace_id, client_id, title, description, is_active,
		    allow_recovery, allow_registration, allow_password_login, registration_type, slug)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, body.WorkspaceID, body.ClientID, body.Title, body.Description, isActive,
		allowRecovery, allowReg, allowPwdLogin, regType, slugArg,
	); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to create application: "+err.Error())
		return
	}

	// Ensure keypair (best-effort).
	ensureKeypair(reqCtx, id)

	// Provision the per-application login-log database (best-effort,
	// same convention) — only possible if a slug was actually
	// reserved above, since the file is named after it. See
	// plan/login-audit-log/plan.md Step 7.
	if slug != "" {
		provisionLoginLog(reqCtx, id, orgID, slug)
	}

	row, err := fetchApplicationRow(orgDB, id)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusCreated, row)
}

// --- GET ----------------------------------------------------------------

func applicationGet(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	_, reg, ok := resolveAppAclDBs(reqCtx, w)
	if !ok {
		return
	}

	_, appID := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if appID == "" {
		appID = reqCtx.GetURI().GetPathVariable("id")
	}

	wsID := extractWorkspaceIDParam(r)

	if appID != "" {
		orgDB, _, err := appOrgDB(reg, appID)
		if err != nil {
			response.ErrorResponse(w, http.StatusNotFound, "application not found")
			return
		}
		row, err := fetchApplicationRow(orgDB, appID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				response.ErrorResponse(w, http.StatusNotFound, "application not found")
				return
			}
			response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		response.SuccessResponse(w, http.StatusOK, row)
		return
	}

	if wsID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "workspace_id filter is required")
		return
	}

	// Resolve org by scanning per-org DBs for the workspace.
	orgDB, _, err := wsOrgDB(reg, wsID)
	if err != nil {
		response.ErrorResponse(w, http.StatusNotFound, "workspace not found")
		return
	}

	rows, err := orgDB.Query(
		`SELECT id, workspace_id, client_id, title, description, created_at, is_active,
		        allow_recovery, allow_registration, allow_password_login, registration_type
		 FROM applications
		 WHERE workspace_id = ? AND is_active = TRUE
		 ORDER BY created_at DESC`,
		wsID,
	)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to query applications: "+err.Error())
		return
	}
	defer rows.Close()

	out := []applicationRow{}
	for rows.Next() {
		row, err := scanApplicationRow(rows)
		if err != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		out = append(out, row)
	}
	response.SuccessResponse(w, http.StatusOK, out)
}

// --- PATCH / PUT --------------------------------------------------------

func applicationUpdate(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	_, reg, ok := resolveAppAclDBs(reqCtx, w)
	if !ok {
		return
	}

	_, appID := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if appID == "" {
		appID = reqCtx.GetURI().GetPathVariable("id")
	}
	if appID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "application id missing from path")
		return
	}

	var body applicationBody
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}

	orgDB, _, err := appOrgDB(reg, appID)
	if err != nil {
		response.ErrorResponse(w, http.StatusNotFound, "application not found")
		return
	}

	existing, err := fetchApplicationRow(orgDB, appID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.ErrorResponse(w, http.StatusNotFound, "application not found")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	newTitle := existing.Title
	newDesc := existing.Description
	newIsActive := existing.IsActive
	newAllowRecovery := existing.AllowRecovery
	newAllowReg := existing.AllowRegistration
	newAllowPwd := existing.AllowPasswordLogin
	newRegType := existing.RegistrationType

	if body.Title != "" {
		newTitle = body.Title
	}
	if body.Description != "" {
		newDesc = body.Description
	}
	if body.IsActive != nil {
		newIsActive = *body.IsActive
	}
	if body.AllowRecovery != nil {
		newAllowRecovery = *body.AllowRecovery
	}
	if body.AllowRegistration != nil {
		newAllowReg = *body.AllowRegistration
	}
	if body.AllowPasswordLogin != nil {
		newAllowPwd = *body.AllowPasswordLogin
	}
	if body.RegistrationType != "" {
		newRegType = body.RegistrationType
	}

	if _, err := orgDB.Exec(
		`UPDATE applications SET title = ?, description = ?, is_active = ?,
		        allow_recovery = ?, allow_registration = ?, allow_password_login = ?,
		        registration_type = ?
		 WHERE id = ?`,
		newTitle, newDesc, newIsActive, newAllowRecovery, newAllowReg, newAllowPwd, newRegType, appID,
	); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to update application: "+err.Error())
		return
	}

	row, err := fetchApplicationRow(orgDB, appID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, row)
}

// --- DELETE -------------------------------------------------------------

func applicationDelete(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()

	_, reg, ok := resolveAppAclDBs(reqCtx, w)
	if !ok {
		return
	}

	_, appID := uri.ExtractKeyAndValueFromURI(reqCtx.GetURI().GetPath())
	if appID == "" {
		appID = reqCtx.GetURI().GetPathVariable("id")
	}
	if appID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "application id missing from path")
		return
	}

	orgDB, _, err := appOrgDB(reg, appID)
	if err != nil {
		response.ErrorResponse(w, http.StatusNotFound, "application not found")
		return
	}

	existing, err := fetchApplicationRow(orgDB, appID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.ErrorResponse(w, http.StatusNotFound, "application not found")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	if _, err := orgDB.Exec(`UPDATE applications SET is_active = FALSE WHERE id = ?`, appID); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to delete application: "+err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, existing)
}

// --- internal helpers --------------------------------------------------

func extractWorkspaceIDParam(r *http.Request) string {
	q := r.URL.Query()
	if v := strings.TrimSpace(q.Get("filter[@exact:workspace_id]")); v != "" {
		return v
	}
	return strings.TrimSpace(q.Get("workspace_id"))
}

func fetchApplicationRow(db *sql.DB, id string) (applicationRow, error) {
	row := db.QueryRow(
		`SELECT id, workspace_id, client_id, title, description, created_at, is_active,
		        allow_recovery, allow_registration, allow_password_login, registration_type
		 FROM applications WHERE id = ? LIMIT 1`,
		id,
	)
	return scanApplicationRowFrom(row)
}

func scanApplicationRowFrom(row *sql.Row) (applicationRow, error) {
	var out applicationRow
	var createdAt sql.NullString
	if err := row.Scan(
		&out.ID, &out.WorkspaceID, &out.ClientID, &out.Title, &out.Description,
		&createdAt, &out.IsActive, &out.AllowRecovery, &out.AllowRegistration,
		&out.AllowPasswordLogin, &out.RegistrationType,
	); err != nil {
		return applicationRow{}, err
	}
	if createdAt.Valid {
		out.CreatedAt = createdAt.String
	}
	return out, nil
}

func scanApplicationRow(rows *sql.Rows) (applicationRow, error) {
	var out applicationRow
	var createdAt sql.NullString
	if err := rows.Scan(
		&out.ID, &out.WorkspaceID, &out.ClientID, &out.Title, &out.Description,
		&createdAt, &out.IsActive, &out.AllowRecovery, &out.AllowRegistration,
		&out.AllowPasswordLogin, &out.RegistrationType,
	); err != nil {
		return applicationRow{}, err
	}
	if createdAt.Valid {
		out.CreatedAt = createdAt.String
	}
	return out, nil
}

// slugPattern matches runs of characters not allowed in a generated
// slug - kept deliberately narrow (lowercase alphanumerics only)
// since a slug becomes a filesystem path component for a
// per-application login-log database file
// (organization/<orgID>/applications/<appID>/<slug>_login.db). See
// plan/login-audit-log/plan.md Step 5.
var slugPattern = regexp.MustCompile(`[^a-z0-9]+`)

// maxSlugCollisionAttempts bounds the "-2", "-3", ... retry loop in
// reserveApplicationSlug - a real title producing this many exact
// collisions is implausible; bailing out with a logged warning beats
// looping forever.
const maxSlugCollisionAttempts = 50

// deriveSlug lowercases title, collapses every run of non-alphanumeric
// characters into a single hyphen, and trims leading/trailing
// hyphens - the same kebab-case convention used for URL slugs
// elsewhere. Falls back to "app" if title has no alphanumeric
// characters at all, so a candidate is never empty.
func deriveSlug(title string) string {
	s := slugPattern.ReplaceAllString(strings.ToLower(title), "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "app"
	}
	return s
}

// reserveApplicationSlug reserves a globally-unique, immutable slug
// for a new application in the main DB's application_slugs table -
// the only place cross-org uniqueness can actually be enforced, since
// applications themselves live in each organization's own users.db.
// Starts from deriveSlug(title) and appends "-2", "-3", ... on a
// collision. Best-effort, like ensureKeypair below: a reservation
// failure (e.g. the main DB briefly unavailable) must never block
// application creation itself, since the slug only matters for a
// downstream, non-essential feature (per-application login logging).
// Returns "" on any failure, after logging a warning - the
// application simply won't get login-log provisioning until an
// operator investigates. See plan/login-audit-log/plan.md Step 5.
func reserveApplicationSlug(reqCtx request.RequestContext, applicationID, organizationID, title string) string {
	log := reqCtx.GetDI().GetLogger()
	manager := reqCtx.GetDI().GetDatabaseManager()
	if manager == nil || manager.Connector == nil || manager.Connector.DB == nil {
		if log != nil {
			log.Warning("application slug: main database not available for %s", applicationID)
		}
		return ""
	}

	base := deriveSlug(title)
	candidate := base
	for attempt := 1; attempt <= maxSlugCollisionAttempts; attempt++ {
		if attempt > 1 {
			candidate = fmt.Sprintf("%s-%d", base, attempt)
		}
		_, err := manager.Connector.DB.Exec(
			`INSERT INTO application_slugs (slug, application_id, organization_id, created_at) VALUES (?, ?, ?, ?)`,
			candidate, applicationID, organizationID, time.Now().UTC().Format("2006-01-02 15:04:05"),
		)
		if err == nil {
			return candidate
		}
		if !strings.Contains(err.Error(), "UNIQUE constraint failed") {
			if log != nil {
				log.Warning("application slug: reserve for %s: %v", applicationID, err)
			}
			return ""
		}
	}
	if log != nil {
		log.Warning("application slug: could not reserve a unique slug for %s after %d attempts", applicationID, maxSlugCollisionAttempts)
	}
	return ""
}

// provisionLoginLog creates and migrates applicationID's per-application
// login-log database and starts its archiver, best-effort — like
// ensureKeypair, a provisioning failure here must never block
// application creation itself. Resolves the registry via the same
// Get(string) (interface{}, bool) duck-typed lookup ensureKeypair
// uses below, rather than importing config/di directly — this
// package is itself imported by config/resource (for
// CustomApplicationsHandler's registration), which config/di in turn
// imports, so importing config/di back here would be a cycle. See
// plan/login-audit-log/plan.md Step 7.
func provisionLoginLog(reqCtx request.RequestContext, applicationID, organizationID, slug string) {
	log := reqCtx.GetDI().GetLogger()
	ctx := reqCtx.GetDI()
	bag, ok := ctx.(interface {
		Get(string) (interface{}, bool)
	})
	if !ok {
		return
	}
	raw, ok := bag.Get(loginlog_dbregistry.ContextBagKey)
	if !ok {
		return
	}
	registry, ok := raw.(*loginlog_dbregistry.Registry)
	if !ok || registry == nil {
		return
	}
	if err := registry.Provision(applicationID, organizationID, slug); err != nil {
		if log != nil {
			log.Warning("application login-log: provision for %s: %v", applicationID, err)
		}
	}
}

func ensureKeypair(reqCtx request.RequestContext, appID string) {
	log := reqCtx.GetDI().GetLogger()
	ctx := reqCtx.GetDI()
	bag, ok := ctx.(interface {
		Get(string) (interface{}, bool)
	})
	if !ok {
		return
	}
	raw, ok := bag.Get(keys.ContextBagKeyService)
	if !ok {
		return
	}
	svc, ok := raw.(*keys.Service)
	if !ok || svc == nil {
		return
	}
	if err := svc.EnsureActive(appID); err != nil {
		if log != nil {
			log.Warning("app keys: generate keypair for %s: %v", appID, err)
		}
	}
}
