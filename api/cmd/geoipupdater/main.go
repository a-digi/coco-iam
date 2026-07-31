// Command geoipupdater is the write side of
// plan/geoip-enrichment/plan.md — a small, long-lived process that
// checks on its own ticker whether geoip.db needs refreshing from
// MaxMind GeoLite2, and if so downloads, imports, and atomically
// swaps in a fresh copy. All real logic lives in
// github.com/a-digi/coco-sec/geoip/updater; this file is only wiring.
//
// Runs as its own executable, independent of the main coco-iam
// server. Deliberately never launched via cron, and — confirmed with
// the user — deliberately never wrapped in systemd either: the admin
// UI starts and stops this process directly (see geoip.Manager),
// tracking it via the PID file this process writes to itself below.
// That means nothing auto-restarts it if it crashes between admin
// actions — an accepted tradeoff, not an oversight.
package main

import (
	"context"
	"database/sql"
	"os"
	"os/signal"
	"syscall"

	"github.com/a-digi/coco-logger/logger"
	_ "github.com/mattn/go-sqlite3"

	"github.com/a-digi/coco-server/server"

	"github.com/a-digi/coco-iam/config"
	"github.com/a-digi/coco-sec/geoip"
	"github.com/a-digi/coco-sec/geoip/updater"
)

// mainDBPath matches the fixed path main.go itself constructs the
// main database at (dbmanager.NewDatabaseManager("users.db",
// "./data/db", ...)) — not configurable here, since this process
// only ever runs alongside that same main server installation.
const mainDBPath = "./data/db/users.db"

func main() {
	log, err := logger.NewLogger("geoip-updater.log", "data/logs/security")
	if err != nil {
		panic(err)
	}

	cfgBytes, err := config.ReadConfigFile("config.json")
	if err != nil {
		log.Error("geoip-updater: failed to read config.json: %v", err)
		os.Exit(1)
	}

	cfg, err := geoip.LoadConfig(cfgBytes)
	if err != nil {
		log.Error("geoip-updater: invalid config: %v", err)
		os.Exit(1)
	}

	cfg = cfg.WithSettings(loadSettings(log))

	if !cfg.Enabled {
		log.Warning("geoip-updater: geoip is not enabled (neither config.json nor the admin-configured settings) - nothing to do, exiting")
		return
	}

	if err := server.WritePID(cfg.PIDFile); err != nil {
		log.Error("geoip-updater: failed to write PID file %s: %v", cfg.PIDFile, err)
		os.Exit(1)
	}
	defer func() {
		if err := server.RemovePID(cfg.PIDFile); err != nil {
			log.Warning("geoip-updater: failed to remove PID file %s: %v", cfg.PIDFile, err)
		}
	}()

	migrationsPath, err := config.ExtractGeoIPMigrationsToTemp()
	if err != nil {
		log.Error("geoip-updater: %v", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// forceCh carries the admin UI's manual "Sync now" trigger —
	// geoip.Manager.SyncNow() signals this process with SIGUSR1, and
	// Updater.Run treats a receive here as "pull immediately",
	// bypassing the normal pull_interval_hours staleness check.
	forceCh := make(chan os.Signal, 1)
	signal.Notify(forceCh, syscall.SIGUSR1)

	updater.New(cfg, migrationsPath, log).Run(ctx, forceCh)
}

// loadSettings reads the admin-editable security_geoip_settings row from the
// main database — a one-shot read at startup, not hot-reloaded while
// this process runs (an admin changing credentials requires a
// stop/start to pick them up, the same way any other config-at-launch
// process works). A missing table (main server has never run its own
// migrations yet on this install) or missing row (nobody has visited
// the settings page yet) are both treated as "no settings saved" —
// cfg.WithSettings is then a no-op, falling back entirely to
// config.json's own static defaults.
func loadSettings(log logger.Logger) geoip.Settings {
	db, err := sql.Open("sqlite3", mainDBPath)
	if err != nil {
		log.Warning("geoip-updater: could not open main database %s: %v (using config.json defaults only)", mainDBPath, err)
		return geoip.Settings{}
	}
	defer db.Close()

	settings, err := geoip.NewSettingsQueryRepo(db).LoadSettings()
	if err != nil {
		log.Warning("geoip-updater: could not load settings from the main database: %v (using config.json defaults only)", err)
		return geoip.Settings{}
	}
	return settings
}
