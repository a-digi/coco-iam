package deleted

import (
	"github.com/a-digi/coco-queue"
	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-logger/logger"
	"github.com/a-digi/coco-orm/orm"
)

// Register attaches the organization-deleted consumer to the queue
// manager. Call from main.go after the queue manager is built but
// before `manager.Start(ctx)` runs.
func Register(mgr queue.Manager, dbm *orm.DatabaseManager, orgRegistry *dbregistry.OrgUserDBRegistry, log logger.Logger) error {
	return mgr.Register("organization-deleted", handler(dbm.Connector.DB, orgRegistry, log), queue.Config{})
}
