package entity

// CheckRequest is the body for the registration availability check.
// Both fields are optional — omit a field to skip its check.
type CheckRequest struct {
	Email    string `json:"email,omitempty"`
	Username string `json:"username,omitempty"`
}

// FieldAvailability reports whether a single identity field is available.
type FieldAvailability struct {
	Available bool `json:"available"`
}

// CheckResponse is returned by the registration availability check endpoint.
// A key is present only when the corresponding field was supplied in the request.
type CheckResponse struct {
	Email    *FieldAvailability `json:"email,omitempty"`
	Username *FieldAvailability `json:"username,omitempty"`
}
