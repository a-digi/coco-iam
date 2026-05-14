package di

import (
	resource_handler "github.com/a-digi/coco-iam/config/resource"
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
