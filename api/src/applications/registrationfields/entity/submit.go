package entity

// RegisterRequest is the public registration form submission body.
// Fields keys: "email" and "username" are literal keys for the identity
// fields; all other keys are application_registration_fields UUIDs.
type RegisterRequest struct {
	Fields map[string]string `json:"fields"`
}

// RegisterSuccess is returned for every accepted registration submission.
// The same body is used for both the new-user and existing-user paths to
// prevent the response from being used to enumerate registered emails.
type RegisterSuccess struct {
	Message string `json:"message"`
}
