package geoip

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// buildFakeUpdaterScript writes a tiny shell script that writes its
// own PID to pidFile, then sleeps until it receives SIGTERM, at which
// point it removes the PID file and exits — mimicking exactly what
// the real geoipupdater will do (see cmd/geoipupdater/main.go), so
// Manager can be tested without a real MaxMind account or CSV data.
func buildFakeUpdaterScript(t *testing.T, pidFile string) string {
	t.Helper()
	scriptPath := filepath.Join(t.TempDir(), "fake-updater.sh")
	script := fmt.Sprintf("#!/bin/sh\necho $$ > %q\ntrap 'rm -f %q; exit 0' TERM\nwhile true; do sleep 1; done\n", pidFile, pidFile)
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("write fake updater script: %v", err)
	}
	return scriptPath
}

func waitForCondition(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("condition never became true within timeout")
}

func TestManager_Start_LaunchesProcessAndWritesPIDFile(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "updater.pid")
	dbPath := filepath.Join(dir, "geoip.db") // never created — LastPulledAt should be zero

	script := buildFakeUpdaterScript(t, pidFile)
	m := NewManager(script, pidFile, dbPath, nil)

	pid, err := m.Start()
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if pid <= 0 {
		t.Fatalf("Start() pid = %d, want > 0", pid)
	}

	waitForCondition(t, 2*time.Second, func() bool {
		_, err := os.Stat(pidFile)
		return err == nil
	})

	status, err := m.Status()
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if !status.Running {
		t.Fatal("Status().Running = false, want true")
	}
	if status.PID != pid {
		t.Errorf("Status().PID = %d, want %d", status.PID, pid)
	}
	if !status.LastPulledAt.IsZero() {
		t.Fatalf("Status().LastPulledAt = %v, want zero (geoip.db never created)", status.LastPulledAt)
	}

	if err := m.Stop(); err != nil {
		t.Fatalf("cleanup Stop() error = %v", err)
	}
	waitForCondition(t, 2*time.Second, func() bool {
		_, err := os.Stat(pidFile)
		return os.IsNotExist(err)
	})
}

func TestManager_Start_RefusesWhenAlreadyRunning(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "updater.pid")
	dbPath := filepath.Join(dir, "geoip.db")

	script := buildFakeUpdaterScript(t, pidFile)
	m := NewManager(script, pidFile, dbPath, nil)

	firstPID, err := m.Start()
	if err != nil {
		t.Fatalf("first Start() error = %v", err)
	}
	waitForCondition(t, 2*time.Second, func() bool {
		_, err := os.Stat(pidFile)
		return err == nil
	})
	defer func() { _ = m.Stop() }()

	secondPID, err := m.Start()
	if err == nil {
		t.Fatal("second Start() error = nil, want an error since an instance is already running")
	}
	if secondPID != firstPID {
		t.Errorf("second Start() returned pid %d, want the existing pid %d", secondPID, firstPID)
	}
}

func TestManager_Stop_ActuallyTerminatesTheProcess(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "updater.pid")
	dbPath := filepath.Join(dir, "geoip.db")

	script := buildFakeUpdaterScript(t, pidFile)
	m := NewManager(script, pidFile, dbPath, nil)

	if _, err := m.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	waitForCondition(t, 2*time.Second, func() bool {
		_, err := os.Stat(pidFile)
		return err == nil
	})

	if err := m.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	waitForCondition(t, 2*time.Second, func() bool {
		_, err := os.Stat(pidFile)
		return os.IsNotExist(err)
	})

	status, err := m.Status()
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status.Running {
		t.Fatal("Status().Running = true after Stop(), want false")
	}
}

func TestManager_Stop_IsNoopWhenNotRunning(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "updater.pid") // never created
	m := NewManager("/bin/true", pidFile, filepath.Join(dir, "geoip.db"), nil)

	if err := m.Stop(); err != nil {
		t.Fatalf("Stop() error = %v, want nil (nothing running is not an error)", err)
	}
}

func TestManager_Status_NotRunningWhenNoPIDFile(t *testing.T) {
	dir := t.TempDir()
	m := NewManager("/bin/true", filepath.Join(dir, "updater.pid"), filepath.Join(dir, "geoip.db"), nil)

	status, err := m.Status()
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status.Running {
		t.Fatal("Status().Running = true, want false when no PID file exists")
	}
}

func TestManager_Status_SelfHealsStalePIDFile(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "updater.pid")

	// A PID essentially guaranteed not to correspond to a live
	// process in the test environment.
	if err := os.WriteFile(pidFile, []byte("999999"), 0644); err != nil {
		t.Fatalf("write stale pid file: %v", err)
	}

	m := NewManager("/bin/true", pidFile, filepath.Join(dir, "geoip.db"), nil)
	status, err := m.Status()
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status.Running {
		t.Fatal("Status().Running = true, want false for a stale PID pointing at a dead process")
	}
	if _, err := os.Stat(pidFile); !os.IsNotExist(err) {
		t.Fatal("stale PID file should have been removed by Status()")
	}
}

func TestManager_Status_ReadsLastPulledAtFromGeoIPDB(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "geoip.db")

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open geoip.db: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE geoip_meta (key TEXT NOT NULL PRIMARY KEY, value TEXT NOT NULL)`); err != nil {
		t.Fatalf("create geoip_meta: %v", err)
	}
	want := time.Now().UTC().Truncate(time.Second)
	if _, err := db.Exec(`INSERT INTO geoip_meta (key, value) VALUES ('last_pulled_at', ?)`, want.Format(time.RFC3339)); err != nil {
		t.Fatalf("seed last_pulled_at: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close geoip.db: %v", err)
	}

	m := NewManager("/bin/true", filepath.Join(dir, "updater.pid"), dbPath, nil)
	status, err := m.Status()
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if !status.LastPulledAt.Equal(want) {
		t.Fatalf("Status().LastPulledAt = %v, want %v", status.LastPulledAt, want)
	}
}
