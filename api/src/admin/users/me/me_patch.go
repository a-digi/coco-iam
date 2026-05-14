package me

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/a-digi/coco-iam/src/admin/users/profile/entity"
	profile_repo "github.com/a-digi/coco-iam/src/admin/users/profile/repository"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// MePatchHandler serves PATCH /api/v1/admin/users/me. Updates the
// profile fields the admin owns about themselves. Avatar handling
// is split off into its own upload/delete endpoints — admins who
// accidentally send an avatar_asset_id on this endpoint get a 400
// so the two lifecycles can't trample each other.
type MePatchHandler struct{}

// patchBody is deliberately NOT a map — using named fields rejects
// typos at decode time via DisallowUnknownFields.
type patchBody struct {
	FirstName *string `json:"first_name,omitempty"`
	LastName  *string `json:"last_name,omitempty"`
	Phone     *string `json:"phone,omitempty"`
	Locale    *string `json:"locale,omitempty"`
	Timezone  *string `json:"timezone,omitempty"`
}

const profileFieldMaxLen = 120

// localeRE accepts BCP 47-shaped tags: a 2-3 letter language,
// optionally followed by `-subtag` groups (script / region / etc.).
// Permissive — we don't validate against a real locale list.
var localeRE = regexp.MustCompile(`^[A-Za-z]{2,3}(?:-[A-Za-z0-9]{2,8})*$`)

// timezoneRE accepts IANA-shaped names: one or more `/`-separated
// identifier segments (e.g. Europe/Berlin, America/Argentina/Buenos_Aires).
var timezoneRE = regexp.MustCompile(`^[A-Za-z_]+(?:/[A-Za-z_]+)*$`)

func (h *MePatchHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	ctx := reqCtx.GetDI()

	userID, ok := subjectFromBearer(r.Header.Get("Authorization"))
	if !ok {
		response.ErrorResponse(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var body patchBody
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	defer r.Body.Close()

	if err := validatePatchBody(body); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	manager := ctx.GetDatabaseManager()
	if manager == nil || manager.Connector == nil || manager.Connector.DB == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "database manager not available")
		return
	}
	db := manager.Connector.DB
	repo := profile_repo.New(db)

	// Merge the incoming patch onto the current row so PATCH is
	// actually partial (not PUT). Missing fields keep their prior
	// value; explicit empty strings are acceptable updates.
	current, err := repo.FindByAdminUserID(userID)
	if err != nil && !errors.Is(err, profile_repo.ErrNotFound) {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	merged := mergePatch(current, body, userID)

	if err := repo.Upsert(merged); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, map[string]string{"status": "ok"})
}

// validatePatchBody enforces the string-length + format rules from
// the plan. Pure — takes no I/O dependency.
func validatePatchBody(body patchBody) error {
	if body.FirstName != nil {
		if err := validateLen("first_name", *body.FirstName, profileFieldMaxLen); err != nil {
			return err
		}
	}
	if body.LastName != nil {
		if err := validateLen("last_name", *body.LastName, profileFieldMaxLen); err != nil {
			return err
		}
	}
	if body.Phone != nil {
		if err := validateLen("phone", *body.Phone, profileFieldMaxLen); err != nil {
			return err
		}
	}
	if body.Locale != nil {
		v := strings.TrimSpace(*body.Locale)
		if v != "" && !localeRE.MatchString(v) {
			return errors.New("locale must look like a BCP 47 tag (e.g. en-US) or be empty")
		}
	}
	if body.Timezone != nil {
		v := strings.TrimSpace(*body.Timezone)
		if v != "" && !timezoneRE.MatchString(v) {
			return errors.New("timezone must look like an IANA name (e.g. Europe/Berlin) or be empty")
		}
	}
	return nil
}

func validateLen(field, value string, max int) error {
	// Rune-length, not byte-length — `Müller` is 6 chars.
	if countRunes(value) > max {
		return fmt.Errorf("%s: maximum %d characters", field, max)
	}
	return nil
}

func countRunes(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}

// mergePatch applies the non-nil fields of `body` onto `current`,
// returning the AdminUserProfile to upsert. When `current` is nil
// (no existing profile row), starts from an empty profile keyed to
// the user id. Trims whitespace for the text fields.
func mergePatch(current *entity.AdminUserProfile, body patchBody, userID string) *entity.AdminUserProfile {
	out := &entity.AdminUserProfile{AdminUserID: userID}
	if current != nil {
		out.FirstName = current.FirstName
		out.LastName = current.LastName
		out.Phone = current.Phone
		out.AvatarAssetID = current.AvatarAssetID
		out.Locale = current.Locale
		out.Timezone = current.Timezone
	}
	if body.FirstName != nil {
		out.FirstName = strings.TrimSpace(*body.FirstName)
	}
	if body.LastName != nil {
		out.LastName = strings.TrimSpace(*body.LastName)
	}
	if body.Phone != nil {
		out.Phone = strings.TrimSpace(*body.Phone)
	}
	if body.Locale != nil {
		out.Locale = strings.TrimSpace(*body.Locale)
	}
	if body.Timezone != nil {
		out.Timezone = strings.TrimSpace(*body.Timezone)
	}
	return out
}
