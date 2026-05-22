// Package httputil provides shared response envelope types for swag annotations.
// These types mirror the JSON shapes produced by response.SuccessResponse so that
// the generated OpenAPI spec accurately reflects the wire format.
// Handlers do not need to import this package — it exists solely for documentation.
package httputil

// ResendResult is the response payload for activation resend endpoints.
type ResendResult struct {
	Status    string `json:"status"     example:"sent"`
	ExpiresAt string `json:"expires_at" example:"2025-01-01T00:00:00Z"`
}

// ResendSuccess is the swag-friendly success envelope for resend endpoints.
type ResendSuccess struct {
	Success bool         `json:"success" example:"true"`
	Message ResendResult `json:"message"`
}
