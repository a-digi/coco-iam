// Package activation implements the agnostic user-activation flow:
// creation → email with link + temp password → user sets a real password.
// Both admin_users and regular users share the same tokens, storage, and
// endpoints; the `UserType` discriminator is the only difference.
package activation

import "time"

// UserType discriminates which user table an activation row targets.
// Every activation belongs to exactly one.
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
const (
	ContextBagKeyService = "activation.Service"
)

// Event names for the mail engine's event→template binding.
const (
	EventAdminInvite                 = "admin_invite"
	EventUserInvite                  = "user_invite"
	EventAppRegistrationNotification = "app_registration_notification"
)

// RedirectTarget is the optional per-activation hint that tells the
// post-activation screen where the user should be sent next. When set,
// the activate handler composes `/login/a/<org>/<ws>/<client_id>` from
// these slugs and returns it as `redirect_url`. Nil → the frontend
// falls back to the default login.
type RedirectTarget struct {
	OrgSlug       string
	WorkspaceSlug string
	ClientID      string
}

// StartArgs holds the input to Service.Start — everything required to
// materialise an activation row and send the invite email.
type StartArgs struct {
	UserType UserType
	UserID   string
	Username string
	Email    string
	// OrgID is the organization UUID for UserTypeUser. When supplied,
	// the service skips the user_org_index lookup. Ignored for admins.
	OrgID string
	// Redirect, when set, stamps the activation row with a post-
	// activation login target. Optional — callers can leave it nil to
	// preserve the default "land on /login" behaviour.
	Redirect *RedirectTarget
}

// StartResult is returned to callers so they can log the temp password
// (useful in dev without a mail server) or surface a copy-paste link.
type StartResult struct {
	ActivationID string
	Token        string    // plaintext — only usable in the API response of admin flows
	TempPassword string    // plaintext — communicated to the creator
	ExpiresAt    time.Time
}

// ActivateResult is produced when a valid token + new-password pair is
// submitted. Callers mint a JWT from this so the user is immediately
// logged in.
type ActivateResult struct {
	UserType UserType
	UserID   string
	Username string
	Email    string
	// RedirectURL is the composed per-application login path
	// (`/login/a/<org>/<ws>/<client_id>`) when the activation row
	// carried a target, otherwise empty. The handler surfaces it as
	// `redirect_url` in the response; empty → the client uses its
	// default destination.
	RedirectURL string
}
