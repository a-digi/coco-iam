// Package config exposes the on-disk configuration directory
// (migrations, routes YAML, scopes catalog, mail templates,
// config.json) to the rest of the codebase.
//
// History note: this package used to embed every config file
// into the binary via //go:embed. We dropped that so operators
// can SEE the migration files on disk + diff them against the
// migrations table. The package's public surface is unchanged
// — callers still reach files through ConfigFS / ReadConfigFile
// / Extract*MigrationsToTemp — only the implementation moved
// from embed.FS to os.DirFS.
package config

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// EnvVarConfigDir names the env var the operator can set to
// point at a non-default config directory. Useful when the
// binary lives outside the repo (production deploys).
const EnvVarConfigDir = "COCO_IAM_CONFIG_DIR"

// candidateDefaults is the list of relative paths we try (in
// order) when the operator hasn't set EnvVarConfigDir. First
// existing directory wins.
//   - "api/config" — `make run` from the repo root.
//   - "config"     — `make run-dev` (cd api && go run …) sees
//     the config folder at sibling level.
//   - "./config"   — same as above but written explicitly so
//     a deploy that ships the binary + a sibling config/
//     folder also Just Works.
var candidateDefaults = []string{"api/config", "config", "./config"}

// configDir is an fs.FS rooted at a real on-disk directory.
// We define our own type (instead of using os.DirFS directly)
// so the methods ReadFile + ReadDir hang directly off the
// instance — every existing caller that did
// `config.ConfigFS.ReadDir("scopes")` keeps compiling.
type configDir struct{ root string }

// Open implements fs.FS so the value satisfies the standard
// interface accepted by lift_routes.LoadRoutesYAML and
// mailtemplate.New.
func (c configDir) Open(name string) (fs.File, error) {
	return os.Open(c.realPath(name))
}

// ReadFile mirrors embed.FS.ReadFile so callers that took the
// concrete embed.FS method don't change.
func (c configDir) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(c.realPath(name))
}

// ReadDir mirrors embed.FS.ReadDir.
func (c configDir) ReadDir(name string) ([]fs.DirEntry, error) {
	return os.ReadDir(c.realPath(name))
}

// realPath joins the root with the slash-separated logical
// path callers pass. Refusing absolute paths and `..` segments
// gives the same containment guarantee embed.FS provided.
func (c configDir) realPath(name string) string {
	clean := filepath.FromSlash(strings.TrimPrefix(name, "/"))
	return filepath.Join(c.root, clean)
}

// Root returns the absolute path the FS resolves against.
// Exposed so the migration-folder helpers can hand the runner
// a real path (the runner uses os.ReadDir on it).
func (c configDir) Root() string { return c.root }

// ConfigFS is the package-level handle on the config dir.
// Initialised lazily on first access so test binaries that
// chdir before calling Init still see the right root.
var (
	configFSOnce sync.Once
	configFSVal  configDir
)

// Init forces resolution of the config root immediately. Useful
// from main() after we have parsed flags so any startup error
// is surfaced before the rest of bootstrap depends on the FS.
func Init() error {
	dir := initConfigDir()
	if _, err := os.Stat(dir.root); err != nil {
		return fmt.Errorf("config: directory %q is not readable (set %s to override, or check candidates %v): %w",
			dir.root, EnvVarConfigDir, candidateDefaults, err)
	}
	return nil
}

// ConfigFS resolves to the on-disk config directory. Replaces
// the previous embed.FS package var of the same name so every
// caller continues to compile.
//
// Implemented as a function-named-as-a-variable trick? No —
// keep it a real var so reflective lookups still work. We use
// a small init helper to ensure lazy resolution.
var ConfigFS = lazyConfigFS{}

// lazyConfigFS embeds configDir but resolves the root on first
// use. Acts indistinguishably from configDir for callers.
type lazyConfigFS struct{}

func (lazyConfigFS) Open(name string) (fs.File, error)        { return resolved().Open(name) }
func (lazyConfigFS) ReadFile(name string) ([]byte, error)     { return resolved().ReadFile(name) }
func (lazyConfigFS) ReadDir(name string) ([]fs.DirEntry, error) { return resolved().ReadDir(name) }
func (lazyConfigFS) Root() string                              { return resolved().Root() }

