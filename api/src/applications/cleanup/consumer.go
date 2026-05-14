package cleanup

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/a-digi/coco-queue"
	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-iam/src/orgrouter"
	"github.com/a-digi/coco-logger/logger"
	"github.com/a-digi/coco-orm/orm"
)

// Register attaches the application-user-cleanup consumer to the queue
// manager. Call this at app startup, before `manager.Start(ctx)`.
func Register(mgr queue.Manager, _ *orm.DatabaseManager, orgRegistry *dbregistry.OrgUserDBRegistry, log logger.Logger) error {
	handler := func(_ context.Context, rawPayload []byte) error {
		var p Payload
		if err := json.Unmarshal(rawPayload, &p); err != nil {
			return fmt.Errorf("invalid cleanup payload: %w", err)
		}
		if p.UserID == "" {
			return fmt.Errorf("cleanup payload missing user_id")
		}
		return processOne(orgRegistry, log, p)
	}

	return mgr.Register("application-user-cleanup", handler, queue.Config{})
}

// processOne walks every direct application_user_acl row the user holds and
// deletes it unless the user is still in a group mapped (via
// application_group_acl) to the same application.
//
// application_user_acl, user_group_members, and application_group_acl all
// live in the per-org DB.
func processOne(orgRegistry *dbregistry.OrgUserDBRegistry, log logger.Logger, p Payload) error {
	userID := p.UserID
	var orgDB *sql.DB
	if p.OrgID != "" && orgRegistry != nil {
		// Fast path: org is known, resolve directly without routing table.
		if mgr, err := orgRegistry.For(p.OrgID); err == nil {
			orgDB = mgr.Connector.DB
		}
	} else if orgRegistry != nil {
		// Fallback: scan per-org DBs to find the user.
		if odb, _, err := orgrouter.OrgDBFor(orgRegistry, userID); err == nil {
			orgDB = odb
		}
	}
	if orgDB == nil {
		log.Warning("application-user-cleanup: cannot resolve org db for user %s, skipping", userID)
		return nil
	}

	rows, err := orgDB.Query(
		`SELECT id, application_id FROM application_user_acl WHERE user_id = ?`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("load direct ACLs: %w", err)
	}

	type aclRow struct {
		id    string
		appID string
	}
	var acls []aclRow
	for rows.Next() {
		var r aclRow
		if err := rows.Scan(&r.id, &r.appID); err != nil {
			rows.Close()
			return fmt.Errorf("scan direct ACL: %w", err)
		}
		acls = append(acls, r)
	}
	rows.Close()

	if len(acls) == 0 {
		log.Info("application-user-cleanup: user %s has no direct ACLs, nothing to do", userID)
		return nil
	}

	// Resolve the user's current group memberships from the per-org DB.
	groupRows, err := orgDB.Query(
		`SELECT group_id FROM user_group_members WHERE user_id = ? AND is_active = TRUE`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("load group memberships: %w", err)
	}
	userGroups := map[string]bool{}
	for groupRows.Next() {
		var gid string
		if err := groupRows.Scan(&gid); err != nil {
			groupRows.Close()
			return fmt.Errorf("scan group membership: %w", err)
		}
		userGroups[gid] = true
	}
	groupRows.Close()

	removed := 0
	kept := 0
	for _, acl := range acls {
		covered, err := hasCoverage(orgDB, acl.appID, userGroups)
		if err != nil {
			return fmt.Errorf("coverage check for app %s: %w", acl.appID, err)
		}
		if covered {
			kept++
			continue
		}
		if _, err := orgDB.Exec(`DELETE FROM application_user_acl WHERE id = ?`, acl.id); err != nil {
			return fmt.Errorf("delete ACL %s: %w", acl.id, err)
		}
		removed++
	}

	log.Info("application-user-cleanup: user %s — %d removed, %d kept (group-covered)", userID, removed, kept)
	return nil
}

// hasCoverage returns true if any of the user's current groups has an active
// application_group_acl row on the given application. application_group_acl
// lives in the per-org DB.
func hasCoverage(orgDB *sql.DB, appID string, userGroups map[string]bool) (bool, error) {
	if len(userGroups) == 0 {
		return false, nil
	}

	rows, err := orgDB.Query(
		`SELECT group_id FROM application_group_acl WHERE application_id = ? AND is_active = TRUE`,
		appID,
	)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var gid string
		if err := rows.Scan(&gid); err != nil {
			return false, err
		}
		if userGroups[gid] {
			return true, nil
		}
	}
	return false, rows.Err()
}
