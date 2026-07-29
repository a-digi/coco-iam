package geoip

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/a-digi/coco-logger/logger"
	"github.com/a-digi/coco-server/server"
)

// Status is Manager.Status's result — the admin UI's view of the
// geoip-updater process.
type Status struct {
	Running      bool
	PID          int
	LastPulledAt time.Time // zero if unknown (never pulled, or geoip.db doesn't exist yet)
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

// Status reports whether the updater is running, its PID, and (best
// effort) when it last successfully pulled fresh data.
func (m *Manager) Status() (Status, error) {
	running, pid, err := m.processStatus()
	if err != nil {
		return Status{}, err
	}
	return Status{Running: running, PID: pid, LastPulledAt: m.lastPulledAt()}, nil
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

// lastPulledAt is a best-effort read of geoip_meta.last_pulled_at
// from the live geoip.db — nil (zero time.Time) if the file doesn't
// exist yet, can't be opened, or has no such row. Duplicates the
// equivalent helper in updater.go rather than importing it: this
// package (geoip) cannot import its own subpackage (geoip/updater),
// which already imports geoip — that direction would be a cycle.
func (m *Manager) lastPulledAt() time.Time {
	if _, err := os.Stat(m.dbPath); err != nil {
		return time.Time{}
	}
	db, err := sql.Open("sqlite3", m.dbPath)
	if err != nil {
		return time.Time{}
	}
	defer db.Close()

	var raw string
	if err := db.QueryRow(`SELECT value FROM geoip_meta WHERE key = 'last_pulled_at'`).Scan(&raw); err != nil {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}
	}
	return t
}
