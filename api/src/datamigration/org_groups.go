package datamigration

import (
	"database/sql"
	"fmt"

	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-iam/src/orgrouter"
	"github.com/a-digi/coco-logger/logger"
)

// MigrateOrgGroupsToOrgDBs copies user_groups, user_group_acl,
// organization_group_acl, and any remaining user_group_members rows from the
// global users.db into each organisation's per-org users.db.
//
// Must be called after orgUserDBRegistry.SweepExisting() so per-org table
// schemas (migrations 19-21) already exist. Idempotent: returns nil
// immediately when user_groups is no longer in the global DB.
func MigrateOrgGroupsToOrgDBs(
	mainDB *sql.DB,
	reg *dbregistry.OrgUserDBRegistry,
	log logger.Logger,
) error {
	if !tableExists(mainDB, "user_groups") {
		return nil
	}

	orgIDs, err := distinctGroupOrgIDs(mainDB)
	if err != nil {
		return fmt.Errorf("org groups migration: list orgs: %w", err)
	}

	allOK := true
	for _, orgID := range orgIDs {
		orgDB, oErr := orgrouter.ForOrg(reg, orgID)
		if oErr != nil {
			log.Warning("org groups migration: open org db %s: %v", orgID, oErr)
			allOK = false
			continue
		}
		if cErr := copyOrgGroups(mainDB, orgDB, orgID); cErr != nil {
			log.Warning("org groups migration: org %s: %v", orgID, cErr)
			allOK = false
		}
	}

	if !allOK {
		return fmt.Errorf("org groups migration: some orgs failed -- global tables preserved for retry")
	}

	if err := dropGlobalGroupTables(mainDB); err != nil {
		return fmt.Errorf("org groups migration: drop global tables: %w", err)
	}
	log.Info("org groups migration: global tables dropped")
	return nil
}

func distinctGroupOrgIDs(mainDB *sql.DB) ([]string, error) {
	rows, err := mainDB.Query(
		`SELECT DISTINCT organization_id FROM user_groups
		 WHERE group_type != 'admin' AND organization_id != ''`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func copyOrgGroups(mainDB, orgDB *sql.DB, orgID string) error {
	rows, err := mainDB.Query(
		`SELECT id, title, group_description, organization_id, created_at, is_active
		 FROM user_groups WHERE organization_id = ? AND group_type != 'admin'`,
		orgID,
	)
	if err != nil {
		return fmt.Errorf("query user_groups: %w", err)
	}

	var groupIDs []string
	for rows.Next() {
		var id, title, desc, orgid, createdAt string
		var isActive bool
		if err := rows.Scan(&id, &title, &desc, &orgid, &createdAt, &isActive); err != nil {
			rows.Close()
			return fmt.Errorf("scan user_groups: %w", err)
		}
		if _, err := orgDB.Exec(
			`INSERT OR IGNORE INTO user_groups
			 (id, title, group_description, organization_id, created_at, is_active)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			id, title, desc, orgid, createdAt, isActive,
		); err != nil {
			rows.Close()
			return fmt.Errorf("insert user_groups: %w", err)
		}
		groupIDs = append(groupIDs, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	if len(groupIDs) == 0 {
		return nil
	}
	ph := inPlaceholders(len(groupIDs))
	args := toAnySlice(groupIDs)

	if err := copyRows(mainDB, orgDB,
		`SELECT id, group_id, roles, created_at, is_active
		 FROM user_group_acl WHERE group_id IN (`+ph+`)`,
		`INSERT OR IGNORE INTO user_group_acl (id, group_id, roles, created_at, is_active)
		 VALUES (?, ?, ?, ?, ?)`,
		args...,
	); err != nil {
		return fmt.Errorf("user_group_acl: %w", err)
	}

	if err := copyRows(mainDB, orgDB,
		`SELECT id, group_id, roles, created_at, is_active
		 FROM organization_group_acl WHERE group_id IN (`+ph+`)`,
		`INSERT OR IGNORE INTO organization_group_acl (id, group_id, roles, created_at, is_active)
		 VALUES (?, ?, ?, ?, ?)`,
		args...,
	); err != nil {
		return fmt.Errorf("organization_group_acl: %w", err)
	}

	if err := copyRows(mainDB, orgDB,
		`SELECT id, user_id, group_id, created_at, is_active
		 FROM user_group_members WHERE group_id IN (`+ph+`)`,
		`INSERT OR IGNORE INTO user_group_members (id, user_id, group_id, created_at, is_active)
		 VALUES (?, ?, ?, ?, ?)`,
		args...,
	); err != nil {
		return fmt.Errorf("user_group_members: %w", err)
	}

	return nil
}

func dropGlobalGroupTables(mainDB *sql.DB) error {
	stmts := []string{
		`DROP TABLE IF EXISTS user_group_acl`,
		`DROP TABLE IF EXISTS organization_group_acl`,
		`DROP TABLE IF EXISTS user_group_members`,
		`DROP TABLE IF EXISTS user_groups`,
	}
	for _, stmt := range stmts {
		if _, err := mainDB.Exec(stmt); err != nil {
			return fmt.Errorf("%s: %w", stmt, err)
		}
	}
	return nil
}
