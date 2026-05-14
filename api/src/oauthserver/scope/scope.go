// Package scope parses and filters OAuth 2.0 / OIDC scope
// strings. Scopes on the wire are space-separated per RFC 6749;
// we keep them as []string internally so the rest of the codebase
// never has to split a string twice.
//
// Standard OIDC scopes (openid, profile, email, offline_access)
// trigger well-known behaviours; everything else is treated as
// an application-defined scope and passed through verbatim.
package scope

import "strings"

// Standard OIDC + OAuth 2.0 scope names. Exported so handlers
// that care about the OIDC-specific behaviour can reference them
// without re-typing the strings.
const (
	ScopeOpenID        = "openid"
	ScopeProfile       = "profile"
	ScopeEmail         = "email"
	ScopeOfflineAccess = "offline_access"
)

// Parse splits a space-separated scope string into its parts.
// Empty strings collapse to nil. Duplicates are de-duplicated
// while preserving first-seen order (the wire order matters for
// some downstream display choices).
func Parse(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	fields := strings.Fields(s)
	out := make([]string, 0, len(fields))
	seen := make(map[string]struct{}, len(fields))
	for _, f := range fields {
		if _, dup := seen[f]; dup {
			continue
		}
		seen[f] = struct{}{}
		out = append(out, f)
	}
	return out
}

// Join writes scopes back to the RFC 6749 wire form.
func Join(scopes []string) string {
	return strings.Join(scopes, " ")
}

// Contains reports whether a scope list mentions target.
// Case-sensitive comparison per RFC 6749 §3.3.
func Contains(scopes []string, target string) bool {
	for _, s := range scopes {
		if s == target {
			return true
		}
	}
	return false
}

// FilterAllowed returns the scopes from requested that also
// appear in allowed. Preserves the order of requested so the
// issued token's scope claim matches what the caller asked for.
func FilterAllowed(requested, allowed []string) []string {
	if len(allowed) == 0 {
		return nil
	}
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, a := range allowed {
		allowedSet[a] = struct{}{}
	}
	out := make([]string, 0, len(requested))
	for _, r := range requested {
		if _, ok := allowedSet[r]; ok {
			out = append(out, r)
		}
	}
	return out
}

// IsSubset reports whether every scope in subset also appears
// in superset. Used by the consent-reuse logic: a cached
// consent is reusable only if it covers everything the current
// request asks for.
func IsSubset(subset, superset []string) bool {
	set := make(map[string]struct{}, len(superset))
	for _, s := range superset {
		set[s] = struct{}{}
	}
	for _, s := range subset {
		if _, ok := set[s]; !ok {
			return false
		}
	}
	return true
}

// TriggersOIDC reports whether the scopes include "openid",
// which is what asks us to issue an id_token.
func TriggersOIDC(scopes []string) bool {
	return Contains(scopes, ScopeOpenID)
}

// TriggersOfflineAccess reports whether the scopes include
// "offline_access", which is what asks us to issue a refresh
// token alongside the access token.
func TriggersOfflineAccess(scopes []string) bool {
	return Contains(scopes, ScopeOfflineAccess)
}

// ClaimsFor returns the OIDC claim names that should be
// populated on the id_token for the requested scopes. See
// OIDC Core §5.4. Unrecognised scopes are ignored here —
// application-defined scopes populate claims only if the
// caller's UserClaimsReader knows how to handle them.
func ClaimsFor(scopes []string) []string {
	claims := []string{}
	for _, s := range scopes {
		switch s {
		case ScopeProfile:
			claims = append(claims,
				"name", "family_name", "given_name", "middle_name",
				"nickname", "preferred_username", "profile", "picture",
				"website", "gender", "birthdate", "zoneinfo", "locale",
				"updated_at",
			)
		case ScopeEmail:
			claims = append(claims, "email", "email_verified")
		}
	}
	return claims
}
