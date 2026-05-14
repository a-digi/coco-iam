package scopecheck

import (
	"net/http"

	user_query "github.com/a-digi/coco-iam/src/admin/users/repository/query"
	"github.com/a-digi/coco-iam/src/auth/security/jwt"
	"github.com/a-digi/coco-orm/orm"
)

// SuperAdminScope is the scope id that always passes the checker.
// Kept local (not imported from jwt.SuperAdmin as Scope type) so callers can
// compare plain strings without coercion.
const SuperAdminScope = "super:admin"

// Checker inspects the caller's JWT-derived scopes.
// Stateless — safe to share between goroutines.
type Checker struct{}

// NewChecker returns a ready-to-use Checker.
func NewChecker() *Checker {
	return &Checker{}
}

// HasScope returns true if the caller's JWT carries the given scope,
// or if the caller holds super:admin (global pass-through).
func (c *Checker) HasScope(headers http.Header, scope string) (bool, error) {
	scopes, err := extractScopes(headers)
	if err != nil {
		return false, err
	}
	for _, s := range scopes {
		if s == SuperAdminScope || s == scope {
			return true, nil
		}
	}
	return false, nil
}

// HasAnyScope returns true if the caller holds at least one of the listed
// scopes, or super:admin.
func (c *Checker) HasAnyScope(headers http.Header, scopes ...string) (bool, error) {
	userScopes, err := extractScopes(headers)
	if err != nil {
		return false, err
	}
	required := make(map[string]struct{}, len(scopes))
	for _, s := range scopes {
		required[s] = struct{}{}
	}
	for _, s := range userScopes {
		if s == SuperAdminScope {
			return true, nil
		}
		if _, ok := required[s]; ok {
			return true, nil
		}
	}
	return false, nil
}

// HasAllScopes returns true only if the caller holds every listed scope
// (AND-composition). super:admin satisfies the check unconditionally.
func (c *Checker) HasAllScopes(headers http.Header, scopes ...string) (bool, error) {
	userScopes, err := extractScopes(headers)
	if err != nil {
		return false, err
	}
	userSet := make(map[string]struct{}, len(userScopes))
	for _, s := range userScopes {
		if s == SuperAdminScope {
			return true, nil
		}
		userSet[s] = struct{}{}
	}
	for _, s := range scopes {
		if _, ok := userSet[s]; !ok {
			return false, nil
		}
	}
	return true, nil
}

// HasScopeFromDB queries the live ACL (user_acl + user_group_acl via
// user_group_members) for the given user. Use when revocation latency
// matters — JWT claims are snapshotted at login and won't reflect
// recently revoked scopes until the token is renewed.
func (c *Checker) HasScopeFromDB(manager *orm.DatabaseManager, userID, scope string) (bool, error) {
	repo := user_query.NewAdminUserQueryRepository(manager)
	acl, err := repo.GetMeAcl(userID)
	if err != nil {
		return false, err
	}
	for _, s := range acl.DirectAcl {
		if s == SuperAdminScope || s == scope {
			return true, nil
		}
	}
	for _, s := range acl.InheritedAcl {
		if s == SuperAdminScope || s == scope {
			return true, nil
		}
	}
	return false, nil
}

func extractScopes(headers http.Header) ([]string, error) {
	payload, err := jwt.CreateJWTTokenFromHeaders(headers)
	if err != nil {
		return nil, err
	}
	return payload.Scopes, nil
}
