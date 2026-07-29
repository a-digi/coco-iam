package geoip

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/a-digi/coco-logger/logger"
	"github.com/a-digi/coco-server/server"
)

// ErrNotRunning is returned by SyncNow when there's no geoip-updater
// process currently running to signal. Exported so the handler
// package can distinguish it from other failures and map it to 409.
var ErrNotRunning = errors.New("geoip: updater is not running")

// Status is Manager.Status's result — the admin UI's view of the
// geoip-updater process.
type Status struct {
	Running           bool
	PID               int
	LastPulledAt      time.Time // zero if unknown (never pulled, or geoip.db doesn't exist yet)
	CountryRangeCount int       // row count of geoip_country_ranges in the live geoip.db, 0 if unknown
	ASNRangeCount     int       // row count of geoip_asn_ranges in the live geoip.db, 0 if unknown
}

// Manager starts, stops, and reports on the geoip-updater process —
// the admin-UI-controlled alternative to a systemd unit, confirmed
// with the user. See plan/geoip-enrichment/plan.md's "Process
// control" section for the full design rationale, in particular why
// the PID file (not in-memory state) is what makes Stop work even
// after this admin server process itself has been restarted.
type Manager struct {
	binaryPath string
	pidFile    string
	dbPath     string
	log        logger.Logger
}

// NewManager builds a Manager. binaryPath and pidFile should come
// from Config.UpdaterBinaryPath/Config.PIDFile; dbPath from
// Config.DBPath (used only for the best-effort LastPulledAt read).
func NewManager(binaryPath, pidFile, dbPath string, log logger.Logger) *Manager {
	return &Manager{binaryPath: binaryPath, pidFile: pidFile, dbPath: dbPath, log: log}
}

// Start execs the geoip-updater binary, detached into its own session
// so it survives this admin server process's own exit/restart, and
// refuses if an instance is already running (tracked via the PID
// file, not in-memory state — see the package doc comment). The
// binary path is always the static, operator-configured
// Config.UpdaterBinaryPath — never derived from request input, since
// that would make this an arbitrary-command-execution primitive.
func (m *Manager) Start() (pid int, err error) {
	if running, existingPID, _ := m.processStatus(); running {
		return existingPID, fmt.Errorf("geoip: updater already running (pid %d)", existingPID)
	}

	cmd := exec.Command(m.binaryPath)
	// Setsid detaches the child into its own session so it is never
	// killed as a side effect of this admin server's own process
	// group receiving a signal (e.g. Ctrl-C in dev, or a supervisor
	// signalling the whole group on redeploy) — it must outlive this
	// specific admin-server process instance.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("geoip: start updater: %w", err)
	}

	// Reap the child's exit status whenever it happens, without
	// blocking Start() on it. Setsid alone doesn't prevent a zombie
	// process entry while this admin process is still alive — not
	// calling Wait() at all would leak one every time the updater
	// exits, including via Stop()'s own SIGTERM.
	go func() { _ = cmd.Wait() }()

	return cmd.Process.Pid, nil
}

// Stop signals the running updater process (found via the PID file,
// written by the process itself at its own startup — see
// cmd/geoipupdater/main.go) to shut down gracefully. A no-op, not an
// error, if nothing is currently running.
func (m *Manager) Stop() error {
	running, pid, err := m.processStatus()
	if err != nil {
		return err
	}
	if !running {
		return nil
	}
	if err := server.SendSIGTERM(pid); err != nil {
		return fmt.Errorf("geoip: stop updater (pid %d): %w", pid, err)
	}
	return nil
}

// SyncNow signals the running geoip-updater process (via SIGUSR1, the
// same PID-file-sourced PID Stop() already signals with SIGTERM) to
// pull fresh data immediately, bypassing the normal
// pull_interval_hours staleness check — the manual "Sync now" action
// from the admin UI. Returns ErrNotRunning if nothing is currently
// running (there's no process to signal); the handler maps that to
// 409, since a stale UI/race is the only way this gets hit — the
// button is disabled client-side whenever the updater isn't running.
func (m *Manager) SyncNow() error {
	running, pid, err := m.processStatus()
	if err != nil {
		return err
	}
	if !running {
		return ErrNotRunning
	}
	if err := syscall.Kill(pid, syscall.SIGUSR1); err != nil {
		return fmt.Errorf("geoip: sync now (pid %d): %w", pid, err)
	}
	return nil
}

// Status reports whether the updater is running, its PID, and (best
// effort) when it last successfully pulled fresh data and how many
// ranges the live geoip.db currently holds.
func (m *Manager) Status() (Status, error) {
	running, pid, err := m.processStatus()
	if err != nil {
		return Status{}, err
	}
	lastPulledAt, countryCount, asnCount := m.dbStats()
	return Status{
		Running:           running,
		PID:               pid,
		LastPulledAt:      lastPulledAt,
		CountryRangeCount: countryCount,
		ASNRangeCount:     asnCount,
	}, nil
}

// processStatus checks only PID-file liveness — split out from
// Status so Start/Stop's internal guard doesn't also pay for the
// geoip.db read Status does for LastPulledAt. Self-heals a stale PID
// file (process no longer alive, e.g. it crashed) by removing it, so
// a later Start() isn't incorrectly blocked by leftover state from a
// dead process.
func (m *Manager) processStatus() (running bool, pid int, err error) {
	pid, err = server.ReadPID(m.pidFile)
	if err != nil {
		return false, 0, nil // no PID file = not running, not an error
	}
	if err := syscall.Kill(pid, 0); err != nil { // signal 0: liveness check only, never actually signals
		_ = server.RemovePID(m.pidFile)
		return false, 0, nil
	}
	return true, pid, nil
}

// dbStats is a best-effort read of geoip_meta.last_pulled_at and the
// current geoip_country_ranges/geoip_asn_ranges row counts from the
// live geoip.db — zero values across the board if the file doesn't
// exist yet, can't be opened, or the tables aren't there. Folded into
// a single sql.Open rather than three separate helpers: Status() is
// polled every 5s by the admin UI, no reason to open geoip.db more
// than once per call. Duplicates the equivalent last-pulled-at logic
// in updater.go rather than importing it: this package (geoip) cannot
// import its own subpackage (geoip/updater), which already imports
// geoip — that direction would be a cycle.
func (m *Manager) dbStats() (lastPulledAt time.Time, countryCount, asnCount int) {
	if _, err := os.Stat(m.dbPath); err != nil {
		return time.Time{}, 0, 0
	}
	db, err := sql.Open("sqlite3", m.dbPath)
	if err != nil {
		return time.Time{}, 0, 0
	}
	defer db.Close()

	var raw string
	if err := db.QueryRow(`SELECT value FROM geoip_meta WHERE key = 'last_pulled_at'`).Scan(&raw); err == nil {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			lastPulledAt = t
		}
	}
	_ = db.QueryRow(`SELECT COUNT(*) FROM geoip_country_ranges`).Scan(&countryCount)
	_ = db.QueryRow(`SELECT COUNT(*) FROM geoip_asn_ranges`).Scan(&asnCount)
	return lastPulledAt, countryCount, asnCount
}
