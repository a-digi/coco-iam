// Package submit hosts the public POST endpoint that processes a
// registration form submission for a workspace application:
//
//	POST /a/{orgSlug}/{wsSlug}/{appSlug}/register
//
// No authentication — form values are collected from an unauthenticated
// visitor. The handler either creates a new organisation user (sending an
// activation email) or maps an existing user to the application ACL
// (sending a notification email). Both paths return HTTP 202 with the
// same body to prevent user enumeration.
package submit

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/a-digi/coco-iam/src/activation"
	"github.com/a-digi/coco-iam/src/applications/loginpage"
	regfields_entity "github.com/a-digi/coco-iam/src/applications/registrationfields/entity"
	regfields_repo "github.com/a-digi/coco-iam/src/applications/registrationfields/repository"
	"github.com/a-digi/coco-iam/src/general"
	iam_notification "github.com/a-digi/coco-iam/src/notification"
	profile_dbregistry "github.com/a-digi/coco-iam/src/organizations/profile/dbregistry"
	users_dbregistry "github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-iam/src/orgrouter"
	"github.com/a-digi/coco-iam/src/userrules"
	coconotification "github.com/a-digi/coco-notification"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

const genericNotFound = "not found"

// RegisterHandler serves POST /a/{orgSlug}/{wsSlug}/{appSlug}/register.
type RegisterHandler struct{}

// @Summary     Submit application registration
// @Tags        app-public
// @Accept      json
// @Produce     json
// @Param       body  body  regfields_entity.RegisterRequest  true  "Registration field values"
// @Success     202   {object}  regfields_entity.RegisterSuccess
// @Failure     400   {object}  response.ErrorBody
// @Failure     404   {object}  response.ErrorBody
// @Failure     500   {object}  response.ErrorBody
// @Router      /a/{orgSlug}/{wsSlug}/{appSlug}/register [post]
func (h *RegisterHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	ctx := reqCtx.GetDI()

	orgSlug, wsSlug, appSlug, ok := parseSlugSegments(r.URL.Path)
	if !ok {
		response.ErrorResponse(w, http.StatusNotFound, genericNotFound)
		return
	}

	loginSvc := resolveLoginPageService(ctx)
	if loginSvc == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "service not available")
		return
	}

	info, err := loginSvc.Store.FindBySlugs(orgSlug, wsSlug, appSlug)
	if err != nil {
		response.ErrorResponse(w, http.StatusNotFound, genericNotFound)
		return
	}

	allow, _, err := loadRegistrationConfig(reqCtx, info.ID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to read registration config")
		return
	}
	if !allow {
		response.ErrorResponse(w, http.StatusNotFound, genericNotFound)
		return
	}

	var body regfields_entity.RegisterRequest
	if err := reqCtx.BindJSON(&body); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if body.Fields == nil {
		body.Fields = map[string]string{}
	}

	email := strings.TrimSpace(body.Fields["email"])
	username := strings.TrimSpace(body.Fields["username"])
	if email == "" || username == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "email and username are required")
		return
	}
	if _, err := mail.ParseAddress(email); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid email address")
		return
	}

	if rsStore := resolveUserRulesStore(ctx); rsStore != nil {
		rs, _ := rsStore.GetForOrg(info.OrganizationID)
		if violations := userrules.Validate(rs, userrules.Input{
			Username: username,
			Email:    email,
		}); len(violations) > 0 {
			msg := ""
			for _, v := range violations {
				if msg != "" {
					msg += " "
				}
				msg += v
			}
			response.ErrorResponse(w, http.StatusBadRequest, msg)
			return
		}
	}

	profileReg := resolveProfileRegistry(ctx)
	if profileReg == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "profile registry not available")
		return
	}
	profileDB, err := profileReg.For(info.OrganizationID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to open profile db")
		return
	}

	fields, err := regfields_repo.New(profileDB.Connector.DB).ListFieldsForApp(info.ID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to load registration schema")
		return
	}

	usersReg := resolveUsersRegistry(ctx)
	if usersReg == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "users registry not available")
		return
	}
	orgDB, err := orgrouter.ForOrg(usersReg, info.OrganizationID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to open org db")
		return
	}

	var existingUserID, existingUsername, existingEmail string
	err = orgDB.QueryRow(
		`SELECT id, username, email FROM users WHERE LOWER(email) = LOWER(?) LIMIT 1`,
		email,
	).Scan(&existingUserID, &existingUsername, &existingEmail)

	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to check user existence")
		return
	}

	userExists := !errors.Is(err, sql.ErrNoRows)

	if userExists {
		if aclErr := insertACL(orgDB, info.ID, existingUserID); aclErr != nil {
			if log := ctx.GetLogger(); log != nil {
				log.Error("register: insert acl for existing user %s: %v", existingUserID, aclErr)
			}
		}

		if pvErr := writeProfileValues(profileDB.Connector.DB, existingUserID, info.ID, fields, body.Fields); pvErr != nil {
			if log := ctx.GetLogger(); log != nil {
				log.Warning("register: write profile values for existing user %s: %v", existingUserID, pvErr)
			}
		}

		loginURL := "/login/a/" +
			url.PathEscape(orgSlug) + "/" +
			url.PathEscape(wsSlug) + "/" +
			url.PathEscape(appSlug)
		sendNotificationEmail(ctx, info.OrganizationID, info.ID, existingEmail, existingUsername, loginURL)

	} else {
		var taken bool
		if scanErr := orgDB.QueryRow(
			`SELECT COUNT(1) > 0 FROM users WHERE LOWER(username) = LOWER(?)`,
			username,
		).Scan(&taken); scanErr != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, "failed to check username")
			return
		}
		if taken {
			response.ErrorResponse(w, http.StatusBadRequest, "username already taken")
			return
		}

		newUserID := newID()
		if _, insertErr := orgDB.Exec(
			`INSERT INTO users (id, username, email, is_active) VALUES (?, ?, ?, 0)`,
			newUserID, username, email,
		); insertErr != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, "failed to create user")
			return
		}

		if aclErr := insertACL(orgDB, info.ID, newUserID); aclErr != nil {
			if log := ctx.GetLogger(); log != nil {
				log.Error("register: insert acl for new user %s: %v", newUserID, aclErr)
			}
		}

		if pvErr := writeProfileValues(profileDB.Connector.DB, newUserID, info.ID, fields, body.Fields); pvErr != nil {
			if log := ctx.GetLogger(); log != nil {
				log.Warning("register: write profile values for new user %s: %v", newUserID, pvErr)
			}
		}

		if actSvc := resolveActivationService(ctx); actSvc != nil {
			_, actErr := actSvc.Start(context.Background(), activation.StartArgs{
				UserType: activation.UserTypeUser,
				UserID:   newUserID,
				OrgID:    info.OrganizationID,
				AppID:    info.ID,
				Username: username,
				Email:    email,
				Redirect: &activation.RedirectTarget{
					OrgSlug:       orgSlug,
					WorkspaceSlug: wsSlug,
					ClientID:      appSlug,
				},
			})
			if actErr != nil {
				if log := ctx.GetLogger(); log != nil {
					log.Error("register: activation start for user %s: %v", newUserID, actErr)
				}
			}
		}
	}

	response.SuccessResponse(w, http.StatusAccepted, regfields_entity.RegisterSuccess{
		Message: "Your registration has been received. Check your email if an activation link was sent.",
	})
}

