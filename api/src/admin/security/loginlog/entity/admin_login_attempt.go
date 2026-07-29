// Package entity holds the request/response types for the admin
// login-attempt audit log. See plan/login-audit-log/plan.md.
package entity

// AdminLoginAttempt is one admin-console login attempt (success or
// failure) against /admin/oauth/authenticate or /admin/oauth/verify-mfa.
// Lives in the separate admin_login.db, not the main DB. Passwords are
// never stored here.
type AdminLoginAttempt struct {
	ID string `json:"id" example:"b1f6c9e2-1234-4a5b-8c9d-abcdef012345"`
	// AdminUserID is empty when the typed username never resolved to a
	// real admin account.
	AdminUserID string `json:"admin_user_id,omitempty" example:"793424dd-7913-4190-be88-b928559ab4ef"`
	Username    string `json:"username" example:"jdoe"`
	Success     bool   `json:"success" example:"false"`
	// FailureReason is one of invalid_credentials, inactive_user,
	// no_scopes, mfa_required, mfa_failed — empty on success.
	// mfa_required is a provisional outcome (password was correct,
	// MFA verification still pending), not a hard failure.
	FailureReason string `json:"failure_reason,omitempty" example:"invalid_credentials"`
	IP            string `json:"ip" example:"203.0.113.7"`
	UserAgent     string `json:"user_agent,omitempty" example:"Mozilla/5.0 ..."`
	CreatedAt     string `json:"created_at" example:"2026-07-29T20:41:00Z"`
}

// AdminLoginAttemptListResponse is the list endpoint's payload — Total
// is the filtered row count (ignoring Limit/Offset), for the admin
// page's pagination.
type AdminLoginAttemptListResponse struct {
	Attempts []AdminLoginAttempt `json:"attempts"`
	Total    int                 `json:"total" example:"128"`
	Limit    int                 `json:"limit" example:"50"`
	Offset   int                 `json:"offset" example:"0"`
}

// Swag-friendly success envelope.

type AdminLoginAttemptListSuccess struct {
	Success bool                          `json:"success" example:"true"`
	Message AdminLoginAttemptListResponse `json:"message"`
}
