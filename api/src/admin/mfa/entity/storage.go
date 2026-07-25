package entity

import "time"

// AdminUserMfa mirrors the admin_user_mfa row shape — internal
// storage representation, distinct from the API-facing MfaStatus
// etc. in mfa.go. SecretEnc is secretbox-ciphertext, never decrypted
// outside the handler that needs to validate a code.
type AdminUserMfa struct {
	AdminUserID    string
	SecretEnc      string
	IsEnabled      bool
	EnrolledAt     *time.Time
	ConfirmedAt    *time.Time
	FailedAttempts int
	LockedUntil    *time.Time
}

// RecoveryCode mirrors one admin_user_mfa_recovery_codes row.
type RecoveryCode struct {
	ID       string
	CodeHash string
	UsedAt   *time.Time
}
