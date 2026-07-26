package di

import (
	resource_handler "github.com/a-digi/coco-iam/config/resource"
	"github.com/a-digi/coco-iam/src/security/ipguard"
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
	// IPAttacksDBManager and IPAttacksLog are set directly in main.go
	// (both are already constructed before NewContextBag runs, unlike
	// IPGuard) and read back out by config/routes.Init when building
	// IPGuard — see plan/ip-abuse-protection/plan.md sections 10-12.
	IPAttacksDBManager *orm.DatabaseManager
	IPAttacksLog       logger.Logger
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

func (c *ContextBag) GetIPAttacksDBManager() *orm.DatabaseManager {
	return c.IPAttacksDBManager
}

func (c *ContextBag) GetIPAttacksLog() logger.Logger {
	return c.IPAttacksLog
}
