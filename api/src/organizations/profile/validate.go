package profile

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/a-digi/coco-iam/src/organizations/profile/entity"
)

// FieldError is emitted by Validate per field that fails.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

var emailRegex = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// Validate checks the incoming `data` map against the active fields and
// returns a normalised map containing only values that belong to active
// fields. Any key not in the active schema is silently dropped (a
// field may have been soft-deleted after the user's last save).
func Validate(fields []entity.ProfileField, data map[string]interface{}) (map[string]interface{}, []FieldError) {
	cleaned := make(map[string]interface{}, len(fields))
	var errors []FieldError

	fieldByName := make(map[string]entity.ProfileField, len(fields))
	for _, f := range fields {
		if !f.IsActive {
			continue
		}
		fieldByName[f.Name] = f
	}

	for _, f := range fields {
		if !f.IsActive {
			continue
		}
		raw, present := data[f.Name]
		missing := !present || raw == nil || isEmpty(raw)

		if missing {
			if f.IsRequired {
				errors = append(errors, FieldError{Field: f.Name, Message: "required"})
			}
			continue
		}

		value, err := validateField(f, raw)
		if err != nil {
			errors = append(errors, FieldError{Field: f.Name, Message: err.Error()})
			continue
		}
		cleaned[f.Name] = value
	}

	return cleaned, errors
}

func isEmpty(raw interface{}) bool {
	switch v := raw.(type) {
	case string:
		return strings.TrimSpace(v) == ""
	case []interface{}:
		return len(v) == 0
	}
	return false
}

func validateField(f entity.ProfileField, raw interface{}) (interface{}, error) {
	switch f.DataType {
	case entity.DataTypeText, entity.DataTypeLongText:
		return validateString(f, raw)
	case entity.DataTypeNumber:
		return validateNumber(f, raw)
	case entity.DataTypeDate:
		return validateDate(f, raw)
	case entity.DataTypeEmail:
		return validateEmail(f, raw)
	case entity.DataTypeURL:
		return validateURL(f, raw)
	case entity.DataTypeSelect, entity.DataTypeChoice:
		return validateSelect(f, raw)
	case entity.DataTypeMultiple:
		return validateMultiple(f, raw)
	case entity.DataTypeFile:
		// File-type fields are written exclusively by the media-backed
		// upload handler, never through the JSON patch path. The PATCH
		// handler rejects file-type keys before reaching Validate, so
		// this branch is only touched if callers pass through an
		// already-stored asset id (e.g. a save that echoes the current
		// profile_data). Treat it as an opaque pass-through — real
		// content validation lives in the media service.
		if s, ok := raw.(string); ok {
			return strings.TrimSpace(s), nil
		}
		return nil, fmt.Errorf("must be a string asset id")
	}
	return nil, fmt.Errorf("unsupported data type: %s", f.DataType)
}

func validateString(f entity.ProfileField, raw interface{}) (string, error) {
	s, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("must be a string")
	}
	s = strings.TrimSpace(s)

	if f.MinValue != nil && len(s) < *f.MinValue {
		return "", fmt.Errorf("must be at least %d characters", *f.MinValue)
	}
	if f.MaxValue != nil && len(s) > *f.MaxValue {
		return "", fmt.Errorf("must be at most %d characters", *f.MaxValue)
	}
	if f.Regex != "" {
		re, err := regexp.Compile(f.Regex)
		if err != nil {
			return "", fmt.Errorf("field regex is invalid on server side")
		}
		if !re.MatchString(s) {
			return "", fmt.Errorf("does not match required pattern")
		}
	}
	return s, nil
}

func validateNumber(f entity.ProfileField, raw interface{}) (float64, error) {
	var num float64
	switch v := raw.(type) {
	case float64:
		num = v
	case int:
		num = float64(v)
	case int64:
		num = float64(v)
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			return 0, fmt.Errorf("must be a number")
		}
		num = parsed
	default:
		return 0, fmt.Errorf("must be a number")
	}

	if f.MinValue != nil && num < float64(*f.MinValue) {
		return 0, fmt.Errorf("must be >= %d", *f.MinValue)
	}
	if f.MaxValue != nil && num > float64(*f.MaxValue) {
		return 0, fmt.Errorf("must be <= %d", *f.MaxValue)
	}
	return num, nil
}

func validateDate(_ entity.ProfileField, raw interface{}) (string, error) {
	s, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("must be a date string (YYYY-MM-DD)")
	}
	if _, err := time.Parse("2006-01-02", s); err != nil {
		return "", fmt.Errorf("must be in YYYY-MM-DD format")
	}
	return s, nil
}

func validateEmail(f entity.ProfileField, raw interface{}) (string, error) {
	s, err := validateString(f, raw)
	if err != nil {
		return "", err
	}
	if !emailRegex.MatchString(s) {
		return "", fmt.Errorf("must be a valid email address")
	}
	return s, nil
}

func validateURL(f entity.ProfileField, raw interface{}) (string, error) {
	s, err := validateString(f, raw)
	if err != nil {
		return "", err
	}
	parsed, err := url.ParseRequestURI(s)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("must be a valid URL")
	}
	return s, nil
}

func validateSelect(f entity.ProfileField, raw interface{}) (string, error) {
	s, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("must be a string")
	}
	for _, opt := range f.Options {
		if opt == s {
			return s, nil
		}
	}
	return "", fmt.Errorf("must be one of the allowed options")
}

// validateMultiple accepts an array of option strings. Every entry
// must be present in the field's Options; duplicates inside the
// submission are deduplicated silently so clients can POST a tick-
// box form without having to track which boxes were already set.
// Empty array is valid when the field is non-required; when the
// field is required the top-level isEmpty check in Validate catches
// the "user unticked everything" case and emits the usual "required"
// error before this helper runs.
func validateMultiple(f entity.ProfileField, raw interface{}) ([]string, error) {
	arr, ok := raw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("must be an array of strings")
	}
	allowed := make(map[string]struct{}, len(f.Options))
	for _, opt := range f.Options {
		allowed[opt] = struct{}{}
	}
	seen := make(map[string]struct{}, len(arr))
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		s, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("every entry must be a string")
		}
		if _, valid := allowed[s]; !valid {
			return nil, fmt.Errorf("%q is not one of the allowed options", s)
		}
		if _, dup := seen[s]; dup {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out, nil
}
