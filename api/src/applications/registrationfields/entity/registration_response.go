package entity

// IdentityFieldDef describes one fixed identity field required by the
// registration type (e.g. "legacy"). The frontend renders these inputs
// regardless of step configuration; the step entries with
// source="system" tell the frontend which step to render them in.
type IdentityFieldDef struct {
	Name       string `json:"name"`
	Label      string `json:"label"`
	DataType   string `json:"data_type"`
	IsRequired bool   `json:"is_required"`
}

// RegistrationFieldsSuccess is the swag envelope for the GET
// registration-fields response. Steps is typed as []interface{} here
// because the concrete service.StepWithFields type lives in a sibling
// package that imports entity (cycle). The wire shape is identical.
type RegistrationFieldsSuccess struct {
	RegistrationType string             `json:"registration_type"`
	IdentityFields   []IdentityFieldDef `json:"identity_fields"`
	Steps            []interface{}      `json:"steps"`
}
