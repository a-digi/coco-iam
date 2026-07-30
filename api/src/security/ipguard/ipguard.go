package ipguard

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/a-digi/coco-logger/logger"
	"github.com/a-digi/coco-server/server/di"
	"github.com/a-digi/coco-server/server/security"
	"github.com/google/uuid"

	attacks_persistent "github.com/a-digi/coco-iam/src/admin/security/attacks/repository/persistent"
	security_entity "github.com/a-digi/coco-iam/src/admin/security/entity"
	security_persistent "github.com/a-digi/coco-iam/src/admin/security/repository/persistent"
	security_query "github.com/a-digi/coco-iam/src/admin/security/repository/query"
	"github.com/a-digi/coco-iam/src/security/attackbans"
	"github.com/a-digi/coco-iam/src/security/dbhandle"
	"github.com/a-digi/coco-iam/src/security/geoip"
	"github.com/a-digi/coco-iam/src/security/ipguard/firewall"
)

// timeLayout is the format every ip_bans/ip_allowlist/ip_attacks
// timestamp is written and read back in — matches both SQLite's
// CURRENT_TIMESTAMP default and each persistent repo's own UTC
// formatting.
const timeLayout = "2006-01-02 15:04:05"

type banEntry struct {
	tier      string
	reason    string
	expiresAt time.Time
}

// attackState is one in-memory, currently-open attack episode. Always
// accessed under IPGuardSecurityLayer.attacksMu — it has no lock of
// its own, since every access already happens while that lock is
// held.
type attackState struct {
	attackID     string // "" until the DB row has been created
	tier         string
	startedAt    time.Time
	lastSeenAt   time.Time
	hits         int
	banRenewals  int
	targets      map[string]int     // "<method> <path>" -> total hit count
	targetBodies map[string]*string // "<method> <path>" -> first-observed redacted body sample, nil if none captured
}

// IPGuardSecurityLayer implements security.SecurityLayer, wrapping an
// inner layer (in production, the existing ScopeSecurityLayer). It
// runs for every matched route — public or authenticated — since
// RouteBuilder.ServeHTTP calls Authorize unconditionally. See
// plan/ip-abuse-protection/plan.md sections 1 and 4.
//
// Enforcement is in-memory only on the hot path (no SQL per request —
// see plan section 1's rationale); SQL is touched only at hydration
// (once, at construction) and at the comparatively rare moment a ban,
// allowlist entry, or attack episode is created, updated, or removed.
type IPGuardSecurityLayer struct {
	inner   security.SecurityLayer
	cfg     Config
	log     logger.Logger
	limiter *Limiter

	banQuery     *security_query.IPBanQueryRepo
	banPersist   *security_persistent.IPBanPersistentRepo
	allowQuery   *security_query.IPAllowlistQueryRepo
	allowPersist *security_persistent.IPAllowlistPersistentRepo

	attackPersist  *attacks_persistent.AttackPersistentRepo
	attackLog      logger.Logger                 // dedicated log file — see plan section 12
	attackBanQuery *attackbans.SettingsQueryRepo // admin-editable scan/probe ban rule, see plan/attack-ban-rules/plan.md

	firewall firewall.Banner // OS-level enforcement, see plan section 14
	geo      geoip.Lookup    // country/ASN/ISP enrichment, may be nil — see plan/geoip-enrichment/plan.md

	bansMu sync.RWMutex
	bans   map[string]banEntry

	allowMu sync.RWMutex
	allow   map[string]struct{}

	attacksMu sync.Mutex
	attacks   map[string]*attackState // keyed by ip
}

