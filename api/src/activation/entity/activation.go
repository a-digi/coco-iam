// Package activation_entity holds exported request/response types for
// activation endpoints. These types are referenced by swag annotations.
package activation_entity

import (
	oauth_model "github.com/a-digi/coco-oauth/oauth/model"
	"github.com/a-digi/coco-iam/src/userrules"
)

// ActivateRequest is the body for POST /activation/activate and POST /activation/a/activate.
type ActivateRequest struct {
	Token       string `json:"token"        example:"abc123"`
	Email       string `json:"email"        example:"user@example.com"`
	NewPassword string `json:"new_password" example:"S3cr3tP@ss"`
}

// ActivateResponse is the success payload for activation endpoints.
type ActivateResponse struct {
	Token       *oauth_model.TokenResponse `json:"token,omitempty"`
	RedirectURL string                     `json:"redirect_url,omitempty" example:"/login"`
}

// ActivateSuccess is the swag-friendly success envelope for activate endpoints.
type ActivateSuccess struct {
	Success bool            `json:"success" example:"true"`
	Message ActivateResponse `json:"message"`
}

// VerifySuccess is the swag-friendly success envelope for verify endpoints.
type VerifySuccess struct {
	Success bool          `json:"success" example:"true"`
	Message VerifyResponse `json:"message"`
}

// ChangePasswordSuccess is the swag-friendly success envelope for the change-password endpoint.
type ChangePasswordSuccess struct {
	Success bool                   `json:"success" example:"true"`
	Message ChangePasswordResponse `json:"message"`
}

// VerifyRequest is the body for POST /activation/verify and POST /activation/a/verify.
type VerifyRequest struct {
	Token string `json:"token" example:"abc123"`
	Email string `json:"email" example:"user@example.com"`
}

// VerifyResponse is the success payload for activation verify endpoints.
type VerifyResponse struct {
	Ok    bool              `json:"ok"    example:"true"`
	Rules userrules.RuleSet `json:"rules"`
}

// ChangePasswordRequest is the body for POST /auth/change-password.
type ChangePasswordRequest struct {
	NewPassword string `json:"new_password" example:"NewS3cr3t!"`
}

// ChangePasswordResponse is the success payload for POST /auth/change-password.
type ChangePasswordResponse struct {
	Token *oauth_model.TokenResponse `json:"token,omitempty"`
}
