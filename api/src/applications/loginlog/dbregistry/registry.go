// Package dbregistry owns the per-application SQLite file that holds
// one application's end-user login-attempt history, plus a
// dbarchive.Archiver rotating it once it crosses the configured
// threshold - potentially hundreds of these, one per application, all
// sharing the same generalized Archiver type from
// plan/login-audit-log/plan.md Step 1.
//
// On-disk layout (nested under the owning org, alongside its other
// per-org files, and under the application's own globally-unique id
// as a belt-and-suspenders guard against any slug collision):
//
//	<baseDir>/organization/<orgID>/applications/<appID>/<slug>_login.db
//	<baseDir>/organization/<orgID>/applications/<appID>/archives/<slug>_login-<ts>.db
//	<baseDir>/organization/<orgID>/users.db
//
// Unlike apicred_dbregistry.For, this registry's For cannot lazily
// construct a path from an application id alone - the physical path
// also depends on the owning organization id and the application's
// slug, neither of which are derivable from the id by itself. Every
// application must go through Provision (at creation time) or
// SweepExisting (at startup, for applications provisioned in a prior
// run) before For succeeds. See plan/login-audit-log/plan.md Step 6.
package dbregistry

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/a-digi/coco-logger/logger"
	"github.com/a-digi/coco-orm/orm"

	loginlog_persistent "github.com/a-digi/coco-iam/src/applications/loginlog/repository/persistent"
	org_user_dbregistry "github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-iam/src/orgrouter"
	"github.com/a-digi/coco-iam/src/security/dbarchive"
	"github.com/a-digi/coco-iam/src/security/dbhandle"
)

// ContextBagKey is the key under which a single Registry instance is
// stored in the app's ContextBag.
const ContextBagKey = "applications.login_log.db_registry"

const (
	// orgDirName mirrors the same subdirectory name every other
	// per-org registry in this codebase uses.
	orgDirName = "organization"
	// appsDirName groups every application's own subdirectory under
	// one organization.
	appsDirName = "applications"
	// archiveDirName is nested under the application's own directory,
	// not shared globally - keeps a rotated-out generation moving
	// with its owning org/application if either is ever archived or
	// deleted wholesale.
	archiveDirName = "archives"
	// loginDBSuffix names every per-application login-log file -
	// "<slug>_login.db". SweepExisting parses a directory's actual
	// filename against this suffix to recover the slug at startup,
	// rather than needing a separate slug lookup.
	loginDBSuffix = "_login.db"
)

// entry is everything the registry holds for one provisioned
// application: the live DatabaseManager/handle pair, the archiver
// rotating it, and the CancelFunc that stops that archiver's Run
// goroutine on Destroy.
type entry struct {
	manager  *orm.DatabaseManager
	handle   *dbhandle.Handle
	archiver *dbarchive.Archiver
	cancel   context.CancelFunc
}

// Registry opens and caches per-application login-log database
// handles, and owns one dbarchive.Archiver goroutine per provisioned
// application. All calls are safe for concurrent use.
type Registry struct {
	baseDir         string
	migrationsPath  string
	threshold       int64
	orgUserRegistry *org_user_dbregistry.OrgUserDBRegistry
	log             logger.Logger

	mu    sync.Mutex
	cache map[string]*entry // keyed by application id
}

// New builds a registry rooted at baseDir ("./data/db" in production).
// migrationsPath is the filesystem path to the already-extracted
// per-application login-log migration files — callers use
// config.ExtractApplicationLoginMigrationsToTemp() to obtain it at
// startup. orgUserRegistry resolves an owning organization's users.db
// (which holds application_login_archives) — see recorderFor.
func New(baseDir, migrationsPath string, threshold int64, orgUserRegistry *org_user_dbregistry.OrgUserDBRegistry, log logger.Logger) *Registry {
	return &Registry{
		baseDir:         baseDir,
		migrationsPath:  migrationsPath,
		threshold:       threshold,
		orgUserRegistry: orgUserRegistry,
		log:             log,
		cache:           make(map[string]*entry),
	}
}

// KnownAppIDs returns the application ids whose login-log DB
// currently has a cached handle — mirrors
// organizations/users/dbregistry.Registry.KnownOrgIDs() exactly, same
// "point-in-time snapshot of what's currently provisioned" contract.
// Callers that want every application (not just ones already
// provisioned this run) should invoke SweepExisting first. Used by
// the IP-bans "which accounts did this IP try" fan-out — see
// plan/ip-ban-accounts/plan.md.
func (r *Registry) KnownAppIDs() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	ids := make([]string, 0, len(r.cache))
	for appID := range r.cache {
		ids = append(ids, appID)
	}
	return ids
}

