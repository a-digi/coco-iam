package deleted

import (
	"database/sql"
	"fmt"
)

// BuildSnapshot queries the main DB for the organization row and the
// per-org DB for workspaces and applications. It returns a value
// suitable for both persisting to deleted_organizations and driving the
// consumer cascade. Called from the delete listener BeforeExecution,
// while the rows still exist.
//
// orgDB may be nil when the per-org DB is not available; in that case
// the workspaces and applications slices will be empty and the consumer
// will fall back to the routing index at delete time.
func BuildSnapshot(mainDB *sql.DB, orgDB *sql.DB, orgID string) (Snapshot, error) {
	if orgID == "" {
		return Snapshot{}, fmt.Errorf("org id must not be empty")
	}

	var snap Snapshot

	// Organization from main DB.
	err := mainDB.QueryRow(
		`SELECT id, organization_id, title, description, created_at, is_active
		 FROM organization WHERE id = ?`, orgID,
	).Scan(
		&snap.Organization.ID,
		&snap.Organization.OrganizationID,
		&snap.Organization.Title,
		&snap.Organization.Description,
		&snap.Organization.CreatedAt,
		&snap.Organization.IsActive,
	)
	if err != nil {
		return Snapshot{}, fmt.Errorf("snapshot org %s: %w", orgID, err)
	}

	if orgDB == nil {
		return snap, nil
	}

	// Workspaces from per-org DB.
	wsRows, err := orgDB.Query(
		`SELECT id, workspace_id, organization_id, title, description, created_at, is_active
		 FROM workspace`,
	)
	if err != nil {
		return snap, nil
	}
	defer wsRows.Close()
	var wsIDs []string
	for wsRows.Next() {
		var w WorkspaceRow
		if err := wsRows.Scan(
			&w.ID, &w.WorkspaceID, &w.OrganizationID, &w.Title,
			&w.Description, &w.CreatedAt, &w.IsActive,
		); err != nil {
			return Snapshot{}, fmt.Errorf("scan workspace: %w", err)
		}
		snap.Workspaces = append(snap.Workspaces, w)
		wsIDs = append(wsIDs, w.ID)
	}
	if err := wsRows.Err(); err != nil {
		return Snapshot{}, fmt.Errorf("iter workspaces: %w", err)
	}

	if len(wsIDs) == 0 {
		return snap, nil
	}

	// Applications for any workspace in this org from per-org DB.
	placeholders := "?"
	for i := 1; i < len(wsIDs); i++ {
		placeholders += ",?"
	}
	args := make([]interface{}, len(wsIDs))
	for i, id := range wsIDs {
		args[i] = id
	}
	appRows, err := orgDB.Query(
		`SELECT id, client_id, workspace_id, title, description, created_at, is_active
		 FROM applications WHERE workspace_id IN (`+placeholders+`)`, args...,
	)
	if err != nil {
		return snap, nil
	}
	defer appRows.Close()
	for appRows.Next() {
		var a ApplicationRow
		if err := appRows.Scan(
			&a.ID, &a.ClientID, &a.WorkspaceID, &a.Title,
			&a.Description, &a.CreatedAt, &a.IsActive,
		); err != nil {
			return Snapshot{}, fmt.Errorf("scan application: %w", err)
		}
		snap.Applications = append(snap.Applications, a)
	}
	if err := appRows.Err(); err != nil {
		return Snapshot{}, fmt.Errorf("iter applications: %w", err)
	}

	return snap, nil
}
