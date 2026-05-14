package admin

import (
	"net/http"

	"github.com/a-digi/coco-iam/src/applications/registrationfields/entity"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// ListHandler serves GET /api/v1/applications/{id}/registration-fields.
// Returns the saved design in the same nested shape the PUT handler
// accepts, so the admin UI can round-trip without reshaping.
//
// Note: this endpoint returns the RAW rows (source-dependent columns
// filled in only when source='custom'), not the resolved effective
// view the public endpoint publishes. Admins need to see the
// source distinction to edit it.
type ListHandler struct{}

type listStep struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	OrderIndex  int        `json:"order_index"`
	Fields      []listField `json:"fields"`
}

type listField struct {
	ID               string  `json:"id"`
	OrderIndex       int     `json:"order_index"`
	Source           string  `json:"source"`
	ProfileFieldID   *string `json:"profile_field_id,omitempty"`
	RequiredOverride *bool   `json:"required_override,omitempty"`
	Name             string  `json:"name,omitempty"`
	Label            string  `json:"label,omitempty"`
	Description      string  `json:"description,omitempty"`
	DataType         string  `json:"data_type,omitempty"`
	IsRequired       bool    `json:"is_required"`
	MinValue         *int    `json:"min_value,omitempty"`
	MaxValue         *int    `json:"max_value,omitempty"`
	OptionsJSON      string  `json:"options_json,omitempty"`
	Regex            string  `json:"regex,omitempty"`
}

type listResponse struct {
	Steps []listStep `json:"steps"`
}

func (h *ListHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	appID := appIDFromPath(reqCtx)
	if appID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "missing application id")
		return
	}
	repo, _, ok := openRepo(reqCtx, appID)
	if !ok {
		return
	}

	steps, err := repo.ListStepsForApp(appID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	fields, err := repo.ListFieldsForApp(appID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Bucket fields by step_id. Steps without any fields still
	// appear in the response with an empty Fields slice so the
	// admin UI renders the step card. Initialised as [] (not nil)
	// because Go marshals a nil []T as JSON null — the frontend
	// would then crash on step.fields.length.
	byStep := make(map[string][]listField, len(steps))
	for _, s := range steps {
		byStep[s.ID] = []listField{}
	}
	for _, f := range fields {
		if _, ok := byStep[f.StepID]; !ok {
			continue // orphan; shouldn't happen given repo invariants
		}
		byStep[f.StepID] = append(byStep[f.StepID], listField{
			ID:               f.ID,
			OrderIndex:       f.OrderIndex,
			Source:           string(f.Source),
			ProfileFieldID:   f.ProfileFieldID,
			RequiredOverride: f.RequiredOverride,
			Name:             f.Name,
			Label:            f.Label,
			Description:      f.Description,
			DataType:         f.DataType,
			IsRequired:       f.IsRequired,
			MinValue:         f.MinValue,
			MaxValue:         f.MaxValue,
			OptionsJSON:      defaultEmpty(f.OptionsJSON, "[]"),
			Regex:            f.Regex,
		})
	}

	out := listResponse{Steps: make([]listStep, 0, len(steps))}
	for _, s := range steps {
		out.Steps = append(out.Steps, listStep{
			ID:          s.ID,
			Title:       s.Title,
			Description: s.Description,
			OrderIndex:  s.OrderIndex,
			Fields:      byStep[s.ID],
		})
	}
	response.SuccessResponse(w, http.StatusOK, out)
}

// defaultEmpty returns `def` when `s` is the empty string. Saves a
// caller an `if s == "" { s = def }` repetition — we use it to
// ensure options_json is always a valid JSON array on the wire.
func defaultEmpty(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// Silence unused-import guards when this file is trimmed later.
var _ = entity.FieldSourceCustom