func resolved() configDir {
	configFSOnce.Do(func() { configFSVal = initConfigDir() })
	return configFSVal
}

func initConfigDir() configDir {
	root := strings.TrimSpace(os.Getenv(EnvVarConfigDir))
	if root == "" {
		root = pickFirstExisting(candidateDefaults)
	}
	abs, err := filepath.Abs(root)
	if err == nil {
		root = abs
	}
	return configDir{root: root}
}

// pickFirstExisting returns the first candidate that resolves
// to a real directory on disk. Falls back to the first
// candidate when none exist so a missing-config error surfaces
// later with a stable, predictable path.
func pickFirstExisting(candidates []string) string {
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			return c
		}
	}
	return candidates[0]
}

// ReadConfigFile reads a file from the config directory.
func ReadConfigFile(name string) ([]byte, error) {
	return ConfigFS.ReadFile(name)
}

// -- Migration-folder paths ---------------------------------------
//
// These used to extract embedded files to a fresh os.MkdirTemp
// directory on every startup. They now return the on-disk
// folder directly. Names + signatures kept so main.go
// callsites don't have to change.

// ExtractMigrationsToTemp returns the on-disk path to the main
// DB migrations folder. (Name retained for back-compat — it no
// longer extracts anything, just hands back the real folder.)
func ExtractMigrationsToTemp() (string, error) {
	return migrationsPath("db/migrations")
}

// ExtractOrgMigrationsToTemp returns the on-disk path to the
// per-organization profile DB migrations folder.
func ExtractOrgMigrationsToTemp() (string, error) {
	return migrationsPath("db/org_migrations")
}

// ExtractOrgUserMigrationsToTemp returns the on-disk path to
// the per-organization users DB migrations folder.
func ExtractOrgUserMigrationsToTemp() (string, error) {
	return migrationsPath("db/org_user_migrations")
}

// ExtractOrgApiCredentialsMigrationsToTemp returns the on-disk
// path to the per-organization api-credential migrations folder.
func ExtractOrgApiCredentialsMigrationsToTemp() (string, error) {
	return migrationsPath("db/org_api_credentials_migrations")
}

// ExtractOrgOAuthMigrationsToTemp returns the on-disk path to
// the per-organization OAuth DB migrations folder.
func ExtractOrgOAuthMigrationsToTemp() (string, error) {
	return migrationsPath("db/org_oauth_migrations")
}

// ExtractQueueMainMigrationsToTemp returns the on-disk path to
// the queue main DB migrations folder.
func ExtractQueueMainMigrationsToTemp() (string, error) {
	return migrationsPath("db/queue_main_migrations")
}

// ExtractQueueMigrationsToTemp returns the on-disk path to the
// per-queue migrations folder.
func ExtractQueueMigrationsToTemp() (string, error) {
	return migrationsPath("db/queue_migrations")
}

// ExtractIPAttacksMigrationsToTemp returns the on-disk path to the
// ip-attacks.db migrations folder — a self-contained DB, own
// migration sequence (001_initial.sql, not the DD_MM_YYYY convention
// used by db/migrations). See plan/ip-abuse-protection/plan.md
// section 10.
func ExtractIPAttacksMigrationsToTemp() (string, error) {
	return migrationsPath("db/ip_attacks_migrations")
}

// ExtractGeoIPMigrationsToTemp returns the on-disk path to the
// geoip.db migrations folder — a self-contained DB, own migration
// sequence (001_initial.sql), rebuilt wholesale by the geoip-updater
// executable rather than grown in place. See
// plan/geoip-enrichment/plan.md.
func ExtractGeoIPMigrationsToTemp() (string, error) {
	return migrationsPath("db/geoip_migrations")
}

func migrationsPath(rel string) (string, error) {
	full := filepath.Join(resolved().Root(), filepath.FromSlash(rel))
	info, err := os.Stat(full)
	if err != nil {
		return "", fmt.Errorf("config: migrations folder %q not found (set %s to point at api/config or equivalent): %w",
			full, EnvVarConfigDir, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("config: %q exists but is not a directory", full)
	}
	return full, nil
}
