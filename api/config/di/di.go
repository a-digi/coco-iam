package di

import (
	resource_handler "github.com/a-digi/coco-iam/config/resource"
	"github.com/a-digi/coco-iam/src/security/dbarchive"
	"github.com/a-digi/coco-iam/src/security/dbhandle"
	"github.com/a-digi/coco-iam/src/security/geoip"
	"github.com/a-digi/coco-iam/src/security/ipguard"
	"github.com/a-digi/coco-iam/src/security/scanwatch"
	lift_api "github.com/a-digi/coco-lift/resource"
	"github.com/a-digi/coco-logger/logger"
	"github.com/a-digi/coco-orm/orm"
	serverdi "github.com/a-digi/coco-server/server/di"
)

var _ serverdi.Context = (*ContextBag)(nil)

type ContextBag struct {
	items              map[string]interface{}
	DatabaseManager    *orm.DatabaseManager
	Logger             logger.Logger
	ApiResourceHandler *lift_api.ApiResourceHandler
	// IPGuard is set by config/routes.Init once the security layer is
	// constructed (routes are wired after NewContextBag runs, so it
	// can't be filled in here) — see plan/ip-abuse-protection/plan.md
	// section 4. Admin ban/allowlist handlers resolve it via
	// GetIPGuard() so a manual unban actually updates the same
	// in-memory cache Authorize reads, instead of only writing SQL
	// that a still-running process would never see.
	IPGuard *ipguard.IPGuardSecurityLayer
	// IPAttacksHandle and IPAttacksLog are set directly in main.go
	// (both already constructed before NewContextBag runs, unlike
	// IPGuard) and read back out by config/routes.Init when building
	// IPGuard, and by the admin attacks handlers when querying
	// ip-attacks.db — see plan/ip-abuse-protection/plan.md sections
	// 10-12. IPAttacksHandle wraps a swappable *sql.DB connection so
	// the archiver rotating ip-attacks.db (see
	// plan/ip-attacks-db-archiving/plan.md) can hand every consumer a
	// fresh connection without reconstructing any of them — this is
	// why nothing here holds the underlying *orm.DatabaseManager or
	// *sql.DB directly.
	IPAttacksHandle *dbhandle.Handle
	IPAttacksLog    logger.Logger
	// DBArchiver is set directly in main.go (constructed alongside
	// IPAttacksHandle, before NewContextBag runs). Exposed here so the
	// admin security status endpoint can report the current entry
	// count and how close it is to the rotation threshold — see
	// plan/ip-attacks-db-archiving/plan.md.
	DBArchiver *dbarchive.Archiver
	// ScanSource is set directly in main.go. Exposed here so the admin
	// security status endpoint can report whether port-scan detection
	// actually has a log source available on this host — see
	// plan/port-scan-detection/plan.md Phase B. Nothing in the admin
	// API needs the scanwatch.Watcher itself: unlike IPGuard, there is
	// no admin-triggered action against it (no manual ban/allow
	// equivalent), so only the Source's status is exposed.
	ScanSource scanwatch.Source
	// GeoIPManager is set by config/routes.Init once constructed (same
	// timing as IPGuard, above) — resolved by the geoip settings/
	// process-control admin handlers so Start/Stop act on the same
	// instance the whole process shares, not a throwaway one built
	// per-request. See plan/geoip-enrichment/plan.md's "Process
	// control" section.
	GeoIPManager *geoip.Manager
	// GeoIP and GeoIPWatcher are set by config/routes.Init alongside
	// GeoIPManager (same timing/reasoning). GeoIPWatcher is read back
	// out by main.go to start its Run(queueCtx) goroutine — mirrors
	// how ScanSource/DBArchiver are threaded through today.
	GeoIP        geoip.Lookup
	GeoIPWatcher *geoip.Watcher
	// AdminLoginHandle and AdminLoginArchiver are set directly in
	// main.go (constructed the same way as IPAttacksHandle/DBArchiver,
	// just against admin_login.db instead of ip-attacks.db). Exposed
	// here so admin login-log handlers can query the current
	// connection and the admin security status endpoint can report
	// rotation stats, the same way IPAttacksHandle/DBArchiver already
	// are. See plan/login-audit-log/plan.md Step 2.
	AdminLoginHandle   *dbhandle.Handle
	AdminLoginArchiver *dbarchive.Archiver
}

func NewContextBag(manager *orm.DatabaseManager, log logger.Logger) *ContextBag {
	return &ContextBag{
		items:              make(map[string]interface{}),
		DatabaseManager:    manager,
		Logger:             log,
		ApiResourceHandler: resource_handler.GetApiResourceHandler(), // initialize singleton inside
	}
}

func (c *ContextBag) Set(key string, value interface{}) {
	c.items[key] = value
}

func (c *ContextBag) Get(key string) (interface{}, bool) {
	item, ok := c.items[key]
	return item, ok
}

func (c *ContextBag) GetDatabaseManager() *orm.DatabaseManager {
	return c.DatabaseManager
}

func (c *ContextBag) GetLogger() logger.Logger {
	return c.Logger
}

func (c *ContextBag) GetApiResourceHandler() *lift_api.ApiResourceHandler {
	return c.ApiResourceHandler
}

func (c *ContextBag) GetIPGuard() *ipguard.IPGuardSecurityLayer {
	return c.IPGuard
}

func (c *ContextBag) GetIPAttacksHandle() *dbhandle.Handle {
	return c.IPAttacksHandle
}

func (c *ContextBag) GetIPAttacksLog() logger.Logger {
	return c.IPAttacksLog
}

func (c *ContextBag) GetDBArchiver() *dbarchive.Archiver {
	return c.DBArchiver
}

func (c *ContextBag) GetScanSource() scanwatch.Source {
	return c.ScanSource
}

func (c *ContextBag) GetGeoIPManager() *geoip.Manager {
	return c.GeoIPManager
}

func (c *ContextBag) GetGeoIP() geoip.Lookup {
	return c.GeoIP
}

func (c *ContextBag) GetAdminLoginHandle() *dbhandle.Handle {
	return c.AdminLoginHandle
}

func (c *ContextBag) GetAdminLoginArchiver() *dbarchive.Archiver {
	return c.AdminLoginArchiver
}
