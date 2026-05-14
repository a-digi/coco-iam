package producers

import (
	"github.com/a-digi/coco-server/server/request"
)

// OrganizationGroupMemberDeleteListener fires when a user is removed from a
// single group. The entity has `user_id` which we forward to the cleanup queue.
type OrganizationGroupMemberDeleteListener struct{}

func (l *OrganizationGroupMemberDeleteListener) BeforeExecution(_ request.RequestContext, _ interface{}) error {
	return nil
}

func (l *OrganizationGroupMemberDeleteListener) AfterExecution(reqCtx request.RequestContext, entity interface{}) {
	userID, err := extractString(entity, "user_id")
	if err != nil {
		reqCtx.GetDI().GetLogger().Warning("organization-group-member cleanup: failed to read user_id: %v", err)
		return
	}
	publish(reqCtx, userID)
}
