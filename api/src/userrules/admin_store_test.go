package userrules

import (
	"database/sql"
	"testing"

	db "github.com/a-digi/coco-orm/orm"
	_ "github.com/mattn/go-sqlite3"
)

const adminRuleSetsSchema = `
	CREATE TABLE admin_user_rule_sets (
		id TEXT PRIMARY KEY,
		rules_json TEXT NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
`

func freshAdminRuleStore(t *testing.T) *AdminStore {
	t.Helper()
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if _, err := sqlDB.Exec(adminRuleSetsSchema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return &AdminStore{db: sqlDB}
}

func freshAdminRuleStoreFromManager(t *testing.T) *AdminStore {
	t.Helper()
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if _, err := sqlDB.Exec(adminRuleSetsSchema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return NewAdminStore(&db.DatabaseManager{Connector: &db.Connector{DB: sqlDB}})
}

func TestAdminStore_GetReturnsDefaults_WhenNoRow(t *testing.T) {
	s := freshAdminRuleStore(t)
	rs, err := s.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	def := Defaults()
	if rs.Password.MinLength != def.Password.MinLength {
		t.Errorf("MinLength: got %d, want %d", rs.Password.MinLength, def.Password.MinLength)
	}
}

func TestAdminStore_UpsertAndGet_RoundTrips(t *testing.T) {
	s := freshAdminRuleStore(t)
	want := Defaults()
	want.Password.MinLength = 12
	want.Password.MaxLength = 64
	want.Username.MinLength = 5

	if err := s.Upsert(want); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got, err := s.Get()
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Password.MinLength != want.Password.MinLength {
		t.Errorf("Password.MinLength: got %d, want %d", got.Password.MinLength, want.Password.MinLength)
	}
	if got.Password.MaxLength != want.Password.MaxLength {
		t.Errorf("Password.MaxLength: got %d, want %d", got.Password.MaxLength, want.Password.MaxLength)
	}
	if got.Username.MinLength != want.Username.MinLength {
		t.Errorf("Username.MinLength: got %d, want %d", got.Username.MinLength, want.Username.MinLength)
	}
}

func TestAdminStore_Upsert_IsIdempotent(t *testing.T) {
	s := freshAdminRuleStore(t)

	first := Defaults()
	first.Password.MinLength = 10
	if err := s.Upsert(first); err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	second := Defaults()
	second.Password.MinLength = 16
	if err := s.Upsert(second); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	got, err := s.Get()
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Password.MinLength != 16 {
		t.Errorf("MinLength after second upsert: got %d, want 16", got.Password.MinLength)
	}

	// Confirm exactly one row in the table.
	var count int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM admin_user_rule_sets`).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 row, got %d", count)
	}
}

func TestAdminStore_NewAdminStore_ConstructorWorks(t *testing.T) {
	s := freshAdminRuleStoreFromManager(t)
	if s == nil || s.db == nil {
		t.Fatal("NewAdminStore returned nil or empty store")
	}
}
