// Package entity holds the request/response types for admin TOTP MFA
// self-service (enroll/confirm/status/disable/regenerate) and the
// login-time verification step. See plan/admin-mfa-totp/plan.md.
package entity

// MfaStatus is the current admin's TOTP enrollment state.
type MfaStatus struct {
	Enabled                bool   `json:"enabled"                  example:"true"`
	EnrolledAt             string `json:"enrolled_at,omitempty"     example:"2026-07-25T12:00:00Z"`
	RecoveryCodesRemaining int    `json:"recovery_codes_remaining" example:"8"`
}

// MfaEnrollResponse is returned once, at enrollment start. The secret
// is never re-served after this — only the QR/URI is shown again if
// the admin re-enrolls before confirming.
type MfaEnrollResponse struct {
	Secret          string `json:"secret"           example:"K7PNOKLVPJ24AFFK5J2U4QSMJTAUXZHU"`
	ProvisioningURI string `json:"provisioning_uri" example:"otpauth://totp/coco-iam:admin@example.com?secret=K7PNOKLVPJ24AFFK5J2U4QSMJTAUXZHU&issuer=coco-iam"`
}

// MfaRecoveryCodesResponse is returned once — after /confirm succeeds,
// or after a recovery-codes regenerate call. Codes are stored only as
// bcrypt hashes; this is the only time the plaintext is ever visible.
type MfaRecoveryCodesResponse struct {
	RecoveryCodes []string `json:"recovery_codes" example:"7K9QX-2MN4P,HRDR7-J9Y6B"`
}

// MfaCodeRequest is the body for /confirm and /verify-mfa — a 6-digit
// TOTP code, or (verify-mfa only) an unused recovery code.
type MfaCodeRequest struct {
	Code string `json:"code" example:"123456"`
}

// MfaDisableRequest is the body for disabling MFA or regenerating
// recovery codes — the admin's current password, re-verified inline
// so a hijacked session can't silently turn off the second factor.
type MfaDisableRequest struct {
	Password string `json:"password" example:"CurrentPassw0rd!"`
}

// MfaRequiredResponse replaces the normal token response from the
// login endpoint when the admin has MFA enabled — a short-lived,
// narrowly-scoped (system:mfa_required) token that only /verify-mfa
// will accept.
type MfaRequiredResponse struct {
	MfaRequired bool   `json:"mfa_required" example:"true"`
	MfaToken    string `json:"mfa_token"`
	ExpiresAt   int64  `json:"expires_at" example:"1784980000"`
}

// StatusResponse is a minimal {status: "..."} payload for actions that
// don't otherwise return a resource (e.g. disabling MFA).
type StatusResponse struct {
	Status string `json:"status" example:"disabled"`
}

// Swag-friendly success envelopes.

type MfaStatusSuccess struct {
	Success bool      `json:"success" example:"true"`
	Message MfaStatus `json:"message"`
}

type MfaEnrollSuccess struct {
	Success bool              `json:"success" example:"true"`
	Message MfaEnrollResponse `json:"message"`
}

type MfaRecoveryCodesSuccess struct {
	Success bool                     `json:"success" example:"true"`
	Message MfaRecoveryCodesResponse `json:"message"`
}

type StatusSuccess struct {
	Success bool           `json:"success" example:"true"`
	Message StatusResponse `json:"message"`
}

// MfaRequiredSuccess is the response envelope the admin login
// endpoint returns (HTTP 202) instead of entity.LoginSuccess when the
// admin has MFA enabled.
type MfaRequiredSuccess struct {
	Success bool                `json:"success" example:"true"`
	Message MfaRequiredResponse `json:"message"`
}
