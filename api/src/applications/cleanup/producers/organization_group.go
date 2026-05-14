package producers

import (
	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-iam/src/orgrouter"
	"github.com/a-digi/coco-server/server/request"
)

// OrganizationGroupDeleteListener fires when an entire org-scoped group is
// deleted. BeforeExecution pre-resolves every member so that AfterExecution
// can publish a cleanup task per user, even after the memberships are gone.
type OrganizationGroupDeleteListener struct {
	pending pendingUserIDs
}

func (l *OrganizationGroupDeleteListener) BeforeExecution(reqCtx request.RequestContext, entity interface{}) error {
	log := reqCtx.GetDI().GetLogger()

	groupID, err := extractString(entity, "id")
	if err != nil {
		log.Warning("organization-group cleanup: failed to read id: %v", err)
		return nil
	}

	ctx := reqCtx.GetDI()
	bag, ok := ctx.(interface{ Get(string) (interface{}, bool) })
	if !ok {
		log.Warning("organization-group cleanup: di context not available")
		return nil
	}
	raw, ok := bag.Get(dbregistry.ContextBagKey)
	if !ok {
		log.Warning("organization-group cleanup: org user db registry not available")
		return nil
	}
	reg, _ := raw.(*dbregistry.OrgUserDBRegistry)
	if reg == nil {
		log.Warning("organization-group cleanup: org user db registry is nil")
		return nil
	}

	var userIDs []string
	for _, orgID := range reg.KnownOrgIDs() {
		odb, err := orgrouter.ForOrg(reg, orgID)
		if err != nil {
			continue
		}
		var found string
		if odb.QueryRow(`SELECT id FROM user_groups WHERE id = ? LIMIT 1`, groupID).Scan(&found) != nil {
			continue
		}
		rows, err := odb.Query(
			`SELECT user_id FROM user_group_members WHERE group_id = ? AND is_active = TRUE`,
			groupID,
		)
		if err != nil {
			log.Warning("organization-group cleanup: member lookup failed: %v", err)
			break
		}
		for rows.Next() {
			var uid string
			if err := rows.Scan(&uid); err != nil {
				log.Warning("organization-group cleanup: scan failed: %v", err)
				continue
			}
			userIDs = append(userIDs, uid)
		}
		rows.Close()
		break
	}

	l.pending.put(entity, userIDs)
	return nil
}

func (l *OrganizationGroupDeleteListener) AfterExecution(reqCtx request.RequestContext, entity interface{}) {
	userIDs := l.pending.take(entity)
	publish(reqCtx, userIDs...)
}