// New builds an IPGuardSecurityLayer from the framework's DI context —
// the intended construction path from api/config/routes/routes.go.
// attacksHandle wraps the separate ip-attacks.db connection (section
// 10) in a *dbhandle.Handle, so the connection it holds keeps working
// even after the archiver rotates the file out from under it — see
// plan/ip-attacks-db-archiving/plan.md. attackLog is the dedicated
// attack log file (section 12). The OS-level firewall backend is
// auto-detected via firewall.Detect — callers don't choose it
// explicitly. geo is the geoip.Lookup used to enrich new attack
// episodes (see plan/geoip-enrichment/plan.md) — may be nil, same
// nil-safe convention as fw below.
func New(cfg Config, inner security.SecurityLayer, ctx di.Context, attacksHandle *dbhandle.Handle, attackLog logger.Logger, geo geoip.Lookup) (*IPGuardSecurityLayer, error) {
	manager := ctx.GetDatabaseManager()
	if manager == nil || manager.Connector == nil || manager.Connector.DB == nil {
		return nil, fmt.Errorf("ipguard: database manager not available")
	}
	fw := firewall.Detect(ctx.GetLogger())
	return NewWithDB(cfg, inner, manager.Connector.DB, ctx.GetLogger(), attacksHandle, attackLog, fw, geo)
}

// NewWithDB builds an IPGuardSecurityLayer directly from a *sql.DB (the
// main database, for bans/allowlist), a *dbhandle.Handle (the
// rotatable ip-attacks.db connection), loggers, a firewall.Banner, and
// a geoip.Lookup — the lower-level constructor, easiest to unit test
// without faking the full di.Context interface. log, attackLog, fw,
// and geo may all be nil/absent (fw nil disables OS-level enforcement
// entirely, same as a NoopBanner; geo nil disables geoip enrichment
// entirely — every use is guarded the same way firewall's is).
func NewWithDB(cfg Config, inner security.SecurityLayer, db *sql.DB, log logger.Logger, attacksHandle *dbhandle.Handle, attackLog logger.Logger, fw firewall.Banner, geo geoip.Lookup) (*IPGuardSecurityLayer, error) {
	g := &IPGuardSecurityLayer{
		inner:          inner,
		cfg:            cfg,
		log:            log,
		limiter:        NewLimiter(),
		banQuery:       security_query.NewIPBanQueryRepo(db),
		banPersist:     security_persistent.NewIPBanPersistentRepo(db),
		allowQuery:     security_query.NewIPAllowlistQueryRepo(db),
		allowPersist:   security_persistent.NewIPAllowlistPersistentRepo(db),
		attackPersist:  attacks_persistent.NewAttackPersistentRepo(attacksHandle),
		attackLog:      attackLog,
		attackBanQuery: attackbans.NewSettingsQueryRepo(db),
		firewall:       fw,
		geo:            geo,
		bans:           make(map[string]banEntry),
		allow:          make(map[string]struct{}),
		attacks:        make(map[string]*attackState),
	}
	if err := g.hydrate(); err != nil {
		return nil, err
	}
	return g, nil
}

// hydrate loads currently-active bans and the full allowlist into
// memory once, at construction. A ban row with an unparseable
// timestamp is skipped (logged) rather than failing startup entirely.
// It also force-closes any attack episode left open by a previous
// process instance — see AttackPersistentRepo.CloseAllOpen's doc
// comment for why that's always correct to do here.
func (g *IPGuardSecurityLayer) hydrate() error {
	active, err := g.banQuery.ListActive(time.Now())
	if err != nil {
		return fmt.Errorf("ipguard: hydrate bans: %w", err)
	}
	g.bansMu.Lock()
	for _, b := range active {
		expiresAt, ok := parseTime(b.ExpiresAt)
		if !ok {
			g.warnf("ipguard: skipping ban with unparseable expires_at ip=%s expires_at=%q", b.IP, b.ExpiresAt)
			continue
		}
		g.bans[b.IP] = banEntry{tier: b.Tier, reason: b.Reason, expiresAt: expiresAt}
	}
	g.bansMu.Unlock()

	g.resyncFirewall(active)

	entries, err := g.allowQuery.ListAllowlist()
	if err != nil {
		return fmt.Errorf("ipguard: hydrate allowlist: %w", err)
	}
	g.allowMu.Lock()
	for _, e := range entries {
		g.allow[e.IP] = struct{}{}
	}
	g.allowMu.Unlock()

	if n, err := g.attackPersist.CloseAllOpen(); err != nil {
		g.errorf("ipguard: failed to reconcile orphaned open attacks: %v", err)
	} else if n > 0 {
		g.infof("ipguard: closed %d attack episode(s) left open by a previous process instance", n)
	}

	return nil
}

