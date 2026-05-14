// Package dbregistry owns the per-organization SQLite file that holds
// a single organization's profile-field schema plus its users' profile
// values. One SQLite file per organization, created lazily on first
// use or eagerly via Provision.
//
// On-disk layout (alongside the per-org user DB):
//
//	<baseDir>/organization/<orgID>/profiles.db
//	<baseDir>/organization/<orgID>/users.db
//
// The shared `<baseDir>/organization/<orgID>/` folder means the whole
// org's data lives in one place — easy to locate, easy to archive on
// delete (the deletion consumer moves the entire folder into
// <baseDir>/deleted/organization/<orgID>/).
package dbregistry

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/a-digi/coco-orm/orm"
	"github.com/a-digi/coco-orm/orm/pool"
)

// ContextBagKey is the key under which a single OrgDBRegistry instance
// is stored in the app's ContextBag.
const ContextBagKey = "organizations.profile.db_registry"

// OrgDBRegistry opens and caches per-organization database handles via
// an LRU pool. Each organization has its own SQLite file:
//
//	<baseDir>/organization/<orgID>/profiles.db
//
// All calls are safe for concurrent use.
type OrgDBRegistry struct {
	baseDir        string
	migrationsPath string
	pool           *pool.Pool
}

// New builds a registry rooted at baseDir ("./data/db" in production).
// migrationsPath is the filesystem path to the already-extracted per-org
// migration files — callers use config.ExtractOrgMigrationsToTemp() to
// obtain it at startup.
func New(baseDir, migrationsPath string) *OrgDBRegistry {
	return &OrgDBRegistry{
		baseDir:        baseDir,
		migrationsPath: migrationsPath,
		pool:           pool.New(pool.Config{}),
	}
}

// For returns the DatabaseManager for the given organization id. If the
// database file does not exist yet it is created and migrations are run;
// the handle is cached for subsequent calls.
func (r *OrgDBRegistry) For(orgID string) (*orm.DatabaseManager, error) {
	if orgID == "" {
		return nil, fmt.Errorf("org id must not be empty")
	}
	return r.pool.Get(dbFileName, r.orgDir(orgID), []string{r.migrationsPath})
}

// Provision forces creation + migration of the org's database.
// Called from the organizations.PostEventListener so the file exists
// even before the first profile request.
func (r *OrgDBRegistry) Provision(orgID string) error {
	_, err := r.For(orgID)
	return err
}

// Destroy closes any cached handle and removes the per-org folder.
// Used on hard-delete of an organization. The deletion consumer
// normally moves the folder instead of removing it (to preserve the
// archive); Destroy is kept for callers that want an immediate wipe.
func (r *OrgDBRegistry) Destroy(orgID string) error {
	if orgID == "" {
		return fmt.Errorf("org id must not be empty")
	}
	r.pool.Evict(dbFileName, r.orgDir(orgID))
	path := r.orgDir(orgID)
	if err := os.RemoveAll(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove org dir %s: %w", path, err)
	}
	return nil
}

// SweepExisting walks <baseDir>/organization/ and opens the profile
// DB for every org folder that contains one, so newly-added
// migrations apply immediately at startup.
func (r *OrgDBRegistry) SweepExisting() error {
	orgsRoot := filepath.Join(r.baseDir, orgDirName)
	entries, err := os.ReadDir(orgsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read org root %s: %w", orgsRoot, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		orgID := entry.Name()
		if orgID == "" {
			continue
		}
		// Only open folders that already contain a profiles.db.
		if _, err := os.Stat(filepath.Join(orgsRoot, orgID, dbFileName)); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("sweep: stat %s: %w", orgID, err)
		}
		if _, err := r.For(orgID); err != nil {
			return fmt.Errorf("sweep: failed to open %s: %w", orgID, err)
		}
	}
	return nil
}

// KnownOrgIDs returns the org ids whose profile DB currently has a cached
// handle. Callers that want a point-in-time snapshot (e.g. admin
// dashboard fan-out) should invoke SweepExisting first to ensure every
// on-disk file has been opened.
func (r *OrgDBRegistry) KnownOrgIDs() []string {
	prefix := filepath.Join(r.baseDir, orgDirName) + "/"
	suffix := "/" + dbFileName
	keys := r.pool.Keys()
	ids := make([]string, 0, len(keys))
	for _, k := range keys {
		if strings.HasPrefix(k, prefix) && strings.HasSuffix(k, suffix) {
			mid := strings.TrimPrefix(k, prefix)
			mid = strings.TrimSuffix(mid, suffix)
			if mid != "" && !strings.Contains(mid, "/") {
				ids = append(ids, mid)
			}
		}
	}
	return ids
}

// orgDir returns the directory that holds this org's SQLite files.
func (r *OrgDBRegistry) orgDir(orgID string) string {
	return filepath.Join(r.baseDir, orgDirName, orgID)
}

const (
	// orgDirName is the single subdirectory under baseDir that groups
	// every organization's files.
	orgDirName = "organization"
	// dbFileName is the profile DB's filename inside an org folder.
	dbFileName = "profiles.db"
)
