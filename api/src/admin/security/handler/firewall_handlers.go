package handler

import (
	"net/http"
	"sort"
	"time"

	security_entity "github.com/a-digi/coco-iam/src/admin/security/entity"
	"github.com/a-digi/coco-iam/src/auth/scopecheck"
	"github.com/a-digi/coco-lift/resource/uri"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// FirewallResyncHandler serves POST /api/v1/admin/security/firewall/resync.
type FirewallResyncHandler struct{}

// @Summary     Resync active IP bans into the OS-level firewall
// @Description Re-applies every currently-active (non-expired) ip_bans row through the same
// @Description Ban() path a fresh ban already uses — useful after a host reboot or manual firewall
// @Description flush. Already-loaded IPs are safely re-applied, not duplicated at the DB level.
// @Tags        security
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} security_entity.FirewallResyncSuccess
// @Failure     401,403,500 {object} response.ErrorBody
// @Router      /admin/security/firewall/resync [post]
func (h *FirewallResyncHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()

	guard, ok := resolveIPGuard(reqCtx)
	if !ok {
		return
	}

	bans, err := guard.ListBans()
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	now := time.Now().UTC()
	var synced, skipped, failed int
	for _, b := range bans {
		expiresAt, err := time.Parse(time.RFC3339, b.ExpiresAt)
		if err != nil {
			failed++
			continue
		}
		remaining := expiresAt.Sub(now)
		if remaining <= 0 {
			skipped++
			continue
		}
		if err := guard.Ban(b.IP, b.Tier, b.Reason, remaining, nil); err != nil {
			failed++
			continue
		}
		synced++
	}

	response.SuccessResponse(w, http.StatusOK, security_entity.FirewallResyncResponse{
		Synced:         synced,
		SkippedExpired: skipped,
		Failed:         failed,
	})
}

// FirewallRulesHandler serves GET /api/v1/admin/security/firewall/rules.
type FirewallRulesHandler struct{}

// @Summary     List OS-level firewall ban rules
// @Description Returns the IPs currently blocked at the OS firewall level, read live from the
// @Description backend in use (iptables/pf) — informational; /admin/security/ip-bans remains the
// @Description source of truth for what should be banned.
// @Tags        security
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} security_entity.FirewallRulesSuccess
// @Failure     401,403,500 {object} response.ErrorBody
// @Router      /admin/security/firewall/rules [get]
func (h *FirewallRulesHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()

	guard, ok := resolveIPGuard(reqCtx)
	if !ok {
		return
	}

	name, _, _ := guard.FirewallStatus()
	rawRules, err := guard.ListFirewallRules()
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.SuccessResponse(w, http.StatusOK, security_entity.FirewallRulesResponse{
		Backend: name,
		Rules:   countFirewallRules(rawRules),
	})
}

// countFirewallRules aggregates a raw per-rule list (one entry per
// underlying OS rule, so a duplicated rule appears more than once)
// into distinct IPs with a count — a count above 1 is the exact
// duplicate-rule symptom this feature exists to surface. Sorted by IP
// for a stable, deterministic response.
func countFirewallRules(raw []string) []security_entity.FirewallRuleEntry {
	counts := make(map[string]int, len(raw))
	for _, ip := range raw {
		counts[ip]++
	}
	entries := make([]security_entity.FirewallRuleEntry, 0, len(counts))
	for ip, count := range counts {
		entries = append(entries, security_entity.FirewallRuleEntry{IP: ip, Count: count})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].IP < entries[j].IP })
	return entries
}

// FirewallRuleRemoveHandler serves DELETE
// /api/v1/admin/security/firewall/rules/{ip:<value>} — a manual
// removal of every OS-level rule for one IP. See this package's doc
// comment for the {key:value} path convention.
type FirewallRuleRemoveHandler struct{}

// @Summary     Remove OS-level firewall rules for one IP
// @Description Removes every OS-level rule for ip — there may be more than one if Ban() was called
// @Description repeatedly for an already-banned IP. If ip is still actively banned in
// @Description /admin/security/ip-bans, that ban row is deleted too: removing an IP from this page
// @Description means fully unbanning it, not leaving a rule that would just reappear on the next
// @Description resync. Requires both admin:security:firewall:write (route-level) and
// @Description admin:security:ipbans:write (checked here), since this can delete an ip_bans row —
// @Description mirrors IPBanAccountsHandler's pattern of an additional in-handler scope check beyond
// @Description the route's own gate.
// @Tags        security
// @Produce     json
// @Security    BearerAuth
// @Param       ip path string true "IP address, as {ip:<value>}"
// @Success     200 {object} security_entity.FirewallRuleRemoveSuccess
// @Failure     400,401,403,500 {object} response.ErrorBody
// @Router      /admin/security/firewall/rules/{ip} [delete]
func (h *FirewallRuleRemoveHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	key, ip := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if key != "ip" || ip == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "ip is required")
		return
	}

	// This action can cascade-delete an ip_bans row, so it requires
	// admin:security:ipbans:write too — not just this route's own
	// admin:security:firewall:write gate. The route-config's scopes
	// list is OR-only (any one scope satisfies it), so an AND
	// requirement has to be enforced explicitly here.
	checker := scopecheck.NewChecker()
	hasIPBansWrite, _ := checker.HasScope(r.Header, "admin:security:ipbans:write")
	hasSuperAdmin, _ := checker.HasScope(r.Header, "super:admin")
	if !hasIPBansWrite && !hasSuperAdmin {
		response.ErrorResponse(w, http.StatusForbidden, "requires admin:security:ipbans:write")
		return
	}

	guard, ok := resolveIPGuard(reqCtx)
	if !ok {
		return
	}

	removed, alsoUnbanned, err := guard.RemoveAllFirewallRules(ip)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.SuccessResponse(w, http.StatusOK, security_entity.FirewallRuleRemoveResponse{
		Removed:      removed,
		AlsoUnbanned: alsoUnbanned,
	})
}
