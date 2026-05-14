package deleted

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-iam/src/orgrouter"
	"github.com/a-digi/coco-logger/logger"
)

// dbBaseDir is the directory that holds both the main DB and every
// per-org folder. Hard-coded to match main.go's dbmanager calls.
const dbBaseDir = "./data/db"

// orgDirName is the per-org folder parent under dbBaseDir. Each org's
// files live at <dbBaseDir>/<orgDirName>/<orgID>/{profiles,users}.db.
// Must match the constant defined in both dbregistry packages.
const orgDirName = "organization"

// deletedDir is the archive folder the consumer creates on first use
// and moves the per-org folder into. Renamed from the earlier
// `deleted_databases` to match the new "one folder per deleted org"
// layout: `<deletedDir>/<orgID>/` rather than the old
// `<deletedDir>/<stamp>__<orgID>/`.
const deletedDir = "./data/db/deleted"

// LegacyDeletedDir is the pre-rename archive folder. Kept as a
// constant so main.go can run MigrateLegacyArchiveDir on boot and
// move old archives into the new layout without losing them.
const LegacyDeletedDir = "./data/db/deleted_databases"

// uploadsRoots lists the on-disk upload trees owned per application.
// The consumer removes the `<appID>` subdirectory under each root for
// every deleted application. Order doesn't matter.
var uploadsRoots = []string{
	"./data/uploads/media",
	"./data/uploads/login-assets",
}

// processOne runs the full cascade for one organization. Every step
// is idempotent so a retry after partial progress is safe.
func processOne(mainDB *sql.DB, orgRegistry *dbregistry.OrgUserDBRegistry, log logger.Logger, p Payload) error {
	orgID := p.OrganizationID
	if orgID == "" {
		return nil
	}

	// Open the per-org DB early so per-org cleanup can proceed.
	var orgDB *sql.DB
	if orgRegistry != nil {
		if odb, err := orgrouter.ForOrg(orgRegistry, orgID); err == nil {
			orgDB = odb
		}
	}

	// 1. Persist the audit row first. If everything after this crashes
	//    we still have the snapshot.
	if err := persistSnapshot(mainDB, orgID, p.Snapshot); err != nil {
		return fmt.Errorf("persist snapshot: %w", err)
	}

	// 2. Gather ids for the cascade. We prefer the snapshot (it reflects
	//    the pre-delete state); fall back to a live query in case the
	//    snapshot was empty (the listener couldn't build one).
	wsIDs, appIDs := idsFromSnapshot(p.Snapshot)
	if len(wsIDs) == 0 {
		wsIDs = loadWorkspaceIDs(orgDB, orgID)
	}
	if len(appIDs) == 0 {
		appIDs = loadApplicationIDs(orgDB, wsIDs)
	}

	// 3. Cascade rows. We walk per-app data first, then apps,
	//    workspaces, org-scoped users / groups / rule-sets.
	if err := deleteApplicationData(mainDB, orgDB, appIDs); err != nil {
		return fmt.Errorf("delete application data: %w", err)
	}
	if err := deleteApplications(mainDB, appIDs); err != nil {
		return fmt.Errorf("delete applications: %w", err)
	}
	if err := deleteWorkspaces(mainDB, wsIDs); err != nil {
		return fmt.Errorf("delete workspaces: %w", err)
	}
	if err := deleteOrgScopedGroups(mainDB, orgDB, orgID); err != nil {
		return fmt.Errorf("delete org groups: %w", err)
	}
	if err := deleteOrgScopedUsers(mainDB, orgDB, orgID); err != nil {
		return fmt.Errorf("delete org users: %w", err)
	}
	// 4. Move the whole per-org folder into the archive. The archive
	//    layout mirrors the live tree so admins can navigate them
	//    symmetrically:
	//      live:    ./data/db/organization/<orgID>/
	//      archive: ./data/db/deleted/organization/<orgID>/
	//    Because org UUIDs are unique, the base destination never
	//    collides in practice. The __stamp fallback below only ever
	//    fires if an admin manually replays an identical delete.
	archiveOrgRoot := filepath.Join(deletedDir, orgDirName)
	if err := os.MkdirAll(archiveOrgRoot, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", archiveOrgRoot, err)
	}
	src := filepath.Join(dbBaseDir, orgDirName, orgID)
	dst := filepath.Join(archiveOrgRoot, orgID)
	if _, err := os.Stat(dst); err == nil {
		// Collision — an archive for this orgID already exists.
		// Append a timestamp so the caller's retry doesn't lose
		// either copy.
		stamp := time.Now().UTC().Format("20060102_150405")
		dst = filepath.Join(archiveOrgRoot, fmt.Sprintf("%s__%s", orgID, stamp))
	}
	if err := moveIfExists(src, dst); err != nil {
		// Log and continue — leaving the folder in place on a transient
		// rename failure is preferable to blocking the rest of the
		// cascade. The next retry will move it.
		if log != nil {
			log.Warning("organization-deleted: move %s → %s: %v", src, dst, err)
		}
	}

	// 5. Remove upload directories for every deleted application.
	for _, appID := range appIDs {
		if appID == "" {
			continue
		}
		for _, root := range uploadsRoots {
			path := filepath.Join(root, appID)
			if err := os.RemoveAll(path); err != nil {
				if log != nil {
					log.Warning("organization-deleted: remove %s: %v", path, err)
				}
			}
		}
	}

	if log != nil {
		log.Info(
			"organization-deleted: cascade complete for %s — %d workspaces, %d applications",
			orgID, len(wsIDs), len(appIDs),
		)
	}
	return nil
}

