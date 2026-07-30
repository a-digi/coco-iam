package entity

import oauth_model "github.com/a-digi/coco-oauth/oauth/model"

// CreateUserRequest is the request body for POST /admin/{res:users}.
type CreateUserRequest struct {
	Username     string `json:"username"      example:"jdoe"`
	Email        string `json:"email"         example:"jdoe@example.com"`
	IsActive     bool   `json:"is_active"     example:"true"`
	IsSuperAdmin bool   `json:"is_super_admin" example:"false"`
}

// CreateUserResponse is the response payload for POST /admin/{res:users}.
type CreateUserResponse struct {
	User            *User           `json:"user"`
	Activation      *ActivationEcho `json:"activation,omitempty"`
	ActivationError string          `json:"activation_error,omitempty"`
}

// ActivationEcho confirms that an activation email was enqueued.
type ActivationEcho struct {
	ExpiresAt string `json:"expires_at" example:"2025-01-01T00:00:00Z"`
}

// UpdateUserRequest is the partial-update body for PATCH /admin/{res:users}/{id}.
// All fields are optional — only provided fields are applied. Password changes
// are handled separately — see ResetPasswordRequest (admin-privileged reset)
// and the self-service account/password/change flow.
type UpdateUserRequest struct {
	Email        *string `json:"email,omitempty"         example:"new@example.com"`
	IsActive     *bool   `json:"is_active,omitempty"     example:"true"`
	IsSuperAdmin *bool   `json:"is_super_admin,omitempty" example:"false"`
}

// ResetPasswordRequest is the body for POST /admin/users/{id}/reset-password —
// an admin-privileged reset that does not require the target user's current
// password (the privilege boundary is the route's own scope check).
type ResetPasswordRequest struct {
	NewPassword string `json:"new_password" example:"a-new-strong-password"`
}

// ResetPasswordResponse confirms the reset succeeded.
type ResetPasswordResponse struct {
	Ok bool `json:"ok" example:"true"`
}

// SendActivationRequest has no body — activation is triggered by the path param alone.
type SendActivationResult struct {
	Status    string `json:"status"     example:"sent"`
	ExpiresAt string `json:"expires_at" example:"2025-01-01T00:00:00Z"`
}

// Swag-friendly success envelopes for admin user endpoints.

// LoginSuccess is the swag-friendly success envelope for the admin login endpoint.
type LoginSuccess struct {
	Success bool                      `json:"success" example:"true"`
	Message oauth_model.TokenResponse `json:"message"`
}

type UserSuccess struct {
	Success bool  `json:"success" example:"true"`
	Message *User `json:"message"`
}

type CreateUserSuccess struct {
	Success bool               `json:"success" example:"true"`
	Message CreateUserResponse `json:"message"`
}

type SendActivationSuccess struct {
	Success bool                 `json:"success" example:"true"`
	Message SendActivationResult `json:"message"`
}

type ResetPasswordSuccess struct {
	Success bool                  `json:"success" example:"true"`
	Message ResetPasswordResponse `json:"message"`
}
