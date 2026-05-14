package userprofile

import (
	"github.com/a-digi/coco-iam/src/organizations/profile"
	"github.com/a-digi/coco-iam/src/organizations/profile/entity"
)

// FieldError surfaces a single per-field validation failure during a
// PATCH. Aliased to profile.FieldError so merge callers don't have
// to import the organizations/profile package.
type FieldError = profile.FieldError

// MergeProfileData applies a JSON patch onto the current profile_data
// map. Returns the merged map plus any per-field validation errors.
// Pure — no I/O — so the handler's orchestration can stay shallow and
// every branch below is trivially testable without a DB or RSA keys.
//
// Rules:
//   - Unknown keys → error. The PATCH contract is "names in the
//     body must exist in profile_fields" so a typo doesn't silently
//     get discarded.
//   - file-type keys → error. File fields are written by the upload
//     endpoint, never through JSON; accepting one here would mean
//     trusting an asset id the client chose.
//   - value == nil on a non-required field → clears the field from
//     the merged map.
//   - value == nil on a required field → "required" error (clearing
//     a required field is the same invariant as a full save leaving
//     it blank).
//   - Omitted keys keep their prior value.
//   - Known, non-nil values run through the same per-field rules
//     organizations/profile.Validate applies on full save.
func MergeProfileData(
	fields []entity.ProfileField,
	current map[string]any,
	patch map[string]any,
) (map[string]any, []FieldError) {
	fieldByName := make(map[string]entity.ProfileField, len(fields))
	for _, f := range fields {
		if !f.IsActive {
			continue
		}
		fieldByName[f.Name] = f
	}

	merged := make(map[string]any, len(current)+len(patch))
	for k, v := range current {
		merged[k] = v
	}

	var errs []FieldError
	for key, raw := range patch {
		f, ok := fieldByName[key]
		if !ok {
			errs = append(errs, FieldError{Field: key, Message: "unknown field"})
			continue
		}
		if f.DataType == entity.DataTypeFile {
			errs = append(errs, FieldError{Field: key, Message: "file fields must be set via the upload endpoint"})
			continue
		}
		if raw == nil {
			if f.IsRequired {
				errs = append(errs, FieldError{Field: key, Message: "required"})
				continue
			}
			delete(merged, key)
			continue
		}
		cleaned, fieldErrs := profile.Validate(
			[]entity.ProfileField{f},
			map[string]interface{}{key: raw},
		)
		if len(fieldErrs) > 0 {
			errs = append(errs, fieldErrs...)
			continue
		}
		if v, ok := cleaned[key]; ok {
			merged[key] = v
		}
	}
	return merged, errs
}
