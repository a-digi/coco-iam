package userrules

// Defaults returns the rule set applied when no configuration exists
// for a given scope. Safe enough that a fresh install is usable
// without the admin touching anything.
func Defaults() RuleSet {
	return RuleSet{
		Password: PasswordRules{
			MinLength:        8,
			MaxLength:        128,
			RequireUpper:     false,
			RequireLower:     false,
			RequireDigit:     false,
			RequireSpecial:   false,
			DisallowUsername: true,
			DisallowEmail:    true,
			ExpiryDays:       0,
			NotifyDays:       []int{},
		},
		Username: UsernameRules{
			MinLength: 3,
			MaxLength: 64,
			Regex:     `^[a-zA-Z0-9_.\-]+$`,
			Reserved:  []string{"root", "admin", "system"},
		},
		Email: EmailRules{
			AllowedDomains: []string{},
			BlockedDomains: []string{},
		},
	}
}