// ClientIP resolves r's caller through this layer's configured
// proxy-header fallback chain — the same resolution Authorize/
// RecordRecon use internally, exposed so other handlers (e.g. the
// admin login-log recorder) don't need their own copy of the
// trust-header config. See plan/login-audit-log/plan.md Step 3.
func (g *IPGuardSecurityLayer) ClientIP(r *http.Request) string {
	return ClientIP(r, g.cfg.TrustProxyIPHeaders)
}

// Authorize is the enforcement path — see plan section 1's flow
// diagram: allowlist bypass, then ban check, then global tier, then
// (on a sensitive path) the sensitive tier, then delegate to inner.
// Every rejection additionally feeds the attack-tracking state
// (section 11) and the dedicated attack log (section 12).
func (g *IPGuardSecurityLayer) Authorize(w http.ResponseWriter, r *http.Request, ctx di.Context, route *security.Route) error {
	ip := ClientIP(r, g.cfg.TrustProxyIPHeaders)

	if g.isAllowed(ip) {
		return g.inner.Authorize(w, r, ctx, route)
	}

	if banned, tier, reason, retryAfter := g.checkBanned(ip); banned {
		g.recordAttackHit(ip, tier, reason, r, route, false)
		writeTooManyRequests(w, retryAfter)
		return fmt.Errorf("ipguard: ip banned: %s", ip)
	}

	globalWindow := time.Duration(g.cfg.RateLimit.Global.WindowSeconds) * time.Second
	if !g.limiter.Allow(ip+":global", g.cfg.RateLimit.Global.Requests, globalWindow) {
		banDuration := time.Duration(g.cfg.RateLimit.Global.BanSeconds) * time.Second
		reason := "global rate limit exceeded"
		g.autoBan(ip, "global", reason, banDuration)
		g.recordAttackHit(ip, "global", reason, r, route, true)
		writeTooManyRequests(w, banDuration)
		return fmt.Errorf("ipguard: global rate limit exceeded: %s", ip)
	}

	if g.isSensitivePath(route) {
		sensitiveWindow := time.Duration(g.cfg.RateLimit.Sensitive.WindowSeconds) * time.Second
		if !g.limiter.Allow(ip+":sensitive", g.cfg.RateLimit.Sensitive.Requests, sensitiveWindow) {
			banDuration := time.Duration(g.cfg.RateLimit.Sensitive.BanSeconds) * time.Second
			reason := "sensitive rate limit exceeded"
			g.autoBan(ip, "sensitive", reason, banDuration)
			g.recordAttackHit(ip, "sensitive", reason, r, route, true)
			writeTooManyRequests(w, banDuration)
			return fmt.Errorf("ipguard: sensitive rate limit exceeded: %s", ip)
		}
	}

	return g.inner.Authorize(w, r, ctx, route)
}

// RecordRecon records ip's probe against a path that matched no route
// at all — coco-server's RouteBuilder.NotFoundHook calls this
// directly, bypassing Authorize entirely, since there is no route to
// authorize against (Authorize only ever runs for matched routes —
// see its own doc comment). route is always nil here, so
// recordAttackHit falls back to the raw request path exactly as
// received, not a route pattern.
//
// Unlike every other tier, this never rate-limits or bans: a single
// stray 404 (a browser's /favicon.ico, a crawler, a typo) isn't
// evidence of an attack on its own the way tripping a rate limit is —
// this tier is visibility-only by design. See
// plan/port-scan-detection/plan.md Phase A and its Open Question 2 on
// whether to auto-ban here later.
func (g *IPGuardSecurityLayer) RecordRecon(ip string, r *http.Request) {
	if g.isAllowed(ip) {
		return
	}
	g.recordAttackHit(ip, "unmatched", "no matching route", r, nil, false)
}

