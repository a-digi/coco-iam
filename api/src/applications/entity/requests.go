package entity

// ApplicationSuccess is the swag-friendly success envelope for application endpoints.
type ApplicationSuccess struct {
	Success bool         `json:"success" example:"true"`
	Message *Application `json:"message"`
}

// ApplicationListSuccess is the swag-friendly success envelope for application list endpoints.
type ApplicationListSuccess struct {
	Success bool          `json:"success" example:"true"`
	Message []Application `json:"message"`
}

// ApplicationRequest is the create/update body for application admin endpoints.
type ApplicationRequest struct {
	WorkspaceID        string `json:"workspace_id"                  example:"ws-uuid"`
	ClientID           string `json:"client_id"                     example:"my-app"`
	Title              string `json:"title"                         example:"My Application"`
	Description        string `json:"description,omitempty"         example:"App description"`
	AllowRecovery      *bool  `json:"allow_recovery,omitempty"`
	AllowRegistration  *bool  `json:"allow_registration,omitempty"`
	AllowPasswordLogin *bool  `json:"allow_password_login,omitempty"`
	RegistrationType   string `json:"registration_type,omitempty"   example:"legacy"`
	IsActive           *bool  `json:"is_active,omitempty"`
}
