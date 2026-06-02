// Package public hosts the slug-routed public endpoint that
// external applications fetch to render their registration form:
//
//	GET /a/{orgSlug}/{wsSlug}/{appSlug}/registration-fields
//
// No authentication — field schemas are the public contract of
// what registration asks for. When the app has allow_registration
// disabled (or the slug triple doesn't resolve), the endpoint
// returns `404 not found` with no distinguishing body — the
// response must not be usable as an oracle for "which apps have
// registration enabled".
package public

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/a-digi/coco-iam/src/applications/loginpage"
	regfields_entity "github.com/a-digi/coco-iam/src/applications/registrationfields/entity"
	"github.com/a-digi/coco-iam/src/applications/registrationfields/repository"
	"github.com/a-digi/coco-iam/src/applications/registrationfields/service"
	profile_dbregistry "github.com/a-digi/coco-iam/src/organizations/profile/dbregistry"
	users_dbregistry "github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-iam/src/orgrouter"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// genericNotFound is the single obfuscated body returned for every
// not-found path — bad slug, disabled registration, missing app.
// Callers can't tell which rejected them.
const genericNotFound = "not found"

// RegistrationFieldsHandler serves the GET endpoint. Logs any
// orphan warnings from the resolver to the DI logger but still
// serves a best-effort response (consumer gets the fields we
// could resolve).
type RegistrationFieldsHandler struct{}

type response200 struct {
	RegistrationType string                         `json:"registration_type"`
	IdentityFields   []regfields_entity.IdentityFieldDef `json:"identity_fields"`
	Steps            []service.StepWithFields       `json:"steps"`
}

// @Summary     Get public registration fields
// @Tags        app-public
// @Produce     json
// @Success     200  {object}  regfields_entity.RegistrationFieldsSuccess
// @Failure     404  {object}  response.ErrorBody
// @Failure     500  {object}  response.ErrorBody
// @Router      /a/{orgSlug}/{wsSlug}/{appSlug}/registration-fields [get]
func (h *RegistrationFieldsHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	ctx := reqCtx.GetDI()

	orgSlug, wsSlug, appSlug, ok := parseSlugSegments(r.URL.Path)
	if !ok {
		response.ErrorResponse(w, http.StatusNotFound, genericNotFound)
		return
	}

	loginSvc := resolveLoginPageService(ctx)
	profileReg := resolveProfileRegistry(ctx)
	if loginSvc == nil || profileReg == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "service not available")
		return
	}

	// Resolve slug triple → app info. Any failure collapses to the
	// generic 404 so the endpoint can't leak which slugs exist.
	info, err := loginSvc.Store.FindBySlugs(orgSlug, wsSlug, appSlug)
	if err != nil {
		response.ErrorResponse(w, http.StatusNotFound, genericNotFound)
		return
	}

	// Feature gate. allow_registration = false is indistinguishable
	// from "unknown app" on the wire.
	allow, regType, err := loadRegistrationConfig(reqCtx, info.ID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to read registration config")
		return
	}
	if !allow {
		response.ErrorResponse(w, http.StatusNotFound, genericNotFound)
		return
	}

	// Open the per-org profiles.db (shared with profile_fields +
	// user_profiles).
	profileDB, err := profileReg.For(info.OrganizationID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to open per-org db")
		return
	}
	repo := repository.New(profileDB.Connector.DB)

	steps, warnings, err := service.LoadForApp(repo, info.ID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to load registration schema")
		return
	}
	if len(warnings) > 0 {
		if log := ctx.GetLogger(); log != nil {
			for _, warn := range warnings {
				log.Warning(warn)
			}
		}
	}

	if steps == nil {
		// Always serialise `steps` as `[]`, never `null`. Consumer
		// UIs don't want to branch on nullness.
		steps = []service.StepWithFields{}
	}

	response.SuccessResponse(w, http.StatusOK, response200{
		RegistrationType: regType,
		IdentityFields:   identityFieldsFor(regType),
		Steps:            steps,
	})
}

// loadRegistrationConfig reads the two flags on the applications
// row we need: allow_registration (feature gate) and
// registration_type (consumer hint). Applications now live in the
// per-org DB; resolve it by scanning per-org DBs.
func loadRegistrationConfig(reqCtx request.RequestContext, appID string) (bool, string, error) {
	bag, ok := reqCtx.GetDI().(bagGetter)
	if !ok {
		return false, "", errors.New("registrationfields public: DI not a bagGetter")
	}
	raw, ok := bag.Get(users_dbregistry.ContextBagKey)
	if !ok {
		return false, "", errors.New("registrationfields public: users registry not in DI")
	}
	reg, ok := raw.(*users_dbregistry.OrgUserDBRegistry)
	if !ok {
		return false, "", errors.New("registrationfields public: users registry type mismatch")
	}

	orgDB, _, err := orgrouter.OrgDBForApp(reg, appID)
	if err != nil {
		return false, "", nil
	}

	var allow bool
	var regType string
	err = orgDB.QueryRow(
		`SELECT allow_registration, registration_type FROM applications WHERE id = ? LIMIT 1`,
		appID,
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

// -- slug parsing ------------------------------------------------------

// parseSlugSegments extracts (orgSlug, wsSlug, appSlug) from a path
// shaped like `/a/<org>/<ws>/<app>/registration-fields`. Malformed
// path returns ok=false; the caller collapses that to a 404.
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

// -- identity fields ----------------------------------------------------

// identityFieldsFor returns the fixed identity fields required by the
// given registration type. For "legacy" (and the empty-string default)
// those are always email and username — the user sets a password later
// via the activation link.
func identityFieldsFor(regType string) []regfields_entity.IdentityFieldDef {
	switch regType {
	case "legacy", "":
		return []regfields_entity.IdentityFieldDef{
			{Name: "email", Label: "Email address", DataType: "email", IsRequired: true},
			{Name: "username", Label: "Username", DataType: "text", IsRequired: true},
		}
	default:
		return []regfields_entity.IdentityFieldDef{}
	}
}

// -- DI resolvers ------------------------------------------------------

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