// Ban records a ban for ip — updates the in-memory cache immediately
// (so enforcement never waits on a DB round-trip) and persists it.
// createdBy is nil for auto-bans (tier "global"/"sensitive") and an
// admin_user_id for manual bans (tier "manual"). Also fires an
// OS-level firewall ban (section 14) in the background — manual bans
// get the same network-level enforcement as auto-bans, since an admin
// banning an IP by hand expects it actually blocked, not just
// rejected at the application layer.
func (g *IPGuardSecurityLayer) Ban(ip, tier, reason string, duration time.Duration, createdBy *string) error {
	expiresAt := time.Now().Add(duration)

	g.bansMu.Lock()
	g.bans[ip] = banEntry{tier: tier, reason: reason, expiresAt: expiresAt}
	g.bansMu.Unlock()

	g.warnf("ipguard: banned ip=%s tier=%s reason=%q until=%s", ip, tier, reason, expiresAt.Format(time.RFC3339))
	g.banFirewall(ip, duration)

	if err := g.banPersist.UpsertBan(ip, tier, reason, expiresAt, createdBy); err != nil {
		return fmt.Errorf("ipguard: ban: %w", err)
	}
	return nil
}

// autoBan is Ban for the enforcement path itself — errors are logged,
// not returned, since a failed DB write must never stop the in-memory
// ban (already applied) from taking effect on this and subsequent
// requests.
func (g *IPGuardSecurityLayer) autoBan(ip, tier, reason string, duration time.Duration) {
	if err := g.Ban(ip, tier, reason, duration, nil); err != nil {
		g.errorf("ipguard: failed to persist auto-ban for %s: %v", ip, err)
	}
}

// Unban removes ip's ban, both persisted and in-memory, and lifts the
// OS-level firewall ban (if any) in the background. Errors if no ban
// exists for ip.
func (g *IPGuardSecurityLayer) Unban(ip string) error {
	if err := g.banPersist.DeleteBan(ip); err != nil {
		return fmt.Errorf("ipguard: unban: %w", err)
	}
	g.bansMu.Lock()
	delete(g.bans, ip)
	g.bansMu.Unlock()
	g.unbanFirewall(ip)
	return nil
}

// banFirewall and unbanFirewall run the OS-level firewall call in its
// own goroutine — the HTTP response (or the sweeper's tick) must
// never wait on a subprocess. firewall_linux.go's own 2s timeout
// bounds how long that goroutine can run; a nil g.firewall (no
// backend configured, e.g. in tests) is a silent no-op.
func (g *IPGuardSecurityLayer) banFirewall(ip string, duration time.Duration) {
	if g.firewall == nil {
		return
	}
	go func() {
		if err := g.firewall.Ban(ip, duration); err != nil {
			g.errorf("ipguard: firewall ban failed for %s (%s): %v", ip, g.firewall.Name(), err)
		}
	}()
}

func (g *IPGuardSecurityLayer) unbanFirewall(ip string) {
	if g.firewall == nil {
		return
	}
	go func() {
		if err := g.firewall.Unban(ip); err != nil {
			g.errorf("ipguard: firewall unban failed for %s (%s): %v", ip, g.firewall.Name(), err)
		}
	}()
}

// resyncFirewall re-applies every non-expired ban to the OS firewall at
// startup. A fresh process (redeploy, crash restart, host reboot) has no
// OS-level firewall rules of its own even though the DB/in-memory ban
// state survives hydrate() — without this, a ban keeps working at the
// application layer and keeps showing in the admin UI, but silently
// stops being enforced at the network level until an admin notices and
// clicks "Resync now" (see FirewallResyncHandler, which this mirrors).
// See plan/firewall-startup-resync/plan.md.
func (g *IPGuardSecurityLayer) resyncFirewall(active []security_entity.IPBan) {
	if g.firewall == nil || !g.firewall.Available() {
		return
	}
	now := time.Now()
	for _, b := range active {
		expiresAt, ok := parseTime(b.ExpiresAt)
		if !ok || expiresAt.Before(now) {
			continue
		}
		g.banFirewall(b.IP, expiresAt.Sub(now))
	}
}

