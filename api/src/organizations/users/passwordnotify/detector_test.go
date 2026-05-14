package orgpwnotify

import (
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/a-digi/coco-iam/src/userrules"
)

// --- schema ---------------------------------------------------------------

const schemaUsers = `
CREATE TABLE users (
	id        TEXT NOT NULL PRIMARY KEY,
	email     TEXT NOT NULL,
	username  TEXT NOT NULL,
	is_active INTEGER NOT NULL DEFAULT 1
);
`

const schemaUserAuthPassword = `
CREATE TABLE user_auth_password (
	user_id    TEXT NOT NULL PRIMARY KEY,
	password   TEXT NOT NULL DEFAULT 'hash',
	changed_at DATETIME,
	is_active  INTEGER NOT NULL DEFAULT 1
);
`

const schemaUserNotifyPrefs = `
CREATE TABLE user_password_notify_prefs (
	user_id     TEXT NOT NULL PRIMARY KEY,
	notify_days TEXT NOT NULL DEFAULT '[]',
	updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);
`

const schemaUserNotifyLog = `
CREATE TABLE user_password_notify_log (
	id                   TEXT NOT NULL PRIMARY KEY,
	user_id              TEXT NOT NULL,
	password_changed_at  TEXT NOT NULL,
	days_before          INTEGER NOT NULL,
	sent_at              DATETIME DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(user_id, password_changed_at, days_before)
);
`

const schemaOrgRuleSets = `
CREATE TABLE user_rule_sets (
	id         TEXT PRIMARY KEY,
	rules_json TEXT NOT NULL,
	updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
`

// --- helpers ---------------------------------------------------------------

func freshOrgDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, ddl := range []string{
		schemaUsers,
		schemaUserAuthPassword,
		schemaUserNotifyPrefs,
		schemaUserNotifyLog,
	} {
		if _, err := db.Exec(ddl); err != nil {
			t.Fatalf("create schema: %v", err)
		}
	}
	return db
}

// orgStoreFromFunc builds an OrgStore that returns rules from a static map.
func orgStoreFromFunc(rules map[string]userrules.RuleSet) *userrules.OrgStore {
	return userrules.NewOrgStoreFromFunc(func(orgID string) (*sql.DB, error) {
		// Return a tiny in-memory DB that holds just the rule set for this org.
		db, err := sql.Open("sqlite3", ":memory:")
		if err != nil {
			return nil, err
		}
		if _, err := db.Exec(schemaOrgRuleSets); err != nil {
			return nil, err
		}
		rs, ok := rules[orgID]
		if !ok {
			rs = userrules.Defaults()
		}
		raw, _ := json.Marshal(rs)
		_, err = db.Exec(
			`INSERT INTO user_rule_sets (id, rules_json) VALUES ('default', ?)`,
			string(raw),
		)
		return db, err
	})
}