// --- snapshot persistence -------------------------------------------

func persistSnapshot(db *sql.DB, orgID string, snap Snapshot) error {
	raw, err := json.Marshal(snap)
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}
	_, err = db.Exec(
		`INSERT OR IGNORE INTO deleted_organizations (organization_id, snapshot_json)
		 VALUES (?, ?)`,
		orgID, string(raw),
	)
	if err != nil {
		return fmt.Errorf("insert deleted_organizations: %w", err)
	}
	return nil
}

// --- cascade ---------------------------------------------------------

func idsFromSnapshot(snap Snapshot) (wsIDs, appIDs []string) {
	for _, w := range snap.Workspaces {
		if w.ID != "" {
			wsIDs = append(wsIDs, w.ID)
		}
	}
	for _, a := range snap.Applications {
		if a.ID != "" {
			appIDs = append(appIDs, a.ID)
		}
	}
	return
}

func loadWorkspaceIDs(orgDB *sql.DB, orgID string) []string {
	if orgDB == nil {
		return nil
	}
	// workspace is in the per-org DB and carries organization_id for
	// filtering (kept from original schema).
	rows, err := orgDB.Query(`SELECT id FROM workspace WHERE organization_id = ?`, orgID)
	if err != nil {
		// workspace.organization_id may not exist if the column was removed;
		// fall back to selecting all workspaces in this org DB.
		rows, err = orgDB.Query(`SELECT id FROM workspace`)
		if err != nil {
			return nil
		}
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			out = append(out, id)
		}
	}
	return out
}

func loadApplicationIDs(orgDB *sql.DB, wsIDs []string) []string {
	if len(wsIDs) == 0 || orgDB == nil {
		return nil
	}
	ph, args := inClause(wsIDs)
	rows, err := orgDB.Query(`SELECT id FROM applications WHERE workspace_id IN (`+ph+`)`, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			out = append(out, id)
		}
	}
	return out
}

// deleteApplicationData wipes per-application rows that live in the
// main DB before the per-org folder is archived. Tables that moved to
// the per-org DB (application_login_settings, application_login_assets,
// application_keys, application_oauth_clients, application_org_index,
// etc.) are removed by archiving the whole per-org folder.
func deleteApplicationData(mainDB *sql.DB, orgDB *sql.DB, appIDs []string) error {
	if len(appIDs) == 0 {
		return nil
	}
	ph, args := inClause(appIDs)
	// Main DB tables still holding per-application data.
	mainStmts := []string{
		// Media rows for applications whose bytes live under
		// ./data/uploads/media/<appID>.
		`DELETE FROM media_folders WHERE owner_id IN (` + ph + `)`,
		`DELETE FROM media_files WHERE owner_id IN (` + ph + `)`,
	}
	for _, stmt := range mainStmts {
		if _, err := mainDB.Exec(stmt, args...); err != nil {
			return fmt.Errorf("exec %q: %w", stmt, err)
		}
	}
	return nil
}

