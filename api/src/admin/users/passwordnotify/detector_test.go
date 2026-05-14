package adminpwnotify

import (
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	coco_orm "github.com/a-digi/coco-orm/orm"
	_ "github.com/mattn/go-sqlite3"

	"github.com/a-digi/coco-iam/src/userrules"
)

// --- schema ---------------------------------------------------------------

const schemaAdminUsers = `
CREATE TABLE admin_users (
	id         TEXT NOT NULL PRIMARY KEY,
	email      TEXT NOT NULL,
	username   TEXT NOT NULL,
	is_active  INTEGER NOT NULL DEFAULT 1
);
`

const schemaAdminAuthPassword = `
CREATE TABLE admin_auth_password (
	user_id    TEXT NOT NULL PRIMARY KEY,
	password   TEXT NOT NULL DEFAULT 'hash',
	changed_at DATETIME,
	is_active  INTEGER NOT NULL DEFAULT 1
);
`

const schemaAdminNotifyPrefs = `
CREATE TABLE admin_password_notify_prefs (
	user_id     TEXT NOT NULL PRIMARY KEY,
	notify_days TEXT NOT NULL DEFAULT '[]',
	updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);
`

const schemaAdminNotifyLog = `
CREATE TABLE admin_password_notify_log (
	id                   TEXT NOT NULL PRIMARY KEY,
	user_id              TEXT NOT NULL,
	password_changed_at  TEXT NOT NULL,
	days_before          INTEGER NOT NULL,
	sent_at              DATETIME DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(user_id, password_changed_at, days_before)
);
`

const schemaAdminRuleSets = `
CREATE TABLE admin_user_rule_sets (
	id         TEXT PRIMARY KEY,
	rules_json TEXT NOT NULL,
	updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
`

// --- helpers ---------------------------------------------------------------

func freshDetectorDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, ddl := range []string{
		schemaAdminUsers,
		schemaAdminAuthPassword,
		schemaAdminNotifyPrefs,
		schemaAdminNotifyLog,
		schemaAdminRuleSets,
	} {
		if _, err := db.Exec(ddl); err != nil {
			t.Fatalf("create schema: %v", err)
		}
	}
	return db
}

func setExpiryDays(t *testing.T, db *sql.DB, days int) {
	t.Helper()
	rs := userrules.Defaults()
	rs.Password.ExpiryDays = days
	raw, _ := json.Marshal(rs)
	_, err := db.Exec(
		`INSERT INTO admin_user_rule_sets (id, rules_json)
		 VALUES ('admin', ?)
		 ON CONFLICT(id) DO UPDATE SET rules_json = excluded.rules_json`,
		string(raw),
	)
	if err != nil {
		t.Fatalf("setExpiryDays: %v", err)
	}
}