// ListBans returns every ban row, including already-expired ones the
// sweeper hasn't pruned yet — for the admin ban-management page.
func (g *IPGuardSecurityLayer) ListBans() ([]security_entity.IPBan, error) {
	return g.banQuery.ListBans()
}

// AllowIP adds ip to the allowlist — both persisted and in-memory —
// exempting it from rate limiting and bans entirely.
func (g *IPGuardSecurityLayer) AllowIP(ip, note, createdBy string) error {
	if err := g.allowPersist.InsertAllowlistEntry(ip, note, createdBy); err != nil {
		return fmt.Errorf("ipguard: allow: %w", err)
	}
	g.allowMu.Lock()
	g.allow[ip] = struct{}{}
	g.allowMu.Unlock()
	return nil
}

// DisallowIP removes ip from the allowlist, both persisted and
// in-memory. Errors if ip was never allowlisted.
func (g *IPGuardSecurityLayer) DisallowIP(ip string) error {
	if err := g.allowPersist.DeleteAllowlistEntry(ip); err != nil {
		return fmt.Errorf("ipguard: disallow: %w", err)
	}
	g.allowMu.Lock()
	delete(g.allow, ip)
	g.allowMu.Unlock()
	return nil
}

// ListAllowlist returns every allowlist entry — for the admin page.
func (g *IPGuardSecurityLayer) ListAllowlist() ([]security_entity.IPAllowlistEntry, error) {
	return g.allowQuery.ListAllowlist()
}

// PruneExpiredBans deletes expired rows from ip_bans and evicts the
// matching in-memory entries. Intended to be called on a fixed
// interval by the sweeper (see plan section 6) — zero matches is the
// normal case, not an error.
func (g *IPGuardSecurityLayer) PruneExpiredBans() error {
	now := time.Now()
	n, err := g.banPersist.DeleteExpired(now)
	if err != nil {
		return fmt.Errorf("ipguard: prune expired bans: %w", err)
	}

	var expiredIPs []string
	g.bansMu.Lock()
	for ip, b := range g.bans {
		if !now.Before(b.expiresAt) {
			expiredIPs = append(expiredIPs, ip)
			delete(g.bans, ip)
		}
	}
	g.bansMu.Unlock()

	// Closes the same loop as the DB prune above — a firewall rule
	// left in place after its ban has expired is exactly the
	// unbounded-growth problem this whole feature exists to avoid, now
	// for the firewall's rule table instead of a SQL table. See plan
	// section 14's "Lifecycle / cleanup" note.
	for _, ip := range expiredIPs {
		g.unbanFirewall(ip)
	}

	if n > 0 {
		g.infof("ipguard: pruned %d expired ban(s)", n)
	}
	return nil
}

// FirewallStatus reports which OS-level firewall backend is in effect
// — for GET /admin/security/status, so the admin Attacks page can
// show a truthful warning when only application-layer enforcement is
// active. See plan sections 13-14.
func (g *IPGuardSecurityLayer) FirewallStatus() (name string, available bool, detail string) {
	if g.firewall == nil {
		return "none", false, "no firewall backend configured"
	}
	return g.firewall.Name(), g.firewall.Available(), g.firewall.Detail()
}

// PruneStaleCounters evicts in-memory rate-limit counters idle past
// twice the largest configured window — bounds memory growth from IPs
// that have stopped sending traffic. See plan section 1's "Memory
// growth" note.
func (g *IPGuardSecurityLayer) PruneStaleCounters() {
	g.limiter.Prune(2 * g.largestWindow())
}

