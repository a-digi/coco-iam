// Package userprofile exposes the signed-in workspace-application
// user's own profile via the slug-routed `/a/{org}/{ws}/{app}/profile/me`
// endpoint. Read-only; subject identity comes from the user's
// RS256 access token.
//
// The package sits separately from admin-side `organizations/profile`
// — that one serves admin session clients managing other users; this
// one serves the user themselves with a user access token.
package userprofile

import (
	"sort"

	"github.com/a-digi/coco-iam/src/organizations/profile/entity"
)

// FieldWithValue is the wire shape each entry of the response's
// `fields` array takes: the field definition plus the user's value
// for it. Matches the self-describing layout the existing
// `/registration-fields` endpoint uses so external consumers treat
// both surfaces the same way.
type FieldWithValue struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Label       string      `json:"label"`
	Description string      `json:"description,omitempty"`
	DataType    string      `json:"data_type"`
	IsRequired  bool        `json:"is_required"`
	MinValue    *int        `json:"min_value,omitempty"`
	MaxValue    *int        `json:"max_value,omitempty"`
	Options     []string    `json:"options,omitempty"`
	Regex       string      `json:"regex,omitempty"`
	// Value is the user's current value for this field, or null
	// when the user hasn't filled it in. Type follows DataType:
	// text → string, number → float64, date → string, select → string, etc.
	Value interface{} `json:"value"`
}

// BuildResponse merges the org's profile-field definitions with the
// user's stored values. Pure function — no I/O — so the handler's
// orchestration can stay simple and this logic is trivially
// testable.
//
// Rules:
//   - Inactive fields (`is_active = false`) are omitted entirely;
//     admins retiring a field shouldn't see it bounce back into
//     the user view.
//   - Fields appear sorted by OrderIndex ascending so the consumer
//     can render the form in the same order the admin designed.
//     Ties broken by Name for deterministic ordering.
//   - Fields with no stored value get `value: null`.
func BuildResponse(fields []entity.ProfileField, data map[string]interface{}) []FieldWithValue {
	if data == nil {
		data = map[string]interface{}{}
	}
	active := make([]entity.ProfileField, 0, len(fields))
	for _, f := range fields {
		if !f.IsActive {
			continue
		}
		active = append(active, f)
	}
	sort.SliceStable(active, func(i, j int) bool {
		if active[i].OrderIndex != active[j].OrderIndex {
			return active[i].OrderIndex < active[j].OrderIndex
		}
		return active[i].Name < active[j].Name
	})

	out := make([]FieldWithValue, 0, len(active))
	for _, f := range active {
		entry := FieldWithValue{
			ID:          f.ID,
			Name:        f.Name,
			Label:       f.Label,
			Description: f.Description,
			DataType:    f.DataType,
			IsRequired:  f.IsRequired,
			MinValue:    f.MinValue,
			MaxValue:    f.MaxValue,
			Options:     f.Options,
			Regex:       f.Regex,
			Value:       nil,
		}
		if v, ok := data[f.Name]; ok {
			entry.Value = v
		}
		out = append(out, entry)
	}
	return out
}
