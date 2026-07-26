// Package handler serves the admin IP ban/allowlist management API
// under /api/v1/admin/security/*. See plan/ip-abuse-protection/plan.md
// sections 2, 4, 5, 9.
//
// Unlike most admin handlers in this codebase, these do not construct
// their own repositories from a raw *sql.DB. The in-memory ban/
// allowlist cache inside IPGuardSecurityLayer is a genuine
// shared-mutable singleton — writing to SQLite directly here (as
// admin/mfa's handlers do) would leave a manual unban invisible to the
// live enforcement path until the process restarts. So every handler
// resolves the same running IPGuardSecurityLayer instance and calls
// its methods, which update memory and persistence together.
package handler

import (
	"net"
	"net/http"

	"github.com/a-digi/coco-iam/config/di"
	jwt_token "github.com/a-digi/coco-iam/src/auth/security/jwt"
	"github.com/a-digi/coco-iam/src/security/ipguard"
	"github.com/a-digi/coco-oauth/oauth"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// resolveIPGuard returns the shared IPGuardSecurityLayer instance —
// the same one enforcing every live request — so a manual ban/unban
// here takes effect immediately. Writes a 500 response itself and
// returns ok=false if unavailable, matching the resolveStore-style
// convention used elsewhere (e.g.
// api/src/admin/mail/accounts/delete_handler.go's resolveSettingsStore).
func resolveIPGuard(reqCtx request.RequestContext) (*ipguard.IPGuardSecurityLayer, bool) {
	w := reqCtx.GetWriter()
	bag, ok := reqCtx.GetDI().(*di.ContextBag)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "DI context has unexpected type")
		return nil, false
	}
	guard := bag.GetIPGuard()
	if guard == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "ip guard not available")
		return nil, false
	}
	return guard, true
}

// callerAdminUserID extracts admin_user_id from the caller's bearer
// token, to record who created a manual ban/allowlist entry. Returns
// "" if it can't be determined — the caller already holds a valid
// admin:security scope by the time a handler runs, so a missing
// subject here means "unknown creator", not an auth failure.
func callerAdminUserID(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if header == "" {
		return ""
	}
	token, err := oauth.ExtractBearer(header)
	if err != nil {
		return ""
	}
	userID, err := jwt_token.ParseJWTSubject(token)
	if err != nil {
		return ""
	}
	return userID
}

// validateIP reports whether ip is a syntactically valid IPv4/IPv6
// address. Every ban/allowlist mutation passes through this before
// reaching IPGuardSecurityLayer — keeps the stored data consistent
// with what ClientIP() can ever actually produce, and is the same
// discipline plan/ip-abuse-protection/plan.md section 15 requires
// before any value reaches an OS firewall command.
func validateIP(ip string) bool {
	return net.ParseIP(ip) != nil
}