// FlushAttacks writes the current in-memory attack-episode state to
// ip-attacks.db and closes any episode that's gone quiet past its
// tier's grace period (2x that tier's configured ban duration). See
// plan/ip-abuse-protection/plan.md section 11. Intended to be called
// on the same fixed interval as PruneExpiredBans/PruneStaleCounters.
func (g *IPGuardSecurityLayer) FlushAttacks() {
	now := time.Now()

	type snapshot struct {
		ip, attackID string
		tier         string
		hits         int
		banRenewals  int
		lastSeenAt   time.Time
		targets      map[string]int
		targetBodies map[string]*string
	}

	var snapshots []snapshot
	var toClose []string

	g.attacksMu.Lock()
	for ip, state := range g.attacks {
		targetsCopy := make(map[string]int, len(state.targets))
		for k, v := range state.targets {
			targetsCopy[k] = v
		}
		bodiesCopy := make(map[string]*string, len(state.targetBodies))
		for k, v := range state.targetBodies {
			bodiesCopy[k] = v
		}
		snapshots = append(snapshots, snapshot{
			ip: ip, attackID: state.attackID, tier: state.tier,
			hits: state.hits, banRenewals: state.banRenewals,
			lastSeenAt: state.lastSeenAt, targets: targetsCopy, targetBodies: bodiesCopy,
		})
		if now.Sub(state.lastSeenAt) > g.graceFor(state.tier) {
			toClose = append(toClose, ip)
		}
	}
	g.attacksMu.Unlock()

	for _, snap := range snapshots {
		if snap.attackID == "" {
			continue // CreateAttack never succeeded — nothing to flush
		}
		if err := g.attackPersist.UpdateAttackCounts(snap.attackID, snap.hits, snap.banRenewals, snap.lastSeenAt); err != nil {
			g.errorf("ipguard: failed to flush attack %s: %v", snap.attackID, err)
		}
		for key, count := range snap.targets {
			method, path := splitTargetKey(key)
			if err := g.attackPersist.SetTargetCount(snap.attackID, path, method, count, snap.targetBodies[key]); err != nil {
				g.errorf("ipguard: failed to flush attack target %s %s for %s: %v", method, path, snap.attackID, err)
			}
		}
	}

	if len(toClose) == 0 {
		return
	}
	g.attacksMu.Lock()
	closed := 0
	for _, ip := range toClose {
		state, ok := g.attacks[ip]
		if !ok {
			continue
		}
		if state.attackID != "" {
			if err := g.attackPersist.CloseAttack(state.attackID, state.lastSeenAt); err != nil {
				g.errorf("ipguard: failed to close attack %s: %v", state.attackID, err)
			} else {
				closed++
			}
		}
		delete(g.attacks, ip)
	}
	g.attacksMu.Unlock()
	if closed > 0 {
		g.infof("ipguard: closed %d attack episode(s) quiet past their grace period", closed)
	}
}

func (g *IPGuardSecurityLayer) isAllowed(ip string) bool {
	g.allowMu.RLock()
	defer g.allowMu.RUnlock()
	_, ok := g.allow[ip]
	return ok
}

// checkBanned reports whether ip is currently banned and, if so, the
// tier/reason of that ban and how long until it expires. A ban whose
// expiry has already passed (but hasn't been pruned yet) is treated
// as not-banned, letting the request fall through to normal counting.
func (g *IPGuardSecurityLayer) checkBanned(ip string) (banned bool, tier string, reason string, retryAfter time.Duration) {
	g.bansMu.RLock()
	b, ok := g.bans[ip]
	g.bansMu.RUnlock()
	if !ok {
		return false, "", "", 0
	}
	remaining := time.Until(b.expiresAt)
	if remaining <= 0 {
		return false, "", "", 0
	}
	return true, b.tier, b.reason, remaining
}