func insertUser(t *testing.T, db *sql.DB, id, email, username string) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO admin_users (id, email, username, is_active) VALUES (?, ?, ?, 1)`,
		id, email, username,
	)
	if err != nil {
		t.Fatalf("insertUser: %v", err)
	}
}

func insertChangedAt(t *testing.T, db *sql.DB, userID string, changedAt time.Time) string {
	t.Helper()
	s := changedAt.UTC().Format("2006-01-02 15:04:05")
	_, err := db.Exec(
		`INSERT INTO admin_auth_password (user_id, changed_at, is_active)
		 VALUES (?, ?, 1)
		 ON CONFLICT(user_id) DO UPDATE SET changed_at = excluded.changed_at`,
		userID, s,
	)
	if err != nil {
		t.Fatalf("insertChangedAt: %v", err)
	}
	// Read back the value as Go/SQLite will scan it, so the test fingerprint
	// matches what the detector stores in the notify log.
	var scanned string
	if err := db.QueryRow(`SELECT changed_at FROM admin_auth_password WHERE user_id = ?`, userID).Scan(&scanned); err != nil {
		t.Fatalf("re-read changed_at: %v", err)
	}
	return scanned
}

func setNotifyDays(t *testing.T, db *sql.DB, userID string, days []int) {
	t.Helper()
	raw, _ := json.Marshal(days)
	_, err := db.Exec(
		`INSERT INTO admin_password_notify_prefs (user_id, notify_days)
		 VALUES (?, ?)
		 ON CONFLICT(user_id) DO UPDATE SET notify_days = excluded.notify_days`,
		userID, string(raw),
	)
	if err != nil {
		t.Fatalf("setNotifyDays: %v", err)
	}
}

func insertNotifyLog(t *testing.T, db *sql.DB, userID, changedAt string, daysBefore int) {
	t.Helper()
	_, err := db.Exec(
		`INSERT OR IGNORE INTO admin_password_notify_log (id, user_id, password_changed_at, days_before)
		 VALUES (?, ?, ?, ?)`,
		newID(), userID, changedAt, daysBefore,
	)
	if err != nil {
		t.Fatalf("insertNotifyLog: %v", err)
	}
}

// publishRecorder counts Publish calls.
type publishRecorder struct {
	calls []string
}

func (p *publishRecorder) Publish(queueName string, _ interface{}) error {
	p.calls = append(p.calls, queueName)
	return nil
}

func newTestDetector(db *sql.DB, pub publisher) *AdminDetector {
	mgr := &coco_orm.DatabaseManager{Connector: &coco_orm.Connector{DB: db}}
	store := userrules.NewAdminStore(mgr)
	d := NewAdminDetector(db, store, pub, noopLogger{})
	return d
}

// noopLogger discards all log output in tests.
type noopLogger struct{}

func (noopLogger) Info(format string, args ...interface{})    {}
func (noopLogger) Warning(format string, args ...interface{}) {}
func (noopLogger) Error(format string, args ...interface{})   {}
func (noopLogger) Close()                                     {}

// --- tests -----------------------------------------------------------------

func TestAdminDetector_TTLDisabled_NothingPublished(t *testing.T) {
	db := freshDetectorDB(t)
	setExpiryDays(t, db, 0)
	insertUser(t, db, "u1", "u1@example.com", "user1")
	insertChangedAt(t, db, "u1", time.Now().UTC().Add(-25*24*time.Hour))
	setNotifyDays(t, db, "u1", []int{7})

	rec := &publishRecorder{}
	d := newTestDetector(db, rec)
	d.scan(t.Context())

	if len(rec.calls) != 0 {
		t.Errorf("expected 0 publishes, got %d", len(rec.calls))
	}
}

func TestAdminDetector_NoPrefs_NothingPublished(t *testing.T) {
	db := freshDetectorDB(t)
	setExpiryDays(t, db, 30)
	insertUser(t, db, "u2", "u2@example.com", "user2")
	insertChangedAt(t, db, "u2", time.Now().UTC().Add(-25*24*time.Hour))
	// No notify_days prefs inserted

	rec := &publishRecorder{}
	d := newTestDetector(db, rec)
	d.scan(t.Context())

	if len(rec.calls) != 0 {
		t.Errorf("expected 0 publishes (no prefs), got %d", len(rec.calls))
	}
}

func TestAdminDetector_WindowNotReached_NothingPublished(t *testing.T) {
	// ExpiryDays=90, changed 10 days ago, notify_days=[7]
	// expiry = now + 80 days; notify window starts at expiry-7 = now + 73 days
	// now is before that window
	db := freshDetectorDB(t)
	setExpiryDays(t, db, 90)
	insertUser(t, db, "u3", "u3@example.com", "user3")
	insertChangedAt(t, db, "u3", time.Now().UTC().Add(-10*24*time.Hour))
	setNotifyDays(t, db, "u3", []int{7})

	rec := &publishRecorder{}
	d := newTestDetector(db, rec)
	d.scan(t.Context())

	if len(rec.calls) != 0 {
		t.Errorf("expected 0 publishes (window not reached), got %d", len(rec.calls))
	}
}

func TestAdminDetector_WindowReached_Published(t *testing.T) {
	// ExpiryDays=30, changed 24 days ago, notify_days=[7]
	// expiry = now + 6 days; 7-day window already passed (expiry-7 = now-1 day)
	db := freshDetectorDB(t)
	setExpiryDays(t, db, 30)
	insertUser(t, db, "u4", "u4@example.com", "user4")
	insertChangedAt(t, db, "u4", time.Now().UTC().Add(-24*24*time.Hour))
	setNotifyDays(t, db, "u4", []int{7})

	rec := &publishRecorder{}
	d := newTestDetector(db, rec)
	d.scan(t.Context())

	if len(rec.calls) != 1 {
		t.Errorf("expected 1 publish, got %d", len(rec.calls))
	}
}

func TestAdminDetector_AlreadySent_NotPublishedAgain(t *testing.T) {
	db := freshDetectorDB(t)
	setExpiryDays(t, db, 30)
	insertUser(t, db, "u5", "u5@example.com", "user5")
	changedAt := insertChangedAt(t, db, "u5", time.Now().UTC().Add(-24*24*time.Hour))
	setNotifyDays(t, db, "u5", []int{7})
	// Pre-seed the log to simulate already-sent
	insertNotifyLog(t, db, "u5", changedAt, 7)

	rec := &publishRecorder{}
	d := newTestDetector(db, rec)
	d.scan(t.Context())

	if len(rec.calls) != 0 {
		t.Errorf("expected 0 publishes (already sent), got %d", len(rec.calls))
	}
}

func TestAdminDetector_PasswordChanged_ResetsLog(t *testing.T) {
	// Different changedAt string = different fingerprint = published again
	db := freshDetectorDB(t)
	setExpiryDays(t, db, 30)
	insertUser(t, db, "u6", "u6@example.com", "user6")
	oldChangedAt := "2020-01-01 00:00:00"
	insertNotifyLog(t, db, "u6", oldChangedAt, 7)

	// Now set a NEW changed_at (recently changed, within expiry window)
	insertChangedAt(t, db, "u6", time.Now().UTC().Add(-24*24*time.Hour))
	setNotifyDays(t, db, "u6", []int{7})

	rec := &publishRecorder{}
	d := newTestDetector(db, rec)
	d.scan(t.Context())

	if len(rec.calls) != 1 {
		t.Errorf("expected 1 publish (new password, fresh fingerprint), got %d", len(rec.calls))
	}
}

func TestAdminDetector_MultipleNotifyDays_EachFires(t *testing.T) {
	// prefs=[1,7], ExpiryDays=30, changed 29.5 days ago
	// expiry = now + 0.5 days
	// 7-day window: expiry-7 = now-6.5 days -> passed
	// 1-day window: expiry-1 = now-0.5 days -> passed
	db := freshDetectorDB(t)
	setExpiryDays(t, db, 30)
	insertUser(t, db, "u7", "u7@example.com", "user7")
	insertChangedAt(t, db, "u7", time.Now().UTC().Add(-29*24*time.Hour-12*time.Hour))
	setNotifyDays(t, db, "u7", []int{1, 7})

	rec := &publishRecorder{}
	d := newTestDetector(db, rec)
	d.scan(t.Context())

	if len(rec.calls) != 2 {
		t.Errorf("expected 2 publishes (days 1 and 7), got %d", len(rec.calls))
	}
}
