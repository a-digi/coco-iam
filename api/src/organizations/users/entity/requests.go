package entity

// CreateOrgUserRequest is the body for POST /admin/{res:organization_users}.
type CreateOrgUserRequest struct {
	Username              string `json:"username"                         example:"jdoe"`
	Email                 string `json:"email"                            example:"jdoe@example.com"`
	OrganizationID        string `json:"organization_id"                  example:"org-uuid"`
	IsActive              bool   `json:"is_active"                        example:"true"`
	RedirectApplicationID string `json:"redirect_application_id,omitempty" example:"app-uuid"`
}

// CreateOrgUserResponse is the success payload for POST /admin/{res:organization_users}.
type CreateOrgUserResponse struct {
	User            *User           `json:"user"`
	Activation      *OrgActivationEcho `json:"activation,omitempty"`
	ActivationError string          `json:"activation_error,omitempty"`
}

// OrgActivationEcho confirms the activation email was enqueued.
type OrgActivationEcho struct {
	ExpiresAt string `json:"expires_at" example:"2025-01-01T00:00:00Z"`
}

// PatchOrgUserRequest is the body for PATCH /admin/{res:organization_users}/{id}.
type PatchOrgUserRequest struct {
	Email    *string `json:"email,omitempty"     example:"new@example.com"`
	IsActive *bool   `json:"is_active,omitempty" example:"true"`
}

// OrgUserResponse wraps User with a computed activation_pending field.
type OrgUserResponse struct {
	User
	ActivationPending bool `json:"activation_pending" example:"false"`
}

// Swag-friendly success envelopes for org user endpoints.

type OrgUserSuccess struct {
	Success bool            `json:"success" example:"true"`
	Message OrgUserResponse `json:"message"`
}

type OrgUserListSuccess struct {
	Success bool              `json:"success" example:"true"`
	Message []OrgUserResponse `json:"message"`
}

type CreateOrgUserSuccess struct {
	Success bool                  `json:"success" example:"true"`
	Message CreateOrgUserResponse `json:"message"`
}
