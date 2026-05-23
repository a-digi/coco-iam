package admin

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/a-digi/coco-iam/src/applications/registrationfields/entity"
	"github.com/a-digi/coco-iam/src/applications/registrationfields/repository"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// ReplaceHandler serves PUT /api/v1/applications/{id}/registration-fields.
// Atomic full replace of both steps and fields. The request body
// mirrors the list endpoint's shape so a round-trip is symmetric.
type ReplaceHandler struct{}

// replacePayload is the wire shape the admin UI PUTs. Stable IDs
// are expected from the client so drag-to-reorder doesn't produce
// delete+add churn.
type replacePayload struct {
	Steps []replaceStep `json:"steps"`
}

type replaceStep struct {
	ID          string         `json:"id"`
	Title       string         `json:"title"`
	Description string         `json:"description,omitempty"`
	OrderIndex  int            `json:"order_index"`
	Fields      []replaceField `json:"fields"`
}

type replaceField struct {
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

// @Summary     Replace registration fields
// @Tags        app-registration-fields
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Application ID"
// @Router      /applications/applications/{id}/registration-fields [put]
func (h *ReplaceHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	appID := appIDFromPath(reqCtx)
	if appID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "missing application id")
		return
	}

	var body replacePayload
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	defer r.Body.Close()

	repo, _, ok := openRepo(reqCtx, appID)
	if !ok {
		return
	}

	steps, fields, err := validateAndBuild(body, appID, repo)
	if err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := repo.ReplaceForApp(appID, steps, fields); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, map[string]string{"status": "ok"})
}

// validateAndBuild does all the admin-side invariant checks in a
// pure helper (easy to unit-test without any HTTP plumbing), then
// returns the repository-shaped entities ready to pass into
// ReplaceForApp.
//
// Invariants enforced:
//   - at least one step
//   - every step has an id
//   - duplicate step ids rejected
//   - every field has an id
//   - duplicate field ids rejected
//   - field.step_id references a step in the same payload
//   - source ∈ {profile, custom}
//   - custom fields require name + label + data_type
//   - profile fields require a profile_field_id that exists + is active
//   - duplicate field `name` (across all steps) rejected
func validateAndBuild(p replacePayload, appID string, probe profileFieldProbe) ([]entity.Step, []entity.Field, error) {
	if len(p.Steps) == 0 {
		return nil, nil, errors.New("at least one step is required")
	}

	steps := make([]entity.Step, 0, len(p.Steps))
	stepIDs := make(map[string]struct{}, len(p.Steps))
	var fields []entity.Field
	fieldIDs := make(map[string]struct{})
	fieldNames := make(map[string]string) // name → offending field id (for clearer errors)

	for _, s := range p.Steps {
		if s.ID == "" {
			return nil, nil, errors.New("step id is required")
		}
		if _, dup := stepIDs[s.ID]; dup {
			return nil, nil, fmt.Errorf("duplicate step id %q", s.ID)
		}
		stepIDs[s.ID] = struct{}{}
		steps = append(steps, entity.Step{
			ID:            s.ID,
			ApplicationID: appID,
			OrderIndex:    s.OrderIndex,
			Title:         s.Title,
			Description:   s.Description,
		})

		for _, f := range s.Fields {
			if f.ID == "" {
				return nil, nil, errors.New("field id is required")
			}
			if _, dup := fieldIDs[f.ID]; dup {
				return nil, nil, fmt.Errorf("duplicate field id %q", f.ID)
			}
			fieldIDs[f.ID] = struct{}{}

			src := entity.FieldSource(f.Source)
			if !src.IsValid() {
				return nil, nil, fmt.Errorf("field %q: invalid source %q", f.ID, f.Source)
			}

			// Derive the `name` used for duplicate detection. For
			// profile-sourced rows the name comes from the
			// referenced profile_fields row — we probe the DB for
			// it below. For custom rows the admin supplies name
			// directly.
			var effectiveName string

			switch src {
			case entity.FieldSourceCustom:
				if f.Name == "" || f.Label == "" || f.DataType == "" {
					return nil, nil, fmt.Errorf("field %q: custom source requires name, label, and data_type", f.ID)
				}
				effectiveName = f.Name

			case entity.FieldSourceProfile:
				if f.ProfileFieldID == nil || *f.ProfileFieldID == "" {
					return nil, nil, fmt.Errorf("field %q: profile source requires profile_field_id", f.ID)
				}
				exists, perr := probe.ProfileFieldExists(*f.ProfileFieldID)
				if perr != nil {
					return nil, nil, fmt.Errorf("field %q: profile_field_id probe: %w", f.ID, perr)
				}
				if !exists {
					return nil, nil, fmt.Errorf("field %q: unknown profile_field_id %q", f.ID, *f.ProfileFieldID)
				}
				// Name for dup-detection: the profile_field_id
				// itself — two registration rows pointing at the
				// same profile field would duplicate on submit.
				effectiveName = "profile:" + *f.ProfileFieldID
			}

			if prev, dup := fieldNames[effectiveName]; dup {
				return nil, nil, fmt.Errorf("field %q: duplicate name %q (also on field %q)",
					f.ID, effectiveName, prev)
			}
			fieldNames[effectiveName] = f.ID

			fields = append(fields, entity.Field{
				ID:               f.ID,
				ApplicationID:    appID,
				StepID:           s.ID,
				OrderIndex:       f.OrderIndex,
				Source:           src,
				ProfileFieldID:   f.ProfileFieldID,
				RequiredOverride: f.RequiredOverride,
				Name:             f.Name,
				Label:            f.Label,
				Description:      f.Description,
				DataType:         f.DataType,
				IsRequired:       f.IsRequired,
				MinValue:         f.MinValue,
				MaxValue:         f.MaxValue,
				OptionsJSON:      f.OptionsJSON,
				Regex:            f.Regex,
			})
		}
	}

	return steps, fields, nil
}

// profileFieldProbe is the narrow interface validateAndBuild needs
// to check a profile field's existence. Defined here so tests can
// pass a fake without touching the repository.
type profileFieldProbe interface {
	ProfileFieldExists(id string) (bool, error)
}

// Ensure the real repository satisfies the interface. Moves any
// accidental signature drift from a runtime panic to a compile
// error.
var _ profileFieldProbe = (*repository.Repository)(nil)
