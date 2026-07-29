// Package loginlog records admin-console login attempts into
// admin_login.db. See plan/login-audit-log/plan.md Step 3. Record is
// the single entry point both /admin/oauth/authenticate and
// /admin/oauth/verify-mfa call at each terminal outcome — deliberately
// best-effort: a login-log write failure is logged and swallowed,
// never surfaced as a failure of the login itself.
package loginlog

import (
	"github.com/a-digi/coco-iam/config/di"
	loginlog_persistent "github.com/a-digi/coco-iam/src/admin/security/loginlog/repository/persistent"
	"github.com/a-digi/coco-server/server/request"
)

// Record persists one admin login attempt. adminUserID is empty when
// the typed username never resolved to a real admin account.
// failureReason is one of invalid_credentials, inactive_user,
// no_scopes, mfa_required, mfa_failed, or empty on success. IP is
// resolved from reqCtx's own request via the shared
// IPGuardSecurityLayer trust-header chain — see
// plan/attack-ip-attribution/plan.md — rather than a second
// IP-resolution path.
func Record(reqCtx request.RequestContext, adminUserID, username string, success bool, failureReason string) {
	bag, ok := reqCtx.GetDI().(*di.ContextBag)
	if !ok {
		return
	}
	handle := bag.GetAdminLoginHandle()
	if handle == nil {
		return
	}

	r := reqCtx.GetRequest()
	var ip string
	if guard := bag.GetIPGuard(); guard != nil {
		ip = guard.ClientIP(r)
	}

	repo := loginlog_persistent.NewAdminLoginPersistentRepo(handle)
	if err := repo.RecordAttempt(adminUserID, username, success, failureReason, ip, r.UserAgent()); err != nil {
		if log := bag.GetLogger(); log != nil {
			log.Warning("loginlog: failed to record admin login attempt: %v", err)
		}
	}
}
