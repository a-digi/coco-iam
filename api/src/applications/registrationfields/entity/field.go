package entity

import "time"

// FieldSource discriminates how a registration field gets its
// definition: 'profile' means it's linked back to a profile_fields
// row (avoids re-defining data the org already declared); 'custom'
// means an inline one-off definition.
type FieldSource string

const (
	FieldSourceProfile FieldSource = "profile"
	FieldSourceCustom  FieldSource = "custom"
	FieldSourceSystem  FieldSource = "system"
)

// IsValid returns true for the three supported sources.
func (s FieldSource) IsValid() bool {
	return s == FieldSourceProfile || s == FieldSourceCustom || s == FieldSourceSystem
}

// Field mirrors one row of application_registration_fields.
//
// Columns split into two groups:
//   - Always set: ID, ApplicationID, StepID, OrderIndex, Source.
//   - Source-dependent:
//       source='profile' → ProfileFieldID (+ RequiredOverride)
//       source='custom'  → inline columns Name, Label, DataType, etc.
//
// The service layer merges a 'profile'-sourced row with its
// referenced profile_fields row into an EffectiveField before the
// public endpoint serialises it.
type Field struct {
	ID               string      `json:"id"`
	ApplicationID    string      `json:"application_id"`
	StepID           string      `json:"step_id"`
	OrderIndex       int         `json:"order_index"`
	Source           FieldSource `json:"source"`
	ProfileFieldID   *string     `json:"profile_field_id,omitempty"`
	RequiredOverride *bool       `json:"required_override,omitempty"`

	// Inline definition — populated when Source == FieldSourceCustom.
	Name        string  `json:"name,omitempty"`
	Label       string  `json:"label,omitempty"`
	Description string  `json:"description,omitempty"`
	DataType    string  `json:"data_type,omitempty"`
	IsRequired  bool    `json:"is_required"`
	MinValue    *int    `json:"min_value,omitempty"`
	MaxValue    *int    `json:"max_value,omitempty"`
	OptionsJSON string  `json:"options_json,omitempty"`
	Regex       string  `json:"regex,omitempty"`

	// SystemFieldName is set when Source == FieldSourceSystem.
	// Valid values: "email", "username".
	SystemFieldName *string `json:"system_field_name,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
