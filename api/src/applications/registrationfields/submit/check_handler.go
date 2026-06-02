package submit

import (
	"net/http"
	"strings"

	regfields_entity "github.com/a-digi/coco-iam/src/applications/registrationfields/entity"
	"github.com/a-digi/coco-iam/src/orgrouter"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// CheckHandler serves POST /a/{orgSlug}/{wsSlug}/{appSlug}/register/check.
// It reports whether an email and/or username are available in the organisation
// without creating any state. Both fields are optional — omit one to skip its check.
type CheckHandler struct{}

// @Summary     Check registration field availability
// @Tags        app-public
// @Accept      json
// @Produce     json
// @Param       body  body  regfields_entity.CheckRequest  true  "Fields to check"
// @Success     200   {object}  regfields_entity.CheckResponse
// @Failure     400   {object}  response.ErrorBody
// @Failure     404   {object}  response.ErrorBody
// @Failure     500   {object}  response.ErrorBody
// @Router      /a/{orgSlug}/{wsSlug}/{appSlug}/register/check [post]
func (h *CheckHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	orgSlug, wsSlug, appSlug, ok := parseSlugSegments(r.URL.Path)
	if !ok {
		response.ErrorResponse(w, http.StatusNotFound, genericNotFound)
		return
	}

	loginSvc := resolveLoginPageService(reqCtx.GetDI())
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

	var body regfields_entity.CheckRequest
	if err := reqCtx.BindJSON(&body); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	email := strings.TrimSpace(body.Email)
	username := strings.TrimSpace(body.Username)
	if email == "" && username == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "at least one field must be provided")
		return
	}

	usersReg := resolveUsersRegistry(reqCtx.GetDI())
	if usersReg == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "users registry not available")
		return
	}
	orgDB, err := orgrouter.ForOrg(usersReg, info.OrganizationID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to open org db")
		return
	}

	result := regfields_entity.CheckResponse{}

	if email != "" {
		var count int
		if err := orgDB.QueryRow(
			`SELECT COUNT(1) FROM users WHERE LOWER(email) = LOWER(?)`, email,
		).Scan(&count); err != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, "failed to check email")
			return
		}
		result.Email = &regfields_entity.FieldAvailability{Available: count == 0}
	}

	if username != "" {
		var count int
		if err := orgDB.QueryRow(
			`SELECT COUNT(1) FROM users WHERE LOWER(username) = LOWER(?)`, username,
		).Scan(&count); err != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, "failed to check username")
			return
		}
		result.Username = &regfields_entity.FieldAvailability{Available: count == 0}
	}

	response.SuccessResponse(w, http.StatusOK, result)
}
