// Package service owns the "effective field" resolution that turns
// on-disk rows into the shape the public endpoint publishes. A
// 'profile'-sourced row is merged with its referenced profile_fields
// row (with required_override applied); a 'custom' row passes
// through. Fields are then grouped under their parent step.
//
// The two pure helpers (ResolveEffective, GroupIntoSteps) take plain
// Go values and are fully unit-testable without any DB wiring.
// LoadForApp is the thin orchestrator that stitches the repository
// calls together.
package service

import (
	"fmt"

	"github.com/a-digi/coco-iam/src/applications/registrationfields/entity"
	"github.com/a-digi/coco-iam/src/applications/registrationfields/repository"
)

// EffectiveField is the shape the public endpoint serialises. It's
// the merged view of a registration row and (when source='profile')
// the profile_fields row it points at.
type EffectiveField struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Label           string   `json:"label"`
	Description     string   `json:"description,omitempty"`
	DataType        string   `json:"data_type"`
	IsRequired      bool     `json:"is_required"`
	MinValue        *int     `json:"min_value,omitempty"`
	MaxValue        *int     `json:"max_value,omitempty"`
	Options         []string `json:"options,omitempty"`
	Regex           string   `json:"regex,omitempty"`
	Source          string   `json:"source"`
	ProfileFieldID  string   `json:"profile_field_id,omitempty"`
	SystemFieldName string   `json:"system_field_name,omitempty"`
	// StepID is set by the grouping pass, not the resolver.
	StepID string `json:"-"`
}

// StepWithFields mirrors the nested shape external apps consume:
// one step carrying its ordered field list.
type StepWithFields struct {
	ID          string           `json:"id"`
	Title       string           `json:"title"`
	Description string           `json:"description,omitempty"`
	OrderIndex  int              `json:"order_index"`
	Fields      []EffectiveField `json:"fields"`
}

// ResolveEffective merges one registration row with its linked
// profile_fields row (if any). Returns (effective, ok). `ok=false`
// means the row is an orphan — the registration row is
// 'profile'-sourced but the profile field is missing or inactive —
// and the caller should skip it.
//
// Pure function. Takes plain values, returns plain values, no I/O.
// The linked ProfileField is optional: for 'custom' rows the
// caller passes nil.
func ResolveEffective(f entity.Field, linked *repository.ProfileField) (EffectiveField, bool) {
	switch f.Source {
	case entity.FieldSourceCustom:
		return EffectiveField{
			ID:          f.ID,
			Name:        f.Name,
			Label:       f.Label,
			Description: f.Description,
			DataType:    f.DataType,
			IsRequired:  f.IsRequired,
			MinValue:    f.MinValue,
			MaxValue:    f.MaxValue,
			Options:     decodeOptions(f.OptionsJSON),
			Regex:       f.Regex,
			Source:      string(entity.FieldSourceCustom),
			StepID:      f.StepID,
		}, true

	case entity.FieldSourceProfile:
		// Missing link — the profile field was soft-deleted or
		// never existed. Skip the registration row rather than
		// return a half-baked definition that would confuse the
		// consumer.
		if linked == nil {
			return EffectiveField{}, false
		}
		// required_override, if set, wins over the profile field's
		// default. Null override → inherit the profile field's flag.
		required := linked.IsRequired
		if f.RequiredOverride != nil {
			required = *f.RequiredOverride
		}
		// Description on the registration row (if any) wins over
		// the profile field's description — admins sometimes want
		// a reminder on the registration form that's different
		// from the profile label.
		desc := f.Description
		if desc == "" {
			desc = linked.Description
		}
		return EffectiveField{
			ID:             f.ID,
			Name:           linked.Name,
			Label:          linked.Label,
			Description:    desc,
			DataType:       linked.DataType,
			IsRequired:     required,
			MinValue:       linked.MinValue,
			MaxValue:       linked.MaxValue,
			Options:        decodeOptions(linked.OptionsJSON),
			Regex:          linked.Regex,
			Source:         string(entity.FieldSourceProfile),
			ProfileFieldID: linked.ID,
			StepID:         f.StepID,
		}, true

	case entity.FieldSourceSystem:
		name := ""
		if f.SystemFieldName != nil {
			name = *f.SystemFieldName
		}
		if name == "" {
			return EffectiveField{}, false
		}
		label, dataType := systemFieldMeta(name)
		return EffectiveField{
			ID:              f.ID,
			Name:            name,
			Label:           label,
			DataType:        dataType,
			IsRequired:      true,
			Source:          string(entity.FieldSourceSystem),
			SystemFieldName: name,
			StepID:          f.StepID,
		}, true
	}
	// Unknown source — treated as orphan. Shouldn't happen in
	// practice because the repository rejects invalid sources on
	// write.
	return EffectiveField{}, false
}

