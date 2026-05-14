package orgrouter

import (
	"database/sql"
	"fmt"

	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
)

// OrgDBFor scans all known per-org DBs to find the one that contains userID.
// Returns the DB, the orgID, and nil on success.
func OrgDBFor(registry *dbregistry.OrgUserDBRegistry, userID string) (*sql.DB, string, error) {
	for _, orgID := range registry.KnownOrgIDs() {
		odb, err := ForOrg(registry, orgID)
		if err != nil {
			continue
		}
		var found string
		if odb.QueryRow(`SELECT id FROM users WHERE id = ? LIMIT 1`, userID).Scan(&found) == nil {
			return odb, orgID, nil
		}
	}
	return nil, "", fmt.Errorf("orgrouter: user %s not found in any org", userID)
}

// OrgDBForEmail scans all known per-org DBs to find the one owning email.
// Returns (db, orgID, userID, nil) on success.
func OrgDBForEmail(registry *dbregistry.OrgUserDBRegistry, email string) (*sql.DB, string, string, error) {
	for _, orgID := range registry.KnownOrgIDs() {
		odb, err := ForOrg(registry, orgID)
		if err != nil {
			continue
		}
		var userID string
		if odb.QueryRow(
			`SELECT id FROM users WHERE LOWER(email) = LOWER(?) LIMIT 1`, email,
		).Scan(&userID) == nil {
			return odb, orgID, userID, nil
		}
	}
	return nil, "", "", fmt.Errorf("orgrouter: email not found in any org")
}

// OrgDBForApp scans all known per-org DBs to find the one that owns appID.
// Returns (db, orgID, nil) on success.
func OrgDBForApp(registry *dbregistry.OrgUserDBRegistry, appID string) (*sql.DB, string, error) {
	for _, orgID := range registry.KnownOrgIDs() {
		odb, err := ForOrg(registry, orgID)
		if err != nil {
			continue
		}
		var found string
		if odb.QueryRow(`SELECT id FROM applications WHERE id = ? LIMIT 1`, appID).Scan(&found) == nil {
			return odb, orgID, nil
		}
	}
	return nil, "", fmt.Errorf("orgrouter: application %s not found in any org", appID)
}

// ForOrg opens the per-org users DB for the given org ID.
func ForOrg(registry *dbregistry.OrgUserDBRegistry, orgID string) (*sql.DB, error) {
	mgr, err := registry.For(orgID)
	if err != nil {
		return nil, fmt.Errorf("orgrouter: open org db %s: %w", orgID, err)
	}
	if mgr == nil || mgr.Connector == nil || mgr.Connector.DB == nil {
		return nil, fmt.Errorf("orgrouter: org db %s has no connection", orgID)
	}
	return mgr.Connector.DB, nil
}