// insertACL inserts an application_user_acl row, ignoring duplicates.
func insertACL(orgDB *sql.DB, appID, userID string) error {
	_, err := orgDB.Exec(
		`INSERT OR IGNORE INTO application_user_acl (id, application_id, user_id, roles, is_active)
		 VALUES (?, ?, ?, '[]', 1)`,
		newID(), appID, userID,
	)
	if err != nil {
		return fmt.Errorf("register: insert acl: %w", err)
	}
	return nil
}

// writeProfileValues splits the submitted field values by source and
// persists them: profile-source fields go to user_profiles, custom-source
// fields go to application_user_profiles. System-source fields are
// consumed as identity and not stored here.
func writeProfileValues(
	profileDB *sql.DB,
	userID, appID string,
	fields []regfields_entity.Field,
	submitted map[string]string,
) error {
	profilePatch := map[string]interface{}{}
	customPatch := map[string]interface{}{}

	repo := regfields_repo.New(profileDB)

	for _, f := range fields {
		val, ok := submitted[f.ID]
		if !ok || val == "" {
			continue
		}
		switch f.Source {
		case regfields_entity.FieldSourceProfile:
			if f.ProfileFieldID == nil {
				continue
			}
			pf, err := repo.LookupProfileField(*f.ProfileFieldID)
			if err != nil {
				continue
			}
			profilePatch[pf.Name] = val

		case regfields_entity.FieldSourceCustom:
			if f.Name == "" {
				continue
			}
			customPatch[f.Name] = val
		}
	}

	if len(profilePatch) > 0 {
		if err := mergeUserProfile(profileDB, userID, profilePatch); err != nil {
			return fmt.Errorf("merge user profile: %w", err)
		}
	}

	if len(customPatch) > 0 {
		if err := mergeAppUserProfile(profileDB, appID, userID, customPatch); err != nil {
			return fmt.Errorf("merge app user profile: %w", err)
		}
	}

	return nil
}