func (g *IPGuardSecurityLayer) isSensitivePath(route *security.Route) bool {
	if route == nil {
		return false
	}
	for _, p := range g.cfg.RateLimit.SensitivePaths {
		if p == route.Path {
			return true
		}
	}
	return false
}

// recordAttackHit tracks a rejected request as part of an ongoing (or
// newly-starting) attack episode for ip, and writes one line to the
// dedicated attack log (section 12). causedNewBan marks the specific
// hit that tripped a fresh ban (vs. a hit against an already-active
// one) — counted as a ban renewal.
func (g *IPGuardSecurityLayer) recordAttackHit(ip, tier, reason string, r *http.Request, route *security.Route, causedNewBan bool) {
	path := r.URL.Path
	if route != nil && route.Path != "" {
		path = route.Path
	}
	method := r.Method
	targetKey := method + " " + path
	now := time.Now()

	// Read outside attacksMu — this may block on network I/O reading
	// the client's body, and attacksMu is a single global lock guarding
	// every IP's state, not just this one. Holding it across a body
	// read would let a slow/malicious client stall rate-limit
	// decisions for every other IP too.
	bodySample := captureBodySample(r)

	// Only computed for the loopback/private case — a genuine public
	// attacker IP needs no fallback lead. Cheap (header map reads, no
	// I/O), but only ever used below if this hit turns out to be the
	// one that creates a new episode.
	var originHint *string
	if geoip.IsLoopbackOrPrivate(ip) {
		originHint = captureOriginHint(r)
	}

	// Only computed for the opposite case — loopback/private addresses
	// have no useful geoip data. Frozen into the episode row once,
	// below, and never re-derived later: geoip.db keeps no history of
	// its own, so this snapshot is the only place that fact survives.
	// See plan/geoip-enrichment/plan.md.
	var geoInfo *string
	if g.geo != nil && !geoip.IsLoopbackOrPrivate(ip) {
		if info, ok := g.geo.Lookup(ip); ok {
			if raw, err := json.Marshal(info); err == nil {
				s := string(raw)
				geoInfo = &s
			}
		}
	}

	g.attacksMu.Lock()
	state, ok := g.attacks[ip]
	if !ok {
		state = &attackState{tier: tier, startedAt: now, targets: make(map[string]int), targetBodies: make(map[string]*string)}
		g.attacks[ip] = state
	}
	state.tier = tier
	state.lastSeenAt = now
	state.hits++
	if _, seen := state.targets[targetKey]; !seen {
		state.targetBodies[targetKey] = bodySample
	}
	state.targets[targetKey]++
	if causedNewBan {
		state.banRenewals++
	}
	needsCreate := state.attackID == ""
	if needsCreate {
		// Reserved under the lock so a concurrent hit for the same IP
		// never races into creating a second DB row for one episode.
		state.attackID = uuid.New().String()
	}
	attackID := state.attackID
	hits := state.hits
	startedAt := state.startedAt
	g.attacksMu.Unlock()

	if needsCreate {
		if err := g.attackPersist.CreateAttack(attackID, ip, tier, now, originHint, geoInfo); err != nil {
			g.errorf("ipguard: failed to create attack record for %s: %v", ip, err)
		}
		if originHint != nil {
			g.logOriginHint(ip, tier, attackID, *originHint)
		}
	}

	g.logAttackHit(ip, tier, reason, method, path, attackID, hits)

	// Only the "unmatched" tier (requests to nonexistent routes, via
	// RecordRecon) reaches here without having already gone through
	// Authorize's own global/sensitive ban logic — gating on it avoids
	// a second, redundant autoBan for tiers that already ban themselves
	// earlier in the request lifecycle. See plan/attack-ban-rules/plan.md.
	if tier == "unmatched" {
		g.maybeAutoBanAttack(ip, hits, startedAt, now)
	}
}