// deleteApplications is a no-op: application rows and the routing index
// (application_org_index) now live in the per-org DB which is archived
// as a whole when the org is deleted.
func deleteApplications(_ *sql.DB, _ []string) error {
	return nil
}

// deleteWorkspaces is a no-op: workspace rows live in the per-org DB which
// is archived as a whole when the org is deleted.
func deleteWorkspaces(_ *sql.DB, _ []string) error {
	return nil
}

// deleteOrgScopedGroups walks every user_group owned by the org and
// removes its members, ACL, and the group row itself. All tables now
// live in the per-org DB; this function is a no-op when orgDB is nil
// because the whole per-org folder is archived by the caller anyway.
func deleteOrgScopedGroups(_ *sql.DB, orgDB *sql.DB, orgID string) error {
	if orgDB == nil {
		return nil
	}
	rows, err := orgDB.Query(`SELECT id FROM user_groups WHERE organization_id = ?`, orgID)
	if err != nil {
		return fmt.Errorf("load org groups: %w", err)
	}
	var groupIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fmt.Errorf("scan group id: %w", err)
		}
		groupIDs = append(groupIDs, id)
	}
	rows.Close()

	if len(groupIDs) > 0 {
		ph, args := inClause(groupIDs)
		for _, stmt := range []string{
			`DELETE FROM user_group_members WHERE group_id IN (` + ph + `)`,
			`DELETE FROM user_group_acl WHERE group_id IN (` + ph + `)`,
			`DELETE FROM organization_group_acl WHERE group_id IN (` + ph + `)`,
			`DELETE FROM user_groups WHERE id IN (` + ph + `)`,
		} {
			if _, err := orgDB.Exec(stmt, args...); err != nil {
				return fmt.Errorf("exec %q: %w", stmt, err)
			}
		}
	}
	return nil
}

// deleteOrgScopedUsers removes every user in the org along with their
// password, direct user ACL, and any group memberships from the per-org DB.
func deleteOrgScopedUsers(_ *sql.DB, orgDB *sql.DB, _ string) error {
	if orgDB == nil {
		return nil
	}
	rows, err := orgDB.Query(`SELECT id FROM users`)
	if err != nil {
		return fmt.Errorf("load org user ids: %w", err)
	}
	var userIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fmt.Errorf("scan user id: %w", err)
		}
		userIDs = append(userIDs, id)
	}
	rows.Close()

	if len(userIDs) == 0 {
		return nil
	}
	ph, args := inClause(userIDs)
	for _, stmt := range []string{
		`DELETE FROM user_auth_password WHERE user_id IN (` + ph + `)`,
		`DELETE FROM user_group_members WHERE user_id IN (` + ph + `)`,
		`DELETE FROM application_user_acl WHERE user_id IN (` + ph + `)`,
		`DELETE FROM users WHERE id IN (` + ph + `)`,
	} {
		if _, err := orgDB.Exec(stmt, args...); err != nil {
			return fmt.Errorf("exec %q (org db): %w", stmt, err)
		}
	}
	return nil
}

// --- helpers ---------------------------------------------------------

// moveIfExists renames src → dst, treating a missing src as success.
// Other errors bubble up.
func moveIfExists(src, dst string) error {
	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat %s: %w", src, err)
	}
	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("rename %s → %s: %w", src, dst, err)
	}
	return nil
}

// inClause builds an IN (?,?,?…) placeholder string + args slice for a
// set of string ids. Callers must not pass an empty slice.
func inClause(ids []string) (string, []interface{}) {
	ph := "?"
	for i := 1; i < len(ids); i++ {
		ph += ",?"
	}
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	return ph, args
}

// handler is the queue.Handler signature shape. Separated for testability.
func handler(mainDB *sql.DB, orgRegistry *dbregistry.OrgUserDBRegistry, log logger.Logger) func(context.Context, []byte) error {
	return func(_ context.Context, raw []byte) error {
		var p Payload
		if err := json.Unmarshal(raw, &p); err != nil {
			return fmt.Errorf("invalid organization-deleted payload: %w", err)
		}
		return processOne(mainDB, orgRegistry, log, p)
	}
}