// For returns the cached handle for applicationID — a pure cache
// lookup (see the package doc comment for why this can't lazily
// construct one). Returns an error if applicationID was never
// provisioned.
func (r *Registry) For(applicationID string) (*dbhandle.Handle, error) {
	if applicationID == "" {
		return nil, fmt.Errorf("application id must not be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	e, ok := r.cache[applicationID]
	if !ok {
		return nil, fmt.Errorf("application %s: login-log database not provisioned", applicationID)
	}
	return e.handle, nil
}

// Provision force-creates and migrates applicationID's login-log
// database and starts its archiver goroutine, if not already done.
// Called from application-creation (see
// plan/login-audit-log/plan.md Step 7) — best-effort at that call
// site, same as ensureKeypair/reserveApplicationSlug.
func (r *Registry) Provision(applicationID, organizationID, slug string) error {
	_, err := r.provisionEntry(applicationID, organizationID, slug)
	return err
}

// Destroy stops applicationID's archiver goroutine and closes its
// handle. Deliberately does NOT remove the on-disk file or directory —
// whether a deleted application's login history should be archived
// and kept or actually deleted is an open question (see
// plan/login-audit-log/plan.md's open questions) not yet resolved, so
// this only stops active tracking, leaving the data untouched. Not
// currently wired into any application-deletion flow — see the same
// open question.
func (r *Registry) Destroy(applicationID string) error {
	if applicationID == "" {
		return fmt.Errorf("application id must not be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	e, ok := r.cache[applicationID]
	if !ok {
		return nil
	}
	e.cancel()
	if e.manager != nil && e.manager.Connector != nil && e.manager.Connector.DB != nil {
		_ = e.manager.Connector.DB.Close()
	}
	delete(r.cache, applicationID)
	return nil
}

// SweepExisting walks <baseDir>/organization/*/applications/* and
// provisions (opens + migrates + starts an archiver for) every
// application directory that already holds a <slug>_login.db file —
// so a server restart resumes rotation-checking for every existing
// application without waiting on its next login. The slug is
// recovered from the file's own name (see loginDBSuffix), since this
// registry has no other persisted record of which slug an application
// used.
func (r *Registry) SweepExisting() error {
	orgsRoot := filepath.Join(r.baseDir, orgDirName)
	orgEntries, err := os.ReadDir(orgsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read org root %s: %w", orgsRoot, err)
	}

	for _, orgEntry := range orgEntries {
		if !orgEntry.IsDir() {
			continue
		}
		orgID := orgEntry.Name()

		appsRoot := filepath.Join(orgsRoot, orgID, appsDirName)
		appEntries, err := os.ReadDir(appsRoot)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("sweep: read apps root %s: %w", appsRoot, err)
		}

		for _, appEntry := range appEntries {
			if !appEntry.IsDir() {
				continue
			}
			applicationID := appEntry.Name()

			slug, err := findLoginDBSlug(filepath.Join(appsRoot, applicationID))
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return fmt.Errorf("sweep: find login db for %s: %w", applicationID, err)
			}
			if slug == "" {
				continue
			}

			if _, err := r.provisionEntry(applicationID, orgID, slug); err != nil {
				return fmt.Errorf("sweep: provision %s: %w", applicationID, err)
			}
		}
	}
	return nil
}

// provisionEntry is the shared construction path for Provision and
// SweepExisting: cache hit, or build a fresh DatabaseManager/handle/
// archiver triple and start the archiver's Run goroutine.
func (r *Registry) provisionEntry(applicationID, organizationID, slug string) (*entry, error) {
	if applicationID == "" || organizationID == "" || slug == "" {
		return nil, fmt.Errorf("application id, organization id, and slug must not be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if e, ok := r.cache[applicationID]; ok {
		return e, nil
	}

	dbDir := r.appDir(organizationID, applicationID)
	dbName := slug + loginDBSuffix

	manager, err := orm.NewDatabaseManager(dbName, dbDir, []string{r.migrationsPath})
	if err != nil {
		return nil, fmt.Errorf("failed to open login-log db for application %s: %w", applicationID, err)
	}
	if err := manager.SyncMigrations(); err != nil {
		return nil, fmt.Errorf("failed to sync migrations for application %s: %w", applicationID, err)
	}

	handle, err := dbhandle.New(manager.Connector.DB)
	if err != nil {
		return nil, fmt.Errorf("failed to wrap handle for application %s: %w", applicationID, err)
	}

	recorder, err := r.recorderFor(organizationID, applicationID)
	if err != nil {
		return nil, fmt.Errorf("failed to build archive recorder for application %s: %w", applicationID, err)
	}

	archiveDir := filepath.Join(dbDir, archiveDirName)
	archiver := dbarchive.New(handle, manager, recorder, dbName, dbDir, r.migrationsPath, archiveDir, r.threshold, r.log)

	ctx, cancel := context.WithCancel(context.Background())
	go archiver.Run(ctx)

	e := &entry{manager: manager, handle: handle, archiver: archiver, cancel: cancel}
	r.cache[applicationID] = e
	return e, nil
}

// recorderFor resolves organizationID's own users.db (which holds
// application_login_archives) and builds a RegistryRecorder scoped to
// applicationID.
func (r *Registry) recorderFor(organizationID, applicationID string) (dbarchive.RegistryRecorder, error) {
	orgDB, err := orgrouter.ForOrg(r.orgUserRegistry, organizationID)
	if err != nil {
		return nil, err
	}
	return loginlog_persistent.NewArchiveRecorder(orgDB, applicationID), nil
}

// appDir returns the directory that holds one application's own
// SQLite files.
func (r *Registry) appDir(organizationID, applicationID string) string {
	return filepath.Join(r.baseDir, orgDirName, organizationID, appsDirName, applicationID)
}

// findLoginDBSlug scans appDir for a "*_login.db" file and returns the
// slug prefix - the reverse of dbName construction in provisionEntry.
// Returns an os.ErrNotExist-wrapped error if appDir itself doesn't
// exist yet, and "", nil (not an error) if the directory exists but
// holds no login-log file yet.
func findLoginDBSlug(appDir string) (string, error) {
	entries, err := os.ReadDir(appDir)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if name := e.Name(); strings.HasSuffix(name, loginDBSuffix) {
			return strings.TrimSuffix(name, loginDBSuffix), nil
		}
	}
	return "", nil
}
