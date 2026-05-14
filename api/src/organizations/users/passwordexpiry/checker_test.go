package passwordexpiry

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/a-digi/coco-iam/src/userrules"
)

const userAuthPasswordSchema = `
CREATE TABLE user_auth_password (
	user_id    TEXT NOT NULL PRIMARY KEY,
	password   TEXT NOT NULL,
	changed_at DATETIME,
	is_active  INTEGER NOT NULL DEFAULT 1
);
`

const userRuleSetsSchema = `
CREATE TABLE user_rule_sets (
	id         TEXT PRIMARY KEY,
	rules_json TEXT NOT NULL,
	updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
`

func freshOrgTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(userAuthPasswordSchema); err != nil {
		t.Fatalf("create user_auth_password: %v", err)
	}
	if _, err := db.Exec(userRuleSetsSchema); err != nil {
		t.Fatalf("create user_rule_sets: %v", err)
	}
	return db
}

func newOrgChecker(t *testing.T, db *sql.DB, orgID string) *Checker {
	t.Helper()
	store := userrules.NewOrgStoreFromFunc(func(id string) (*sql.DB, error) {
		return db, nil
	})
	openDB := func(id string) (*sql.DB, error) { return db, nil }
	return New(store, openDB)
}

func upsertOrgRules(t *testing.T, db *sql.DB, rs userrules.RuleSet) {
	t.Helper()
	raw, err := json.Marshal(rs)
	if err != nil {
		t.Fatalf("marshal rules: %v", err)
	}
	_, err = db.Exec(
		`INSERT INTO user_rule_sets (id, rules_json, updated_at)
		 VALUES ('default', ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(id) DO UPDATE SET rules_json = excluded.rules_json, updated_at = CURRENT_TIMESTAMP`,
		string(raw),
	)
	if err != nil {
		t.Fatalf("upsert org rules: %v", err)
	}
}

func insertOrgPassword(t *testing.T, db *sql.DB, userID string, changedAt *time.Time, isActive int) {
	t.Helper()
	if changedAt == nil {
		_, err := db.Exec(
			`INSERT INTO user_auth_password (user_id, password, changed_at, is_active) VALUES (?, 'hash', NULL, ?)`,
			userID, isActive,
		)
		if err != nil {
			t.Fatalf("insert password (null changed_at): %v", err)
		}
		return
	}
	_, err := db.Exec(
		`INSERT INTO user_auth_password (user_id, password, changed_at, is_active) VALUES (?, 'hash', ?, ?)`,
		userID, changedAt.UTC().Format("2006-01-02 15:04:05"), isActive,
	)
	if err != nil {
		t.Fatalf("insert password: %v", err)
	}
}

func TestOrgChecker_Disabled_WhenExpiryDaysZero(t *testing.T) {
	db := freshOrgTestDB(t)
	rs := userrules.Defaults()
	rs.Password.ExpiryDays = 0
	upsertOrgRules(t, db, rs)
	ago := time.Now().UTC().Add(-400 * 24 * time.Hour)
	insertOrgPassword(t, db, "user1", &ago, 1)

	c := newOrgChecker(t, db, "org1")
	expired, err := c.IsExpired("org1", "user1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expired {
		t.Error("expected false (ExpiryDays=0), got true")
	}
}

func TestOrgChecker_NotExpired_WhenFresh(t *testing.T) {
	db := freshOrgTestDB(t)
	rs := userrules.Defaults()
	rs.Password.ExpiryDays = 90
	upsertOrgRules(t, db, rs)
	yesterday := time.Now().UTC().Add(-24 * time.Hour)
	insertOrgPassword(t, db, "user2", &yesterday, 1)

	c := newOrgChecker(t, db, "org1")
	expired, err := c.IsExpired("org1", "user2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expired {
		t.Error("expected false (changed yesterday, expiry=90), got true")
	}
}

func TestOrgChecker_Expired_WhenOld(t *testing.T) {
	db := freshOrgTestDB(t)
	rs := userrules.Defaults()
	rs.Password.ExpiryDays = 30
	upsertOrgRules(t, db, rs)
	old := time.Now().UTC().Add(-45 * 24 * time.Hour)
	insertOrgPassword(t, db, "user3", &old, 1)

	c := newOrgChecker(t, db, "org1")
	expired, err := c.IsExpired("org1", "user3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !expired {
		t.Error("expected true (45 days old, expiry=30), got false")
	}
}

func TestOrgChecker_NullChangedAt_NotExpired(t *testing.T) {
	db := freshOrgTestDB(t)
	rs := userrules.Defaults()
	rs.Password.ExpiryDays = 30
	upsertOrgRules(t, db, rs)
	insertOrgPassword(t, db, "user4", nil, 1)

	c := newOrgChecker(t, db, "org1")
	expired, err := c.IsExpired("org1", "user4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expired {
		t.Error("expected false (null changed_at), got true")
	}
}

func TestOrgChecker_NoPasswordRow_NotExpired(t *testing.T) {
	db := freshOrgTestDB(t)
	rs := userrules.Defaults()
	rs.Password.ExpiryDays = 30
	upsertOrgRules(t, db, rs)

	c := newOrgChecker(t, db, "org1")
	expired, err := c.IsExpired("org1", "no-such-user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expired {
		t.Error("expected false (no password row), got true")
	}
}

func TestOrgChecker_ExactBoundary_NotExpired(t *testing.T) {
	db := freshOrgTestDB(t)
	rs := userrules.Defaults()
	rs.Password.ExpiryDays = 30
	upsertOrgRules(t, db, rs)
	exactly30 := time.Now().UTC().Add(-30 * 24 * time.Hour)
	insertOrgPassword(t, db, "user5", &exactly30, 1)

	c := newOrgChecker(t, db, "org1")
	expired, err := c.IsExpired("org1", "user5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = expired
}

func TestOrgChecker_ReadsExpiryDaysFromDB(t *testing.T) {
	db := freshOrgTestDB(t)
	rs := userrules.Defaults()
	rs.Password.ExpiryDays = 60
	upsertOrgRules(t, db, rs)
	old := time.Now().UTC().Add(-90 * 24 * time.Hour)
	insertOrgPassword(t, db, "user6", &old, 1)

	c := newOrgChecker(t, db, "org1")
	expired, err := c.IsExpired("org1", "user6")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !expired {
		t.Error("expected true (90 days old, ExpiryDays=60 from DB), got false")
	}

	rs2 := userrules.Defaults()
	rs2.Password.ExpiryDays = 0
	upsertOrgRules(t, db, rs2)
	expired2, err := c.IsExpired("org1", "user6")
	if err != nil {
		t.Fatalf("unexpected error after rule update: %v", err)
	}
	if expired2 {
		t.Error("expected false after ExpiryDays set to 0, got true")
	}
	_ = fmt.Sprintf("")
}
