package entity

// MyLoginAttempt is one of the calling end-user's own login attempts
// (success or failure). Unlike ApplicationLoginAttempt (the
// admin-facing shape), this omits application_user_id and username —
// both are always "the caller," so echoing them back is pure noise.
// See plan/self-service-login-log/plan.md.
type MyLoginAttempt struct {
	ID            string `json:"id" example:"b1f6c9e2-1234-4a5b-8c9d-abcdef012345"`
	Success       bool   `json:"success" example:"false"`
	FailureReason string `json:"failure_reason,omitempty" example:"invalid_credentials"`
	IP            string `json:"ip" example:"203.0.113.7"`
	UserAgent     string `json:"user_agent,omitempty" example:"Mozilla/5.0 ..."`
	CreatedAt     string `json:"created_at" example:"2026-07-29T20:41:00Z"`
}

// MyLoginAttemptListResponse is the self-service list endpoint's
// payload — Total is the filtered row count (ignoring Limit/Offset).
type MyLoginAttemptListResponse struct {
	Attempts []MyLoginAttempt `json:"attempts"`
	Total    int              `json:"total" example:"12"`
	Limit    int              `json:"limit" example:"50"`
	Offset   int              `json:"offset" example:"0"`
}

// Swag-friendly success envelope.

type MyLoginAttemptListSuccess struct {
	Success bool                       `json:"success" example:"true"`
	Message MyLoginAttemptListResponse `json:"message"`
}