// systemFieldMeta returns the display label and data type for a known
// system field name. Used when resolving source='system' rows.
func systemFieldMeta(name string) (label, dataType string) {
	switch name {
	case "email":
		return "Email address", "email"
	case "username":
		return "Username", "text"
	default:
		return name, "text"
	}
}

// GroupIntoSteps bundles resolved fields under their parent step.
// Fields whose StepID doesn't match any step in `steps` are dropped
// and returned in the second slice so the caller can log the
// orphan ids. Steps appear in the order they were supplied (the
// repository already sorts by order_index).
//
// Pure function — operates on in-memory values only.
func GroupIntoSteps(steps []entity.Step, fields []EffectiveField) ([]StepWithFields, []string) {
	byStep := make(map[string][]EffectiveField, len(steps))
	for _, s := range steps {
		// Pre-seed with an empty (non-nil) slice so a step without
		// any fields still marshals as `"fields": []` instead of
		// `"fields": null`. JSON null on the wire breaks consumers
		// that read step.fields.length without a guard.
		byStep[s.ID] = []EffectiveField{}
	}

	var orphans []string
	for _, f := range fields {
		if _, ok := byStep[f.StepID]; !ok {
			orphans = append(orphans, f.ID)
			continue
		}
		byStep[f.StepID] = append(byStep[f.StepID], f)
	}

	out := make([]StepWithFields, 0, len(steps))
	for _, s := range steps {
		out = append(out, StepWithFields{
			ID:          s.ID,
			Title:       s.Title,
			Description: s.Description,
			OrderIndex:  s.OrderIndex,
			Fields:      byStep[s.ID],
		})
	}
	return out, orphans
}

// LoadForApp is the orchestrator: pulls steps + fields for the
// application, resolves each field, drops orphans, groups by step.
// Returns the ready-to-marshal list plus a slice of logged-once
// warnings the caller can surface at their discretion.
func LoadForApp(repo *repository.Repository, appID string) ([]StepWithFields, []string, error) {
	steps, err := repo.ListStepsForApp(appID)
	if err != nil {
		return nil, nil, err
	}
	rawFields, err := repo.ListFieldsForApp(appID)
	if err != nil {
		return nil, nil, err
	}

	var warnings []string
	resolved := make([]EffectiveField, 0, len(rawFields))
	for _, f := range rawFields {
		var linked *repository.ProfileField
		if f.Source == entity.FieldSourceProfile && f.ProfileFieldID != nil {
			pf, perr := repo.LookupProfileField(*f.ProfileFieldID)
			if perr == nil {
				linked = pf
			} else {
				warnings = append(warnings, fmt.Sprintf(
					"registrationfields: skipping orphan field %q (profile_field_id %q): %v",
					f.ID, *f.ProfileFieldID, perr,
				))
			}
		}
		eff, ok := ResolveEffective(f, linked)
		if !ok {
			continue
		}
		resolved = append(resolved, eff)
	}

	grouped, stepOrphans := GroupIntoSteps(steps, resolved)
	for _, id := range stepOrphans {
		warnings = append(warnings, fmt.Sprintf("registrationfields: skipping field %q (unknown step)", id))
	}
	return grouped, warnings, nil
}