// maybeAutoBanAttack applies the admin-configured attack ban rule
// (api/src/security/attackbans) to a scan/probe episode. Uses the
// episode's startedAt rather than a true rolling window over
// individual hit timestamps (attackState tracks neither) — this
// correctly catches a burst (many hits in a short span) but not a slow
// trickle spread across longer than WindowSeconds. Disabled by
// default; a LoadSettings error is treated the same as disabled so a
// transient DB hiccup never blocks the request.
func (g *IPGuardSecurityLayer) maybeAutoBanAttack(ip string, hits int, startedAt, now time.Time) {
	rules, err := g.attackBanQuery.LoadSettings()
	if err != nil || !rules.Enabled {
		return
	}
	windowStart := now.Add(-time.Duration(rules.WindowSeconds) * time.Second)
	if hits < rules.Threshold || startedAt.Before(windowStart) {
		return
	}
	reason := fmt.Sprintf("%d probe hits to nonexistent routes within %ds", hits, rules.WindowSeconds)
	g.autoBan(ip, "attack-scan", reason, time.Duration(rules.BanSeconds)*time.Second)
}

// graceFor returns how long an episode may stay quiet before
// FlushAttacks closes it — twice the tier's own ban duration, so a
// renewed ban doesn't prematurely close the episode. Manual bans have
// no tier-specific window configured, so they fall back to the global
// tier's.
func (g *IPGuardSecurityLayer) graceFor(tier string) time.Duration {
	if tier == "sensitive" {
		return 2 * time.Duration(g.cfg.RateLimit.Sensitive.BanSeconds) * time.Second
	}
	return 2 * time.Duration(g.cfg.RateLimit.Global.BanSeconds) * time.Second
}

func (g *IPGuardSecurityLayer) largestWindow() time.Duration {
	maxWindow := time.Duration(g.cfg.RateLimit.Global.WindowSeconds) * time.Second
	if sw := time.Duration(g.cfg.RateLimit.Sensitive.WindowSeconds) * time.Second; sw > maxWindow {
		maxWindow = sw
	}
	return maxWindow
}

func (g *IPGuardSecurityLayer) logAttackHit(ip, tier, reason, method, path, attackID string, hits int) {
	if g.attackLog == nil {
		return
	}
	g.attackLog.Warning(
		"ip=%s tier=%s path=%s method=%s attack_id=%s hit=%d reason=%q",
		ip, tier, path, method, attackID, hits, reason,
	)
}

// logOriginHint writes one line to the dedicated attack log the
// moment an episode opens with a loopback/private ip — visible in
// ip-attacks.log without needing the admin UI. See
// plan/attack-ip-attribution/plan.md Fix 3.
func (g *IPGuardSecurityLayer) logOriginHint(ip, tier, attackID, hint string) {
	if g.attackLog == nil {
		return
	}
	g.attackLog.Warning(
		"ip=%s tier=%s attack_id=%s could not resolve a public ip, origin_hint=%s",
		ip, tier, attackID, hint,
	)
}

func (g *IPGuardSecurityLayer) warnf(format string, args ...interface{}) {
	if g.log != nil {
		g.log.Warning(format, args...)
	}
}

func (g *IPGuardSecurityLayer) errorf(format string, args ...interface{}) {
	if g.log != nil {
		g.log.Error(format, args...)
	}
}

func (g *IPGuardSecurityLayer) infof(format string, args ...interface{}) {
	if g.log != nil {
		g.log.Info(format, args...)
	}
}

func writeTooManyRequests(w http.ResponseWriter, retryAfter time.Duration) {
	if retryAfter > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)
	_, _ = w.Write([]byte(`{"message": "too many requests"}`))
}

func parseTime(s string) (time.Time, bool) {
	for _, layout := range []string{timeLayout, time.RFC3339, time.RFC3339Nano} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

// splitTargetKey reverses the "<method> <path>" key recordAttackHit
// builds. Method never contains a space, so the first space is always
// the correct split point even if path itself is unusual.
func splitTargetKey(key string) (method, path string) {
	parts := strings.SplitN(key, " ", 2)
	if len(parts) != 2 {
		return "", key
	}
	return parts[0], parts[1]
}
