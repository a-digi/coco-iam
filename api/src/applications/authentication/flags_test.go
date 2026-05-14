package authentication

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// Unit coverage for passwordLoginAllowed — the one-line gate the
// login handler + auth-methods handler share. The legacy
// behaviour we MUST preserve is: when the column / row is
// missing, legacy password login stays on. Failing this pins
// would silently lock live deployments out after a migration
// lag window.

func openAppsDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	_, err = db.Exec(`
		CREATE TABLE applications (
		    id TEXT PRIMARY KEY,
		    allow_password_login BOOLEAN NOT NULL DEFAULT 1
		);
	`)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	return db
}

func TestPasswordLoginAllowed_Default(t *testing.T) {
	db := openAppsDB(t)
	if _, err := db.Exec(`INSERT INTO applications (id) VALUES (?)`, "app-1"); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if !passwordLoginAllowed(db, "app-1") {
		t.Error("missing value should default to allowed (legacy parity)")
	}
}

func TestPasswordLoginAllowed_ExplicitlyEnabled(t *testing.T) {
	db := openAppsDB(t)
	if _, err := db.Exec(`INSERT INTO applications (id, allow_password_login) VALUES (?, 1)`, "app-1"); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if !passwordLoginAllowed(db, "app-1") {
		t.Error("expected allowed")
	}
}

func TestPasswordLoginAllowed_ExplicitlyDisabled(t *testing.T) {
	db := openAppsDB(t)
	if _, err := db.Exec(`INSERT INTO applications (id, allow_password_login) VALUES (?, 0)`, "app-1"); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if passwordLoginAllowed(db, "app-1") {
		t.Error("disabled flag must block password login")
	}
}

func TestPasswordLoginAllowed_UnknownAppIsPermissive(t *testing.T) {
	db := openAppsDB(t)
	// No row with this id — we return true so legacy apps still
	// log in if the id passed here is somehow stale. The login
	// handler has its own existence check via FindBySlugs; this
	// helper is just the kill-switch for the flag.
	if !passwordLoginAllowed(db, "missing") {
		t.Error("missing row should default to allowed")
	}
}

func TestPasswordLoginAllowed_NilDBIsPermissive(t *testing.T) {
	if !passwordLoginAllowed(nil, "any") {
		t.Error("nil DB must not lock admins out — return true")
	}
}

func TestPasswordLoginAllowed_EmptyAppIDIsPermissive(t *testing.T) {
	db := openAppsDB(t)
	if !passwordLoginAllowed(db, "") {
		t.Error("empty app id must not lock users out")
	}
}

func TestPasswordLoginAllowed_MissingColumnIsPermissive(t *testing.T) {
	// Simulate a pre-migration DB where the column hasn't been
	// added yet. The query errors; the helper must return true.
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE applications (id TEXT PRIMARY KEY)`); err != nil {
		t.Fatalf("schema: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO applications (id) VALUES (?)`, "app-1"); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if !passwordLoginAllowed(db, "app-1") {
		t.Error("missing column must not lock users out")
	}
}
