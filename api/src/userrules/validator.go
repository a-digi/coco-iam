package userrules

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// Validate runs every applicable validator against `in` and returns
// ordered, human-readable violation messages. An empty slice means
// everything passed.
func Validate(rs RuleSet, in Input) []string {
	var out []string
	if in.Username != "" {
		out = append(out, ValidateUsername(rs.Username, in.Username)...)
	}
	if in.Email != "" {
		out = append(out, ValidateEmail(rs.Email, in.Email)...)
	}
	if in.Password != "" {
		out = append(out, ValidatePassword(rs.Password, in)...)
	}
	return out
}

// ValidatePassword returns ordered violations for the proposed
// password. Needs username/email from `in` when the corresponding
// "disallow" rules are enabled.
func ValidatePassword(rs PasswordRules, in Input) []string {
	var out []string
	pw := in.Password
	if rs.MinLength > 0 && len(pw) < rs.MinLength {
		out = append(out, fmt.Sprintf("Password must be at least %d characters.", rs.MinLength))
	}
	if rs.MaxLength > 0 && len(pw) > rs.MaxLength {
		out = append(out, fmt.Sprintf("Password must be at most %d characters.", rs.MaxLength))
	}
	if rs.RequireUpper && !hasFunc(pw, unicode.IsUpper) {
		out = append(out, "Password must contain at least one uppercase letter.")
	}
	if rs.RequireLower && !hasFunc(pw, unicode.IsLower) {
		out = append(out, "Password must contain at least one lowercase letter.")
	}
	if rs.RequireDigit && !hasFunc(pw, unicode.IsDigit) {
		out = append(out, "Password must contain at least one digit.")
	}
	if rs.RequireSpecial && !hasSpecial(pw) {
		out = append(out, "Password must contain at least one special character.")
	}
	if rs.DisallowUsername && in.Username != "" && containsFold(pw, in.Username) {
		out = append(out, "Password must not contain the username.")
	}
	if rs.DisallowEmail && in.Email != "" {
		local := emailLocalPart(in.Email)
		if local != "" && containsFold(pw, local) {
			out = append(out, "Password must not contain the email address.")
		}
	}
	return out
}

// ValidateUsername checks length, regex, and the reserved list.
func ValidateUsername(rs UsernameRules, username string) []string {
	var out []string
	u := strings.TrimSpace(username)
	if rs.MinLength > 0 && len(u) < rs.MinLength {
		out = append(out, fmt.Sprintf("Username must be at least %d characters.", rs.MinLength))
	}
	if rs.MaxLength > 0 && len(u) > rs.MaxLength {
		out = append(out, fmt.Sprintf("Username must be at most %d characters.", rs.MaxLength))
	}
	if rs.Regex != "" {
		re, err := regexp.Compile(rs.Regex)
		if err == nil && !re.MatchString(u) {
			out = append(out, "Username contains characters that are not allowed.")
		}
		// A bad regex in configuration silently degrades to "no regex"
		// — we'd rather let users through than lock everyone out over
		// a broken admin setting.
	}
	for _, r := range rs.Reserved {
		if strings.EqualFold(u, r) {
			out = append(out, fmt.Sprintf("Username %q is reserved.", u))
			break
		}
	}
	return out
}

// ValidateEmail checks domain allow/block lists. Syntactic validity
// is out of scope — we trust the form to prevent `not-an-email`.
func ValidateEmail(rs EmailRules, email string) []string {
	var out []string
	domain := emailDomain(email)
	if domain == "" {
		out = append(out, "Email address is not valid.")
		return out
	}
	for _, b := range rs.BlockedDomains {
		if strings.EqualFold(domain, b) {
			out = append(out, fmt.Sprintf("Email domain %q is not permitted.", domain))
			return out
		}
	}
	if len(rs.AllowedDomains) > 0 {
		allowed := false
		for _, a := range rs.AllowedDomains {
			if strings.EqualFold(domain, a) {
				allowed = true
				break
			}
		}
		if !allowed {
			out = append(out, fmt.Sprintf("Email domain %q is not permitted.", domain))
		}
	}
	return out
}

// --- helpers ---

func hasFunc(s string, f func(rune) bool) bool {
	for _, r := range s {
		if f(r) {
			return true
		}
	}
	return false
}

func hasSpecial(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) {
			continue
		}
		return true
	}
	return false
}

func containsFold(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

func emailLocalPart(email string) string {
	at := strings.IndexByte(email, '@')
	if at <= 0 {
		return ""
	}
	return email[:at]
}

func emailDomain(email string) string {
	at := strings.IndexByte(email, '@')
	if at < 0 || at == len(email)-1 {
		return ""
	}
	return email[at+1:]
}
