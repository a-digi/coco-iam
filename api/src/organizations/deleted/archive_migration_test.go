package deleted

import (
	"os"
	"path/filepath"
	"testing"
)

// seedArchiveEntry creates one subfolder inside `parent` containing a
// single marker file, so the migration tests can assert by name
// whether the folder moved or not.
func seedArchiveEntry(t *testing.T, parent, name string) {
	t.Helper()
	dir := filepath.Join(parent, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "marker"), []byte(name), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}
}

func TestMigrateLegacyArchiveDir_MissingIsNoop(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, "does-not-exist")
	newRoot := filepath.Join(root, "deleted")

	report, err := MigrateLegacyArchiveDir(legacy, newRoot)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if len(report.Moved) != 0 || len(report.Skipped) != 0 || len(report.Failures) != 0 {
		t.Errorf("want empty report, got %+v", report)
	}
	// Missing legacy → no new root created either (or it's fine if the
	// MkdirAll created an empty one; verify that's harmless).
	// The migration's current behaviour: always mkdir newRoot before
	// reading legacy. Accept either outcome — both are harmless.
}

func TestMigrateLegacyArchiveDir_MovesAllEntries(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, "deleted_databases")
	newRoot := filepath.Join(root, "deleted")
	seedArchiveEntry(t, legacy, "20250101_120000__org-A")
	seedArchiveEntry(t, legacy, "org-B")

	report, err := MigrateLegacyArchiveDir(legacy, newRoot)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if len(report.Moved) != 2 {
		t.Errorf("want 2 moved, got %v", report.Moved)
	}
	// Both entries present at new location.
	for _, name := range []string{"20250101_120000__org-A", "org-B"} {
		path := filepath.Join(newRoot, name, "marker")
		if _, err := os.Stat(path); err != nil {
			t.Errorf("want %s at new location: %v", path, err)
		}
	}
	// Legacy root removed.
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Errorf("legacy root should be gone, stat err=%v", err)
	}
}

func TestMigrateLegacyArchiveDir_DoesNotOverwriteExistingEntries(t *testing.T) {
	// If the admin pre-created a folder at the new location (or a
	// partial previous run already moved it), the migration must
	// skip that entry and leave both copies for manual reconciliation
	// — never silently overwrite archived audit material.
	root := t.TempDir()
	legacy := filepath.Join(root, "deleted_databases")
	newRoot := filepath.Join(root, "deleted")
	seedArchiveEntry(t, legacy, "org-X")
	// Pre-existing at the new location with different content.
	existingDir := filepath.Join(newRoot, "org-X")
	if err := os.MkdirAll(existingDir, 0o755); err != nil {
		t.Fatalf("mkdir existing: %v", err)
	}
	if err := os.WriteFile(filepath.Join(existingDir, "marker"), []byte("EXISTING"), 0o644); err != nil {
		t.Fatalf("write existing: %v", err)
	}

	report, err := MigrateLegacyArchiveDir(legacy, newRoot)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if len(report.Moved) != 0 || len(report.Skipped) != 1 {
		t.Errorf("want 0 moved / 1 skipped, got %+v", report)
	}
	// The pre-existing marker survives.
	got, err := os.ReadFile(filepath.Join(newRoot, "org-X", "marker"))
	if err != nil {
		t.Fatalf("read marker: %v", err)
	}
	if string(got) != "EXISTING" {
		t.Errorf("existing archive must survive untouched, got %q", got)
	}
	// Legacy copy is left in place — operator decides what to do.
	if _, err := os.Stat(filepath.Join(legacy, "org-X", "marker")); err != nil {
		t.Errorf("legacy copy should be left for manual reconciliation: %v", err)
	}
}

func TestMigrateLegacyArchiveDir_Idempotent(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, "deleted_databases")
	newRoot := filepath.Join(root, "deleted")
	seedArchiveEntry(t, legacy, "org-Z")

	if _, err := MigrateLegacyArchiveDir(legacy, newRoot); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	// Second run should be a clean no-op.
	report, err := MigrateLegacyArchiveDir(legacy, newRoot)
	if err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	if len(report.Moved) != 0 || len(report.Skipped) != 0 {
		t.Errorf("second run should be a no-op, got %+v", report)
	}
}

