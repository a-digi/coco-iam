package updater

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	dbmanager "github.com/a-digi/coco-orm/orm"
	_ "github.com/mattn/go-sqlite3"

	"github.com/a-digi/coco-logger/logger"

	"github.com/a-digi/coco-iam/src/security/geoip"
)

// minRowCountRatio guards against a truncated/corrupted download
// silently replacing good data with garbage. There is no rollback
// once a fresh build is renamed into place (geoip.db keeps no
// history, by design), so this is the one check that has to hold. See
// plan/geoip-enrichment/plan.md's security considerations.
const minRowCountRatio = 0.8

// Updater owns the "check every interval, pull if due" loop — the
// write side of plan/geoip-enrichment/plan.md, running as its own
// long-lived process (api/cmd/geoipupdater), never as a goroutine
// inside the main coco-iam server and never via cron.
type Updater struct {
	cfg            geoip.Config
	migrationsPath string
	log            logger.Logger
	downloader     *Downloader
}

// New builds an Updater. migrationsPath is resolved by the caller
// (api/cmd/geoipupdater/main.go, via config.ExtractGeoIPMigrationsToTemp)
// rather than by this package, so api/src/security/geoip/updater has
// no dependency on api/config — mirrors dbarchive.New's own
// migrationsPath parameter.
func New(cfg geoip.Config, migrationsPath string, log logger.Logger) *Updater {
	return &Updater{
		cfg:            cfg,
		migrationsPath: migrationsPath,
		log:            log,
		downloader:     NewDownloader(cfg.MaxMindAccountID, cfg.MaxMindLicenseKey),
	}
}

// Run ticks on cfg.CheckInterval() until ctx is done, pulling fresh
// data whenever the last successful pull is older than
// cfg.PullInterval(). Ticks once immediately before entering the
// loop, so a fresh deploy pulls right away instead of waiting up to a
// full check interval for its first data.
//
// forceSync is the admin UI's manual "Sync now" trigger (geoip.Manager
// signals this process with SIGUSR1; api/cmd/geoipupdater/main.go
// turns that into a value on this channel) — receiving on it pulls
// immediately, deliberately bypassing tryPull's staleness gate, since
// that's the entire point of a manual sync. A nil channel (e.g. in
// tests that don't exercise this path) simply never fires, same as
// any other nil-channel receive in a select.
func (u *Updater) Run(ctx context.Context, forceSync <-chan os.Signal) {
	u.tryPull(ctx)

	ticker := time.NewTicker(u.cfg.CheckInterval())
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			u.tryPull(ctx)
		case <-forceSync:
			if err := u.pullAndImport(ctx); err != nil {
				u.warnf("geoip-updater: forced sync failed, will retry next check: %v", err)
			}
		}
	}
}

// tryPull is a no-op unless the last successful pull is older than
// cfg.PullInterval() — this is the split between "how often do we
// check the clock" (cfg.CheckInterval, the outer loop above) and "how
// stale must the data be before we actually re-download" (this
// check), so a short check interval never turns into hammering
// MaxMind for data that only changes twice a week upstream.
func (u *Updater) tryPull(ctx context.Context) {
	if time.Since(readLastPulledAt(u.cfg.DBPath)) < u.cfg.PullInterval() {
		return
	}
	if err := u.pullAndImport(ctx); err != nil {
		u.warnf("geoip-updater: pull failed, will retry next check: %v", err)
	}
}

