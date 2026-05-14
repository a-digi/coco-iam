package entity

// DataType is the discriminator on ProfileField.DataType.
// Exported constants so handlers and the validator share one source of truth.
const (
	DataTypeText     = "text"
	DataTypeLongText = "long_text"
	DataTypeNumber   = "number"
	DataTypeDate     = "date"
	DataTypeEmail    = "email"
	DataTypeURL      = "url"
	DataTypeSelect   = "select"
	DataTypeChoice   = "choice"
	DataTypeMultiple = "multiple"
	DataTypeFile     = "file"
)

// AllowedDataTypes is the whitelist used by validation on write.
var AllowedDataTypes = map[string]struct{}{
	DataTypeText:     {},
	DataTypeLongText: {},
	DataTypeNumber:   {},
	DataTypeDate:     {},
	DataTypeEmail:    {},
	DataTypeURL:      {},
	DataTypeSelect:   {},
	DataTypeChoice:   {},
	DataTypeMultiple: {},
	DataTypeFile:     {},
}

// DataTypesRequiringOptions is the set of discriminators whose
// admin-side definition must carry a non-empty `options` array.
// Used by the create/update handlers so the rule lives in one
// place as new option-backed types are added.
var DataTypesRequiringOptions = map[string]struct{}{
	DataTypeSelect:   {},
	DataTypeChoice:   {},
	DataTypeMultiple: {},
}

// RequiresOptions reports whether the given type needs a non-empty
// options array to be considered well-defined.
func RequiresOptions(t string) bool {
	_, ok := DataTypesRequiringOptions[t]
	return ok
}

// IsAllowedDataType reports whether the given type is a supported MVP type.
func IsAllowedDataType(t string) bool {
	_, ok := AllowedDataTypes[t]
	return ok
}