func mergeUserProfile(db *sql.DB, userID string, patch map[string]interface{}) error {
	current := map[string]interface{}{}
	var raw string
	err := db.QueryRow(
		`SELECT profile_data FROM user_profiles WHERE user_id = ? LIMIT 1`, userID,
	).Scan(&raw)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("read user_profiles: %w", err)
	}
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &current)
	}
	for k, v := range patch {
		current[k] = v
	}
	blob, err := json.Marshal(current)
	if err != nil {
		return fmt.Errorf("marshal user_profiles: %w", err)
	}
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	_, err = db.Exec(
		`INSERT INTO user_profiles (user_id, profile_data, updated_at)
		 VALUES (?, ?, ?)
		 ON CONFLICT(user_id) DO UPDATE SET
		   profile_data = excluded.profile_data,
		   updated_at   = excluded.updated_at`,
		userID, string(blob), now,
	)
	return err
}

func mergeAppUserProfile(db *sql.DB, appID, userID string, patch map[string]interface{}) error {
	current := map[string]interface{}{}
	var raw string
	err := db.QueryRow(
		`SELECT profile_data FROM application_user_profiles
		 WHERE application_id = ? AND user_id = ? LIMIT 1`,
		appID, userID,
	).Scan(&raw)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("read application_user_profiles: %w", err)
	}
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &current)
	}
	for k, v := range patch {
		current[k] = v
	}
	blob, err := json.Marshal(current)
	if err != nil {
		return fmt.Errorf("marshal application_user_profiles: %w", err)
	}
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	_, err = db.Exec(
		`INSERT INTO application_user_profiles (id, application_id, user_id, profile_data, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(application_id, user_id) DO UPDATE SET
		   profile_data = excluded.profile_data,
		   updated_at   = excluded.updated_at`,
		newID(), appID, userID, string(blob), now,
	)
	return err
}

// sendNotificationEmail dispatches the app_registration_notification event
// for an existing user who has just been mapped to an application. Failures
// are logged by the caller; this function never blocks the request.
func sendNotificationEmail(ctx interface{}, orgID, appID, email, username, loginURL string) {
	mailSvc := resolveMailService(ctx)
	mailCfg := resolveMailScopedResolver(ctx)
	if mailSvc == nil || mailCfg == nil {
		return
	}

	event := activation.EventAppRegistrationNotification
	template := mailCfg.TemplateForEvent(orgID, appID, event)
	if template == "" {
		return
	}
	account, resolvedOrgID, resolvedAppID := mailCfg.AccountForEvent(orgID, appID, event)
	if account == "" {
		return
	}

	websiteTitle := ""
	if gs := resolveGeneralStore(ctx); gs != nil {
		websiteTitle = gs.PageTitle()
	}

	data := map[string]interface{}{
		"Username":     username,
		"LoginURL":     loginURL,
		"WebsiteTitle": websiteTitle,
	}
	task := coconotification.Task{
		Ref: coconotification.SenderRef{Name: account, Scope: iam_notification.Scope(resolvedOrgID, resolvedAppID)},
		To:  []coconotification.Address{{Email: email, Name: username}},
	}
	// Prefer this application's own active template of the same name,
	// then this org's, over the global renderer — falls through
	// untouched (task.Template + task.Data set) when neither tier has
	// one of its own.
	if subject, text, html, ok, rerr := mailCfg.RenderTemplate(orgID, appID, template, data); rerr == nil && ok {
		task.Subject, task.TextBody, task.HTMLBody = subject, text, html
	} else {
		task.Template = template
		task.Data = data
	}

	_, _ = mailSvc.Enqueue(task)
}

