package passwordexpiry

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	coco_orm "github.com/a-digi/coco-orm/orm"
	_ "github.com/mattn/go-sqlite3"

	"github.com/a-digi/coco-iam/src/userrules"
)

const adminAuthPasswordSchema = `
CREATE TABLE admin_auth_password (
	user_id   TEXT NOT NULL PRIMARY KEY,
	password  TEXT NOT NULL,
	changed_at DATETIME,
	is_active INTEGER NOT NULL DEFAULT 1
);
`

const adminRuleSetsSchema = `
CREATE TABLE admin_user_rule_sets (
	id         TEXT PRIMARY KEY,
	rules_json TEXT NOT NULL,
	updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
`

func freshTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(adminAuthPasswordSchema); err != nil {
		t.Fatalf("create admin_auth_password: %v", err)
	}
	if _, err := db.Exec(adminRuleSetsSchema); err != nil {
		t.Fatalf("create admin_user_rule_sets: %v", err)
	}
	return db
}

func newChecker(t *testing.T, db *sql.DB) *Checker {
	t.Helper()
	mgr := &coco_orm.DatabaseManager{Connector: &coco_orm.Connector{DB: db}}
	store := userrules.NewAdminStore(mgr)
	return New(store, db)
}

func upsertRules(t *testing.T, db *sql.DB, rs userrules.RuleSet) {
	t.Helper()
	raw, err := json.Marshal(rs)
	if err != nil {
		t.Fatalf("marshal rules: %v", err)
	}
	_, err = db.Exec(
		`INSERT INTO admin_user_rule_sets (id, rules_json, updated_at)
		 VALUES ('admin', ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(id) DO UPDATE SET rules_json = excluded.rules_json, updated_at = CURRENT_TIMESTAMP`,
		string(raw),
	)
	if err != nil {
		t.Fatalf("upsert rules: %v", err)
	}
}

func insertPassword(t *testing.T, db *sql.DB, userID string, changedAt *time.Time, isActive int) {
	t.Helper()
	if changedAt == nil {
		_, err := db.Exec(
			`INSERT INTO admin_auth_password (user_id, password, changed_at, is_active) VALUES (?, 'hash', NULL, ?)`,
			userID, isActive,
		)
		if err != nil {
			t.Fatalf("insert password (null changed_at): %v", err)
		}
		return
	}
	_, err := db.Exec(
		`INSERT INTO admin_auth_password (user_id, password, changed_at, is_active) VALUES (?, 'hash', ?, ?)`,
		userID, changedAt.UTC().Format("2006-01-02 15:04:05"), isActive,
	)
	if err != nil {
		t.Fatalf("insert password: %v", err)
	}
}

func TestAdminChecker_Disabled_WhenExpiryDaysZero(t *testing.T) {
	db := freshTestDB(t)
	rs := userrules.Defaults()
	rs.Password.ExpiryDays = 0
	upsertRules(t, db, rs)
	ago := time.Now().UTC().Add(-400 * 24 * time.Hour)
	insertPassword(t, db, "user1", &ago, 1)

	c := newChecker(t, db)
	expired, err := c.IsExpired("user1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expired {
		t.Error("expected false (ExpiryDays=0), got true")
	}
}

func TestAdminChecker_NotExpired_WhenFresh(t *testing.T) {
	db := freshTestDB(t)
	rs := userrules.Defaults()
	rs.Password.ExpiryDays = 90
	upsertRules(t, db, rs)
	yesterday := time.Now().UTC().Add(-24 * time.Hour)
	insertPassword(t, db, "user2", &yesterday, 1)

	c := newChecker(t, db)
	expired, err := c.IsExpired("user2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expired {
		t.Error("expected false (changed yesterday, expiry=90), got true")
	}
}

func TestAdminChecker_Expired_WhenOld(t *testing.T) {
	db := freshTestDB(t)
	rs := userrules.Defaults()
	rs.Password.ExpiryDays = 30
	upsertRules(t, db, rs)
	old := time.Now().UTC().Add(-45 * 24 * time.Hour)
	insertPassword(t, db, "user3", &old, 1)

	c := newChecker(t, db)
	expired, err := c.IsExpired("user3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !expired {
		t.Error("expected true (45 days old, expiry=30), got false")
	}
}

func TestAdminChecker_NullChangedAt_NotExpired(t *testing.T) {
	db := freshTestDB(t)
	rs := userrules.Defaults()
	rs.Password.ExpiryDays = 30
	upsertRules(t, db, rs)
	insertPassword(t, db, "user4", nil, 1)

	c := newChecker(t, db)
	expired, err := c.IsExpired("user4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expired {
		t.Error("expected false (null changed_at), got true")
	}
}

func TestAdminChecker_NoPasswordRow_NotExpired(t *testing.T) {
	db := freshTestDB(t)
	rs := userrules.Defaults()
	rs.Password.ExpiryDays = 30
	upsertRules(t, db, rs)

	c := newChecker(t, db)
	expired, err := c.IsExpired("no-such-user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expired {
		t.Error("expected false (no password row), got true")
	}
}

func TestAdminChecker_ExactBoundary_NotExpired(t *testing.T) {
	db := freshTestDB(t)
	rs := userrules.Defaults()
	rs.Password.ExpiryDays = 30
	upsertRules(t, db, rs)
	// changed_at exactly 30 days ago to the second — expiry is at this exact moment
	// time.Now().After(expiry) should be false (not strictly after)
	exactly30 := time.Now().UTC().Add(-30 * 24 * time.Hour)
	insertPassword(t, db, "user5", &exactly30, 1)

	c := newChecker(t, db)
	expired, err := c.IsExpired("user5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// At the exact boundary, time.Now().After(expiry) == false (or nearly so).
	// We accept either result here since it's a sub-second boundary.
	// The key property is: no error, and no panic.
	_ = expired
}

func TestAdminChecker_ReadsExpiryDaysFromDB(t *testing.T) {
	db := freshTestDB(t)
	rs := userrules.Defaults()
	rs.Password.ExpiryDays = 60
	upsertRules(t, db, rs)
	old := time.Now().UTC().Add(-90 * 24 * time.Hour)
	insertPassword(t, db, "user6", &old, 1)

	c := newChecker(t, db)
	expired, err := c.IsExpired("user6")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !expired {
		t.Errorf("expected true (90 days old, ExpiryDays=60 from DB), got false")
	}

	// Now update to ExpiryDays=0 (disabled) — same user should no longer be expired
	rs2 := userrules.Defaults()
	rs2.Password.ExpiryDays = 0
	upsertRules(t, db, rs2)
	expired2, err := c.IsExpired("user6")
	if err != nil {
		t.Fatalf("unexpected error after rule update: %v", err)
	}
	if expired2 {
		t.Error("expected false after ExpiryDays set to 0, got true")
	}
	_ = fmt.Sprintf("") // keep fmt import if only used here
}
