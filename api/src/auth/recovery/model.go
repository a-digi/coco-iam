// Package recovery implements the email-based password-recovery flow:
// request → email with link → user enters email + new password →
// server re-validates and resets. Distinct from `activation` (which
// provisions *new* accounts) and from `password` (authenticated
// self-service change).
package recovery

import "time"

// UserType discriminates which user table a recovery row targets.
type UserType string

const (
	UserTypeAdmin UserType = "admin"
	UserTypeUser  UserType = "user"
)

// IsValid returns true for the two known user types.
func (t UserType) IsValid() bool {
	return t == UserTypeAdmin || t == UserTypeUser
}

// ContextBagKey values used when wiring services in main.go.
const ContextBagKeyService = "recovery.Service"

// EventPasswordRecovery is the mail-engine event bound by admins to a
// template + account. Matches the string advertised in
// api/src/mail/settings/model.go.
const EventPasswordRecovery = "password_recovery"

// Row is the in-memory shape of a password_recoveries row.
type Row struct {
	ID         string
	UserID     string
	UserType   UserType
	TokenHash  string
	ExpiresAt  time.Time
	ConsumedAt *time.Time
	CreatedAt  time.Time
}
