package producers

import (
	"github.com/a-digi/coco-server/server/request"
)

// OrganizationUserDeleteListener fires when a non-admin user is removed from
// an organization. The entity's `id` is the user id. One cleanup task is
// enqueued for that user after the delete has succeeded.
type OrganizationUserDeleteListener struct{}

func (l *OrganizationUserDeleteListener) BeforeExecution(_ request.RequestContext, _ interface{}) error {
	return nil
}

func (l *OrganizationUserDeleteListener) AfterExecution(reqCtx request.RequestContext, entity interface{}) {
	userID, err := extractString(entity, "id")
	if err != nil {
		reqCtx.GetDI().GetLogger().Warning("organization-user cleanup: failed to read id: %v", err)
		return
	}
	publish(reqCtx, userID)
}
