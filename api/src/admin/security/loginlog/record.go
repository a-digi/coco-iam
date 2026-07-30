// Package loginlog records admin-console login attempts into
// admin_login.db. See plan/login-audit-log/plan.md Step 3. Record is
// the single entry point both /admin/oauth/authenticate and
// /admin/oauth/verify-mfa call at each terminal outcome — deliberately
// best-effort: a login-log write failure is logged and swallowed,
// never surfaced as a failure of the login itself.
package loginlog

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/a-digi/coco-iam/config/di"
	loginlog_persistent "github.com/a-digi/coco-iam/src/admin/security/loginlog/repository/persistent"
	loginlog_query "github.com/a-digi/coco-iam/src/admin/security/loginlog/repository/query"
	"github.com/a-digi/coco-iam/src/security/dbhandle"
	"github.com/a-digi/coco-iam/src/security/geoip"
	"github.com/a-digi/coco-iam/src/security/loginbans"
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

	geoIPInfo := lookupGeoIPInfo(bag, ip)

	repo := loginlog_persistent.NewAdminLoginPersistentRepo(handle)
	if err := repo.RecordAttempt(adminUserID, username, success, failureReason, ip, r.UserAgent(), geoIPInfo); err != nil {
		if log := bag.GetLogger(); log != nil {
			log.Warning("loginlog: failed to record admin login attempt: %v", err)
		}
		return
	}

	if !success && ip != "" {
		checkAdminFailureBan(bag, handle, ip)
	}
}

// checkAdminFailureBan counts this IP's recent failed admin-login
// attempts and, if the configured threshold is crossed, bans it
// through the existing ipguard.Ban — reusing its allowlist bypass,
// firewall integration, and admin bans UI rather than building a
// second enforcement path. Best-effort, like Record itself: an error
// here is swallowed, never surfacing as a failure of the login
// attempt being recorded. See plan/login-ban-rules/plan.md.
func checkAdminFailureBan(bag *di.ContextBag, handle *dbhandle.Handle, ip string) {
	manager := bag.GetDatabaseManager()
	if manager == nil || manager.Connector == nil || manager.Connector.DB == nil {
		return
	}
	rules, err := loginbans.NewSettingsQueryRepo(manager.Connector.DB).LoadSettings()
	if err != nil || !rules.Admin.Enabled {
		return
	}

	since := time.Now().UTC().Add(-time.Duration(rules.Admin.WindowSeconds) * time.Second)
	count, err := loginlog_query.NewAdminLoginQueryRepo(handle).CountRecentFailures(ip, since)
	if err != nil || count < rules.Admin.Threshold {
		return
	}

	guard := bag.GetIPGuard()
	if guard == nil {
		return
	}
	reason := fmt.Sprintf("%d failed admin login attempts within %ds", count, rules.Admin.WindowSeconds)
	if err := guard.Ban(ip, "admin-login-failures", reason, time.Duration(rules.Admin.BanSeconds)*time.Second, nil); err != nil {
		if log := bag.GetLogger(); log != nil {
			log.Warning("loginlog: failed to ban %s after %d failed admin logins: %v", ip, count, err)
		}
	}
}

// lookupGeoIPInfo resolves ip's country/city/ISP via the shared
// geoip.Lookup already wired at boot (same instance the attack-episode
// recording path and the "Fetch GeoIP data" backfill button use), and
// marshals it into the JSON snapshot this login attempt stores.
// Returns "" if ip is loopback/private, has no GeoLite2 coverage,
// GeoIP is disabled, or the lookup fails — never surfaced as a login
// recording failure. See plan/login-log-geoip/plan.md.
func lookupGeoIPInfo(bag *di.ContextBag, ip string) string {
	if ip == "" || geoip.IsLoopbackOrPrivate(ip) {
		return ""
	}
	geo := bag.GetGeoIP()
	if geo == nil {
		return ""
	}
	info, ok := geo.Lookup(ip)
	if !ok {
		return ""
	}
	raw, err := json.Marshal(info)
	if err != nil {
		return ""
	}
	return string(raw)
}