func insertOrgUser(t *testing.T, db *sql.DB, id, email, username string) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO users (id, email, username, is_active) VALUES (?, ?, ?, 1)`,
		id, email, username,
	)
	if err != nil {
		t.Fatalf("insertOrgUser: %v", err)
	}
}

func insertOrgChangedAt(t *testing.T, db *sql.DB, userID string, changedAt time.Time) string {
	t.Helper()
	s := changedAt.UTC().Format("2006-01-02 15:04:05")
	_, err := db.Exec(
		`INSERT INTO user_auth_password (user_id, changed_at, is_active)
		 VALUES (?, ?, 1)
		 ON CONFLICT(user_id) DO UPDATE SET changed_at = excluded.changed_at`,
		userID, s,
	)
	if err != nil {
		t.Fatalf("insertOrgChangedAt: %v", err)
	}
	// Read back the value as Go/SQLite will scan it, so the test fingerprint
	// matches what the detector stores in the notify log.
	var scanned string
	if err := db.QueryRow(`SELECT changed_at FROM user_auth_password WHERE user_id = ?`, userID).Scan(&scanned); err != nil {
		t.Fatalf("re-read changed_at: %v", err)
	}
	return scanned
}

func setOrgNotifyDays(t *testing.T, db *sql.DB, userID string, days []int) {
	t.Helper()
	raw, _ := json.Marshal(days)
	_, err := db.Exec(
		`INSERT INTO user_password_notify_prefs (user_id, notify_days)
		 VALUES (?, ?)
		 ON CONFLICT(user_id) DO UPDATE SET notify_days = excluded.notify_days`,
		userID, string(raw),
	)
	if err != nil {
		t.Fatalf("setOrgNotifyDays: %v", err)
	}
}

func insertOrgNotifyLog(t *testing.T, db *sql.DB, userID, changedAt string, daysBefore int) {
	t.Helper()
	_, err := db.Exec(
		`INSERT OR IGNORE INTO user_password_notify_log (id, user_id, password_changed_at, days_before)
		 VALUES (?, ?, ?, ?)`,
		newID(), userID, changedAt, daysBefore,
	)
	if err != nil {
		t.Fatalf("insertOrgNotifyLog: %v", err)
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

// noopLogger discards all log output in tests.
type noopLogger struct{}

func (noopLogger) Info(format string, args ...interface{})    {}
func (noopLogger) Warning(format string, args ...interface{}) {}
func (noopLogger) Error(format string, args ...interface{})   {}
func (noopLogger) Close()                                     {}

func makeExpiryRules(days int) userrules.RuleSet {
	rs := userrules.Defaults()
	rs.Password.ExpiryDays = days
	return rs
}

// --- tests -----------------------------------------------------------------

func TestOrgDetector_TTLDisabled_NothingPublished(t *testing.T) {
	orgDB := freshOrgDB(t)
	insertOrgUser(t, orgDB, "u1", "u1@example.com", "user1")
	insertOrgChangedAt(t, orgDB, "u1", time.Now().UTC().Add(-25*24*time.Hour))
	setOrgNotifyDays(t, orgDB, "u1", []int{7})

	store := orgStoreFromFunc(map[string]userrules.RuleSet{
		"org1": makeExpiryRules(0), // disabled
	})
	rec := &publishRecorder{}
	d := &OrgDetector{orgStore: store, queueMgr: rec, log: noopLogger{}}
	d.scanOrg(t.Context(), "org1", orgDB)

	if len(rec.calls) != 0 {
		t.Errorf("expected 0 publishes (TTL disabled), got %d", len(rec.calls))
	}
}

func TestOrgDetector_NoPrefs_NothingPublished(t *testing.T) {
	orgDB := freshOrgDB(t)
	insertOrgUser(t, orgDB, "u2", "u2@example.com", "user2")
	insertOrgChangedAt(t, orgDB, "u2", time.Now().UTC().Add(-25*24*time.Hour))
	// No prefs

	store := orgStoreFromFunc(map[string]userrules.RuleSet{
		"org1": makeExpiryRules(30),
	})
	rec := &publishRecorder{}
	d := &OrgDetector{orgStore: store, queueMgr: rec, log: noopLogger{}}
	d.scanOrg(t.Context(), "org1", orgDB)

	if len(rec.calls) != 0 {
		t.Errorf("expected 0 publishes (no prefs), got %d", len(rec.calls))
	}
}

func TestOrgDetector_WindowNotReached_NothingPublished(t *testing.T) {
	orgDB := freshOrgDB(t)
	insertOrgUser(t, orgDB, "u3", "u3@example.com", "user3")
	insertOrgChangedAt(t, orgDB, "u3", time.Now().UTC().Add(-10*24*time.Hour))
	setOrgNotifyDays(t, orgDB, "u3", []int{7})

	store := orgStoreFromFunc(map[string]userrules.RuleSet{
		"org1": makeExpiryRules(90),
	})
	rec := &publishRecorder{}
	d := &OrgDetector{orgStore: store, queueMgr: rec, log: noopLogger{}}
	d.scanOrg(t.Context(), "org1", orgDB)

	if len(rec.calls) != 0 {
		t.Errorf("expected 0 publishes (window not reached), got %d", len(rec.calls))
	}
}

func TestOrgDetector_WindowReached_Published(t *testing.T) {
	orgDB := freshOrgDB(t)
	insertOrgUser(t, orgDB, "u4", "u4@example.com", "user4")
	insertOrgChangedAt(t, orgDB, "u4", time.Now().UTC().Add(-24*24*time.Hour))
	setOrgNotifyDays(t, orgDB, "u4", []int{7})

	store := orgStoreFromFunc(map[string]userrules.RuleSet{
		"org1": makeExpiryRules(30),
	})
	rec := &publishRecorder{}
	d := &OrgDetector{orgStore: store, queueMgr: rec, log: noopLogger{}}
	d.scanOrg(t.Context(), "org1", orgDB)

	if len(rec.calls) != 1 {
		t.Errorf("expected 1 publish, got %d", len(rec.calls))
	}
}

func TestOrgDetector_AlreadySent_NotPublishedAgain(t *testing.T) {
	orgDB := freshOrgDB(t)
	insertOrgUser(t, orgDB, "u5", "u5@example.com", "user5")
	changedAt := insertOrgChangedAt(t, orgDB, "u5", time.Now().UTC().Add(-24*24*time.Hour))
	setOrgNotifyDays(t, orgDB, "u5", []int{7})
	insertOrgNotifyLog(t, orgDB, "u5", changedAt, 7)

	store := orgStoreFromFunc(map[string]userrules.RuleSet{
		"org1": makeExpiryRules(30),
	})
	rec := &publishRecorder{}
	d := &OrgDetector{orgStore: store, queueMgr: rec, log: noopLogger{}}
	d.scanOrg(t.Context(), "org1", orgDB)

	if len(rec.calls) != 0 {
		t.Errorf("expected 0 publishes (already sent), got %d", len(rec.calls))
	}
}

func TestOrgDetector_PasswordChanged_ResetsLog(t *testing.T) {
	orgDB := freshOrgDB(t)
	insertOrgUser(t, orgDB, "u6", "u6@example.com", "user6")
	oldChangedAt := "2020-01-01 00:00:00"
	insertOrgNotifyLog(t, orgDB, "u6", oldChangedAt, 7)

	// New changed_at (recent, within expiry window)
	insertOrgChangedAt(t, orgDB, "u6", time.Now().UTC().Add(-24*24*time.Hour))
	setOrgNotifyDays(t, orgDB, "u6", []int{7})

	store := orgStoreFromFunc(map[string]userrules.RuleSet{
		"org1": makeExpiryRules(30),
	})
	rec := &publishRecorder{}
	d := &OrgDetector{orgStore: store, queueMgr: rec, log: noopLogger{}}
	d.scanOrg(t.Context(), "org1", orgDB)

	if len(rec.calls) != 1 {
		t.Errorf("expected 1 publish (new password fingerprint), got %d", len(rec.calls))
	}
}

func TestOrgDetector_MultipleNotifyDays_EachFires(t *testing.T) {
	orgDB := freshOrgDB(t)
	insertOrgUser(t, orgDB, "u7", "u7@example.com", "user7")
	insertOrgChangedAt(t, orgDB, "u7", time.Now().UTC().Add(-29*24*time.Hour-12*time.Hour))
	setOrgNotifyDays(t, orgDB, "u7", []int{1, 7})

	store := orgStoreFromFunc(map[string]userrules.RuleSet{
		"org1": makeExpiryRules(30),
	})
	rec := &publishRecorder{}
	d := &OrgDetector{orgStore: store, queueMgr: rec, log: noopLogger{}}
	d.scanOrg(t.Context(), "org1", orgDB)

	if len(rec.calls) != 2 {
		t.Errorf("expected 2 publishes (days 1 and 7), got %d", len(rec.calls))
	}
}

func TestOrgDetector_TwoOrgs_Isolated(t *testing.T) {
	// Two org DBs with different users; each has its own expiry config.
	// Org1 has user with window reached; Org2 TTL disabled.
	orgDB1 := freshOrgDB(t)
	insertOrgUser(t, orgDB1, "u8", "u8@example.com", "user8")
	insertOrgChangedAt(t, orgDB1, "u8", time.Now().UTC().Add(-24*24*time.Hour))
	setOrgNotifyDays(t, orgDB1, "u8", []int{7})

	orgDB2 := freshOrgDB(t)
	insertOrgUser(t, orgDB2, "u9", "u9@example.com", "user9")
	insertOrgChangedAt(t, orgDB2, "u9", time.Now().UTC().Add(-24*24*time.Hour))
	setOrgNotifyDays(t, orgDB2, "u9", []int{7})

	store := orgStoreFromFunc(map[string]userrules.RuleSet{
		"org1": makeExpiryRules(30),
		"org2": makeExpiryRules(0), // disabled
	})
	rec := &publishRecorder{}
	d := &OrgDetector{orgStore: store, queueMgr: rec, log: noopLogger{}}

	d.scanOrg(t.Context(), "org1", orgDB1)
	d.scanOrg(t.Context(), "org2", orgDB2)

	if len(rec.calls) != 1 {
		t.Errorf("expected 1 publish (only org1), got %d", len(rec.calls))
	}
}
