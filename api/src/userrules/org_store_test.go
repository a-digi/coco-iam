package userrules

import (
	"database/sql"
	"fmt"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

const orgRuleSetsSchema = `
	CREATE TABLE user_rule_sets (
		id TEXT PRIMARY KEY,
		rules_json TEXT NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
`

func freshOrgRuleDB(t *testing.T) *sql.DB {
	t.Helper()
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if _, err := sqlDB.Exec(orgRuleSetsSchema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return sqlDB
}

func freshOrgRuleStoreWithDBs(dbs map[string]*sql.DB) *OrgStore {
	return &OrgStore{
		openDB: func(orgID string) (*sql.DB, error) {
			db, ok := dbs[orgID]
			if !ok {
				return nil, fmt.Errorf("org %s not found", orgID)
			}
			return db, nil
		},
	}
}

func TestOrgStore_GetReturnsDefaults_WhenNoRow(t *testing.T) {
	db := freshOrgRuleDB(t)
	s := freshOrgRuleStoreWithDBs(map[string]*sql.DB{"org-1": db})

	rs, err := s.GetForOrg("org-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	def := Defaults()
	if rs.Password.MinLength != def.Password.MinLength {
		t.Errorf("MinLength: got %d, want %d", rs.Password.MinLength, def.Password.MinLength)
	}
}

func TestOrgStore_UpsertAndGet_RoundTrips(t *testing.T) {
	db := freshOrgRuleDB(t)
	s := freshOrgRuleStoreWithDBs(map[string]*sql.DB{"org-1": db})

	want := Defaults()
	want.Password.MinLength = 14
	want.Username.MinLength = 6

	if err := s.UpsertForOrg("org-1", want); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got, err := s.GetForOrg("org-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Password.MinLength != want.Password.MinLength {
		t.Errorf("Password.MinLength: got %d, want %d", got.Password.MinLength, want.Password.MinLength)
	}
	if got.Username.MinLength != want.Username.MinLength {
		t.Errorf("Username.MinLength: got %d, want %d", got.Username.MinLength, want.Username.MinLength)
	}
}

func TestOrgStore_Upsert_IsIdempotent(t *testing.T) {
	db := freshOrgRuleDB(t)
	s := freshOrgRuleStoreWithDBs(map[string]*sql.DB{"org-1": db})

	first := Defaults()
	first.Password.MinLength = 10
	if err := s.UpsertForOrg("org-1", first); err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	second := Defaults()
	second.Password.MinLength = 18
	if err := s.UpsertForOrg("org-1", second); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	got, err := s.GetForOrg("org-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Password.MinLength != 18 {
		t.Errorf("MinLength after second upsert: got %d, want 18", got.Password.MinLength)
	}

	// Confirm exactly one row.
	var count int
	_ = db.QueryRow(`SELECT COUNT(*) FROM user_rule_sets`).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 row, got %d", count)
	}
}

func TestOrgStore_TwoOrgsAreIsolated(t *testing.T) {
	dbA := freshOrgRuleDB(t)
	dbB := freshOrgRuleDB(t)
	s := freshOrgRuleStoreWithDBs(map[string]*sql.DB{"org-a": dbA, "org-b": dbB})

	rsA := Defaults()
	rsA.Password.MinLength = 8
	rsB := Defaults()
	rsB.Password.MinLength = 20

	if err := s.UpsertForOrg("org-a", rsA); err != nil {
		t.Fatalf("upsert org-a: %v", err)
	}
	if err := s.UpsertForOrg("org-b", rsB); err != nil {
		t.Fatalf("upsert org-b: %v", err)
	}

	gotA, err := s.GetForOrg("org-a")
	if err != nil {
		t.Fatalf("get org-a: %v", err)
	}
	gotB, err := s.GetForOrg("org-b")
	if err != nil {
		t.Fatalf("get org-b: %v", err)
	}
	if gotA.Password.MinLength != 8 {
		t.Errorf("org-a MinLength: got %d, want 8", gotA.Password.MinLength)
	}
	if gotB.Password.MinLength != 20 {
		t.Errorf("org-b MinLength: got %d, want 20", gotB.Password.MinLength)
	}
}

func TestOrgStore_GetReturnsDefaults_ForUnknownOrg(t *testing.T) {
	s := freshOrgRuleStoreWithDBs(map[string]*sql.DB{})
	rs, err := s.GetForOrg("nonexistent-org")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	def := Defaults()
	if rs.Password.MinLength != def.Password.MinLength {
		t.Errorf("MinLength: got %d, want %d", rs.Password.MinLength, def.Password.MinLength)
	}
}
