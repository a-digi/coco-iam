// Package dbregistry owns the per-organization SQLite file that holds
// one organization's OAuth tokens — authorization codes, refresh
// tokens, and auth-request state — written by the public OAuth
// provider endpoints. One SQLite file per organization, created
// lazily on first use or eagerly via Provision.
//
// On-disk layout (alongside the other per-org files):
//
//	<baseDir>/organization/<orgID>/oauth.db
//	<baseDir>/organization/<orgID>/users.db
//	<baseDir>/organization/<orgID>/profiles.db
//	<baseDir>/organization/<orgID>/api_credentials.db
//
// Tokens belong to an application but are stored in the org folder so
// the whole tenant's data archives together on org delete. The
// deletion consumer moves the full folder into
// <baseDir>/deleted/organization/<orgID>/ without any
// registry-specific hook.
package dbregistry

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/a-digi/coco-orm/orm"
)

// ContextBagKey is the key under which a single OrgOAuthDBRegistry
// instance is stored in the app's ContextBag.
const ContextBagKey = "oauthserver.db_registry"

// OrgOAuthDBRegistry opens and caches per-organization OAuth database
// handles. All calls are safe for concurrent use.
type OrgOAuthDBRegistry struct {
	baseDir        string
	migrationsPath string

	mu    sync.Mutex
	cache map[string]*orm.DatabaseManager
}

// New builds a registry rooted at baseDir ("./data/db" in production).
// migrationsPath is the filesystem path to the already-extracted
// per-org OAuth migration files — callers use
// config.ExtractOrgOAuthMigrationsToTemp() to obtain it at startup.
func New(baseDir, migrationsPath string) *OrgOAuthDBRegistry {
	return &OrgOAuthDBRegistry{
		baseDir:        baseDir,
		migrationsPath: migrationsPath,
		cache:          make(map[string]*orm.DatabaseManager),
	}
}

// For returns the DatabaseManager for the given organization id. If
// the database file does not exist yet it is created and migrations
// are run; the handle is cached for subsequent calls.
func (r *OrgOAuthDBRegistry) For(orgID string) (*orm.DatabaseManager, error) {
	if orgID == "" {
		return nil, fmt.Errorf("org id must not be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if mgr, ok := r.cache[orgID]; ok {
		return mgr, nil
	}

	mgr, err := orm.NewDatabaseManager(dbFileName, r.orgDir(orgID), []string{r.migrationsPath})
	if err != nil {
		return nil, fmt.Errorf("failed to open org oauth db for %s: %w", orgID, err)
	}
	if err := mgr.SyncMigrations(); err != nil {
		return nil, fmt.Errorf("failed to sync migrations on org oauth db %s: %w", orgID, err)
	}
	r.cache[orgID] = mgr
	return mgr, nil
}

// Provision forces creation + migration of the org's OAuth database.
// Called from the organizations PostEventListener so the file exists
// even before the first OAuth request.
func (r *OrgOAuthDBRegistry) Provision(orgID string) error {
	_, err := r.For(orgID)
	return err
}

// Destroy closes any cached handle and removes the org folder. Used
// only on hard-delete of an organization — the deletion consumer
// normally moves the folder instead so the data is archived.
func (r *OrgOAuthDBRegistry) Destroy(orgID string) error {
	if orgID == "" {
		return fmt.Errorf("org id must not be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if mgr, ok := r.cache[orgID]; ok {
		if mgr.Connector != nil && mgr.Connector.DB != nil {
			_ = mgr.Connector.DB.Close()
		}
		delete(r.cache, orgID)
	}

	path := r.orgDir(orgID)
	if err := os.RemoveAll(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove org dir %s: %w", path, err)
	}
	return nil
}

// SweepExisting walks <baseDir>/organization/ and opens the OAuth DB
// for every org folder that contains one, so newly-added migrations
// apply immediately at startup.
func (r *OrgOAuthDBRegistry) SweepExisting() error {
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

// KnownOrgIDs returns all org IDs that currently have a cached
// database handle. Used by the background archiver to enumerate
// which orgs to sweep for expired / consumed tokens.
func (r *OrgOAuthDBRegistry) KnownOrgIDs() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	ids := make([]string, 0, len(r.cache))
	for id := range r.cache {
		ids = append(ids, id)
	}
	return ids
}

// orgDir returns the directory that holds this org's SQLite files.
func (r *OrgOAuthDBRegistry) orgDir(orgID string) string {
	return filepath.Join(r.baseDir, orgDirName, orgID)
}

const (
	// orgDirName is the single subdirectory under baseDir that groups
	// every organization's files. Shared with the users + profile DB
	// registries so all three speak the same on-disk layout.
	orgDirName = "organization"
	// dbFileName is the OAuth DB's filename inside an org folder.
	dbFileName = "oauth.db"
)
