// Package userrules owns the configurable validation rules applied to
// usernames, emails, and passwords. Two independent rule sets live
// here: a single admin-wide set and one per organization.
package userrules

// ContextBagKey under which the Store is registered so handlers and
// other services can resolve it.
const ContextBagKeyStore = "userrules.Store"

// Scope values used in the storage layer.
const (
	ScopeAdmin        = "admin"
	ScopeOrganization = "organization"
	// AdminOwnerID is the sentinel owner_id used for admin-scope rows.
	// There is exactly one admin rule set, so the value is constant.
	AdminOwnerID = "admin"
)

// RuleSet bundles the three rule categories. Stored as JSON in
// `user_rule_sets.rules_json`.
type RuleSet struct {
	Password PasswordRules `json:"password"`
	Username UsernameRules `json:"username"`
	Email    EmailRules    `json:"email"`
}

// PasswordRules describes the shape a new password must have. Zero
// values are the "don't care" signal for booleans; MaxLength == 0 means
// no upper bound.
type PasswordRules struct {
	MinLength        int  `json:"min_length"`
	MaxLength        int  `json:"max_length"`
	RequireUpper     bool `json:"require_upper"`
	RequireLower     bool `json:"require_lower"`
	RequireDigit     bool `json:"require_digit"`
	RequireSpecial   bool `json:"require_special"`
	DisallowUsername bool  `json:"disallow_username"`
	DisallowEmail    bool  `json:"disallow_email"`
	ExpiryDays       int   `json:"expiry_days"`
	NotifyDays       []int `json:"notify_days"`
}

// UsernameRules describes a valid username. Regex is a Go
// regexp-compatible pattern anchored with ^ / $.
type UsernameRules struct {
	MinLength int      `json:"min_length"`
	MaxLength int      `json:"max_length"`
	Regex     string   `json:"regex"`
	Reserved  []string `json:"reserved"`
}

// EmailRules describes which email addresses are acceptable. Domain
// matching is case-insensitive. An empty AllowedDomains list means
// "any domain"; BlockedDomains always wins over AllowedDomains.
type EmailRules struct {
	AllowedDomains []string `json:"allowed_domains"`
	BlockedDomains []string `json:"blocked_domains"`
}

// Input bundles the fields a caller wants validated. Any empty field
// is skipped — callers compose exactly what they're changing.
type Input struct {
	Username        string
	Email           string
	Password        string
	CurrentPassword string // only used when DisallowUsername / DisallowEmail need context
}