// pullAndImport downloads both GeoLite2 CSV editions, imports them
// into a brand-new SQLite file, sanity-checks the result, and
// atomically renames it over the live geoip.db. Every step before the
// final rename is purely preparatory and never touches the live
// file — a failure at any point here leaves cfg.DBPath completely
// untouched, so a bad pull simply gets retried at the next check
// rather than corrupting what's already there.
func (u *Updater) pullAndImport(ctx context.Context) error {
	start := time.Now()

	tmpDir, err := os.MkdirTemp("", "geoip-pull-")
	if err != nil {
		return fmt.Errorf("geoip-updater: create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	countryZip := filepath.Join(tmpDir, "country.zip")
	if err := u.downloader.Download(ctx, editionCountryCSV, countryZip); err != nil {
		return err
	}
	asnZip := filepath.Join(tmpDir, "asn.zip")
	if err := u.downloader.Download(ctx, editionASNCSV, asnZip); err != nil {
		return err
	}

	countryDir := filepath.Join(tmpDir, "country")
	if err := unzip(countryZip, countryDir); err != nil {
		return fmt.Errorf("geoip-updater: unzip country csv: %w", err)
	}
	asnDir := filepath.Join(tmpDir, "asn")
	if err := unzip(asnZip, asnDir); err != nil {
		return fmt.Errorf("geoip-updater: unzip asn csv: %w", err)
	}

	buildDir := filepath.Dir(u.cfg.DBPath)
	buildName := filepath.Base(u.cfg.DBPath) + ".building-" + uuid.New().String()
	buildPath := filepath.Join(buildDir, buildName)
	defer func() { _ = os.Remove(buildPath) }() // no-op once successfully renamed away

	manager, err := dbmanager.NewDatabaseManager(buildName, buildDir, []string{u.migrationsPath})
	if err != nil {
		return fmt.Errorf("geoip-updater: create fresh db: %w", err)
	}
	if err := manager.SyncMigrations(); err != nil {
		_ = manager.Connector.Close()
		return fmt.Errorf("geoip-updater: migrate fresh db: %w", err)
	}

	db := manager.Connector.DB
	countryCount, err := importCountryCSVDir(db, countryDir)
	if err != nil {
		_ = manager.Connector.Close()
		return fmt.Errorf("geoip-updater: %w", err)
	}
	asnCount, err := importASNCSVDir(db, asnDir)
	if err != nil {
		_ = manager.Connector.Close()
		return fmt.Errorf("geoip-updater: %w", err)
	}

	if err := u.sanityCheck(countryCount, asnCount); err != nil {
		_ = manager.Connector.Close()
		return err
	}

	if _, err := db.Exec(
		`INSERT INTO geoip_meta (key, value) VALUES ('last_pulled_at', ?)`,
		time.Now().UTC().Format(time.RFC3339),
	); err != nil {
		_ = manager.Connector.Close()
		return fmt.Errorf("geoip-updater: write last_pulled_at: %w", err)
	}

	if err := manager.Connector.Close(); err != nil {
		return fmt.Errorf("geoip-updater: close fresh db: %w", err)
	}

	if err := os.Rename(buildPath, u.cfg.DBPath); err != nil {
		return fmt.Errorf("geoip-updater: rename into place: %w", err)
	}

	u.infof("geoip-updater: pull complete - %d country ranges, %d asn ranges, took %s",
		countryCount, asnCount, time.Since(start).Round(time.Millisecond))
	return nil
}

// sanityCheck refuses the fresh build if either dataset is empty
// outright, or looks suspiciously small compared to the current live
// file (below minRowCountRatio of its row count). The ratio check is
// skipped for a dataset the previous generation had zero rows for —
// i.e. this is the very first pull ever, nothing to compare against.
func (u *Updater) sanityCheck(newCountryCount, newASNCount int) error {
	if newCountryCount == 0 {
		return fmt.Errorf("geoip-updater: sanity check failed: freshly imported country ranges is empty")
	}
	if newASNCount == 0 {
		return fmt.Errorf("geoip-updater: sanity check failed: freshly imported asn ranges is empty")
	}

	oldCountryCount, oldASNCount := previousRowCounts(u.cfg.DBPath)
	if oldCountryCount > 0 && float64(newCountryCount) < float64(oldCountryCount)*minRowCountRatio {
		return fmt.Errorf("geoip-updater: sanity check failed: new country range count %d is below %.0f%% of previous count %d",
			newCountryCount, minRowCountRatio*100, oldCountryCount)
	}
	if oldASNCount > 0 && float64(newASNCount) < float64(oldASNCount)*minRowCountRatio {
		return fmt.Errorf("geoip-updater: sanity check failed: new asn range count %d is below %.0f%% of previous count %d",
			newASNCount, minRowCountRatio*100, oldASNCount)
	}
	return nil
}

// readLastPulledAt reads geoip_meta.last_pulled_at from the current
// live file at dbPath, or the zero Time if the file doesn't exist yet,
// can't be opened, or has no such row — every one of those is
// legitimately "never pulled", not an error worth surfacing.
// Deliberately os.Stat-guards before opening: sql.Open + a failed
// query against a nonexistent path would otherwise leave a stray
// empty SQLite file sitting at dbPath as a side effect.
func readLastPulledAt(dbPath string) time.Time {
	if _, err := os.Stat(dbPath); err != nil {
		return time.Time{}
	}
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return time.Time{}
	}
	defer db.Close()

	var raw string
	if err := db.QueryRow(`SELECT value FROM geoip_meta WHERE key = 'last_pulled_at'`).Scan(&raw); err != nil {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

// previousRowCounts reads the current live file's row counts for
// sanityCheck's comparison — (0, 0) if the file doesn't exist yet or
// can't be read, which sanityCheck already treats as "nothing to
// compare against, skip the ratio check."
func previousRowCounts(dbPath string) (countryCount, asnCount int) {
	if _, err := os.Stat(dbPath); err != nil {
		return 0, 0
	}
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return 0, 0
	}
	defer db.Close()

	_ = db.QueryRow(`SELECT COUNT(*) FROM geoip_country_ranges`).Scan(&countryCount)
	_ = db.QueryRow(`SELECT COUNT(*) FROM geoip_asn_ranges`).Scan(&asnCount)
	return countryCount, asnCount
}

func (u *Updater) warnf(format string, args ...interface{}) {
	if u.log != nil {
		u.log.Warning(format, args...)
	}
}

func (u *Updater) infof(format string, args ...interface{}) {
	if u.log != nil {
		u.log.Info(format, args...)
	}
}