func TestMigrateLegacyArchiveDir_LegacyFileIsError(t *testing.T) {
	// If an operator creates a file (not dir) at the legacy path,
	// we must error rather than silently misinterpret it.
	root := t.TempDir()
	legacy := filepath.Join(root, "deleted_databases")
	if err := os.WriteFile(legacy, []byte("stray"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	newRoot := filepath.Join(root, "deleted")

	_, err := MigrateLegacyArchiveDir(legacy, newRoot)
	if err == nil {
		t.Fatal("expected error when legacyRoot is not a directory")
	}
}

func TestMigrateLegacyArchiveDir_PromotesFlatDeletedIntoOrganizationSubfolder(t *testing.T) {
	// Exercises the second pass main.go runs: entries currently
	// flat under `./data/db/deleted/<orgID>/` need to move into
	// `./data/db/deleted/organization/<orgID>/`. The `organization/`
	// subfolder itself must be skipped so we don't try to move the
	// destination into itself.
	root := t.TempDir()
	deletedRoot := filepath.Join(root, "deleted")
	newRoot := filepath.Join(deletedRoot, "organization")
	seedArchiveEntry(t, deletedRoot, "org-A")
	seedArchiveEntry(t, deletedRoot, "org-B")
	// Pre-existing organization/ folder with a row — must be left
	// alone by the migration (it's the destination).
	if err := os.MkdirAll(newRoot, 0o755); err != nil {
		t.Fatalf("mkdir newRoot: %v", err)
	}
	seedArchiveEntry(t, newRoot, "org-C-already-nested")

	report, err := MigrateLegacyArchiveDir(deletedRoot, newRoot, "organization")
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if len(report.Moved) != 2 {
		t.Errorf("want org-A + org-B moved, got %v", report.Moved)
	}
	// Expected new layout:
	//   deleted/organization/org-A/marker
	//   deleted/organization/org-B/marker
	//   deleted/organization/org-C-already-nested/marker (untouched)
	for _, name := range []string{"org-A", "org-B", "org-C-already-nested"} {
		path := filepath.Join(newRoot, name, "marker")
		if _, err := os.Stat(path); err != nil {
			t.Errorf("want %s at new location: %v", path, err)
		}
	}
	// Flat locations should be gone.
	for _, name := range []string{"org-A", "org-B"} {
		if _, err := os.Stat(filepath.Join(deletedRoot, name)); !os.IsNotExist(err) {
			t.Errorf("old flat location for %s should be gone, stat err=%v", name, err)
		}
	}
	// deletedRoot itself still exists (organization/ is inside) —
	// the migration must not have removed it.
	if _, err := os.Stat(deletedRoot); err != nil {
		t.Errorf("deletedRoot should survive (it contains organization/): %v", err)
	}
}

func TestMigrateLegacyArchiveDir_DoesNotRemoveSharedRoot(t *testing.T) {
	// When newRoot is nested inside legacyRoot, removing legacyRoot
	// after draining would delete newRoot too. This test pins that
	// the migration refuses to do that.
	root := t.TempDir()
	deletedRoot := filepath.Join(root, "deleted")
	newRoot := filepath.Join(deletedRoot, "organization")
	// No siblings in deletedRoot besides organization/ — legacyRoot
	// is "empty" from the migration's perspective after the skip.
	if err := os.MkdirAll(newRoot, 0o755); err != nil {
		t.Fatalf("mkdir newRoot: %v", err)
	}
	// Put material under newRoot so a mistaken removal would be
	// catastrophic and detectable.
	seedArchiveEntry(t, newRoot, "org-important")

	if _, err := MigrateLegacyArchiveDir(deletedRoot, newRoot, "organization"); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// Critical assertion: newRoot and its contents must survive.
	if _, err := os.Stat(filepath.Join(newRoot, "org-important", "marker")); err != nil {
		t.Fatalf("organization/ contents must not be deleted: %v", err)
	}
	if _, err := os.Stat(deletedRoot); err != nil {
		t.Errorf("deletedRoot must survive because newRoot is inside it: %v", err)
	}
}
