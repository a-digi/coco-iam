package service

import "encoding/json"

// decodeOptions turns the JSON-encoded options blob stored on both
// profile_fields and application_registration_fields into the
// string slice consumers expect on the wire.
//
// Defensive: unmarshal errors collapse to an empty slice rather
// than fail the whole response. The options list is decorative —
// consumers won't break if it's temporarily missing, whereas 500ing
// a consumer's entire registration UI over a malformed blob would
// be outsized.
func decodeOptions(raw string) []string {
	if raw == "" {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}