// loadRegistrationConfig reads allow_registration and registration_type from
// the applications row in the per-org users.db. Returns (false, "", nil) when
// the app row is missing — callers treat that as "registration disabled".
func loadRegistrationConfig(reqCtx request.RequestContext, appID string) (bool, string, error) {
	bag, ok := reqCtx.GetDI().(bagGetter)
	if !ok {
		return false, "", errors.New("register: DI not a bagGetter")
	}
	raw, ok := bag.Get(users_dbregistry.ContextBagKey)
	if !ok {
		return false, "", errors.New("register: users registry not in DI")
	}
	reg, ok := raw.(*users_dbregistry.OrgUserDBRegistry)
	if !ok {
		return false, "", errors.New("register: users registry type mismatch")
	}
	orgDB, _, err := orgrouter.OrgDBForApp(reg, appID)
	if err != nil {
		return false, "", nil
	}
	var allow bool
	var regType string
	err = orgDB.QueryRow(
		`SELECT allow_registration, registration_type FROM applications WHERE id = ? LIMIT 1`, appID,
	).Scan(&allow, &regType)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, "", nil
		}
		return false, "", err
	}
	if regType == "" {
		regType = "legacy"
	}
	return allow, regType, nil
}

// -- slug parsing -------------------------------------------------------

func parseSlugSegments(path string) (org, ws, app string, ok bool) {
	parts := splitSegments(path)
	if len(parts) < 5 {
		return "", "", "", false
	}
	if parts[0] != "a" {
		return "", "", "", false
	}
	return parts[1], parts[2], parts[3], true
}

func splitSegments(path string) []string {
	out := make([]string, 0, 8)
	start := 0
	for i := 0; i <= len(path); i++ {
		if i == len(path) || path[i] == '/' {
			if i > start {
				out = append(out, path[start:i])
			}
			start = i + 1
		}
	}
	return out
}

// -- DI resolvers -------------------------------------------------------

type bagGetter interface {
	Get(key string) (interface{}, bool)
}

func resolveLoginPageService(ctx interface{}) *loginpage.Service {
	bag, ok := ctx.(bagGetter)
	if !ok {
		return nil
	}
	raw, ok := bag.Get(loginpage.ContextBagKeyService)
	if !ok {
		return nil
	}
	svc, _ := raw.(*loginpage.Service)
	return svc
}

func resolveProfileRegistry(ctx interface{}) *profile_dbregistry.OrgDBRegistry {
	bag, ok := ctx.(bagGetter)
	if !ok {
		return nil
	}
	raw, ok := bag.Get(profile_dbregistry.ContextBagKey)
	if !ok {
		return nil
	}
	reg, _ := raw.(*profile_dbregistry.OrgDBRegistry)
	return reg
}

func resolveUsersRegistry(ctx interface{}) *users_dbregistry.OrgUserDBRegistry {
	bag, ok := ctx.(bagGetter)
	if !ok {
		return nil
	}
	raw, ok := bag.Get(users_dbregistry.ContextBagKey)
	if !ok {
		return nil
	}
	reg, _ := raw.(*users_dbregistry.OrgUserDBRegistry)
	return reg
}

func resolveActivationService(ctx interface{}) *activation.Service {
	bag, ok := ctx.(bagGetter)
	if !ok {
		return nil
	}
	raw, ok := bag.Get(activation.ContextBagKeyService)
	if !ok {
		return nil
	}
	svc, _ := raw.(*activation.Service)
	return svc
}

func resolveUserRulesStore(ctx interface{}) *userrules.Store {
	bag, ok := ctx.(bagGetter)
	if !ok {
		return nil
	}
	raw, ok := bag.Get(userrules.ContextBagKeyStore)
	if !ok {
		return nil
	}
	store, _ := raw.(*userrules.Store)
	return store
}

func resolveMailService(ctx interface{}) coconotification.Service {
	bag, ok := ctx.(bagGetter)
	if !ok {
		return nil
	}
	raw, ok := bag.Get(iam_notification.ContextBagKeyService)
	if !ok {
		return nil
	}
	svc, _ := raw.(coconotification.Service)
	return svc
}

func resolveMailScopedResolver(ctx interface{}) *iam_notification.ScopedResolver {
	bag, ok := ctx.(bagGetter)
	if !ok {
		return nil
	}
	raw, ok := bag.Get(iam_notification.ContextBagKey)
	if !ok {
		return nil
	}
	r, _ := raw.(*iam_notification.ScopedResolver)
	return r
}

func resolveGeneralStore(ctx interface{}) *general.Store {
	bag, ok := ctx.(bagGetter)
	if !ok {
		return nil
	}
	raw, ok := bag.Get(general.ContextBagKeyStore)
	if !ok {
		return nil
	}
	s, _ := raw.(*general.Store)
	return s
}

// -- helpers ------------------------------------------------------------

func newID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "00000000-0000-0000-0000-000000000000"
	}
	hx := hex.EncodeToString(buf)
	return fmt.Sprintf("%s-%s-%s-%s-%s", hx[:8], hx[8:12], hx[12:16], hx[16:20], hx[20:32])
}
