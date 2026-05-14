package keys

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// fakeResolver returns an in-memory appID → orgID mapping so the
// FileStore can be tested without a real DB.
type fakeResolver struct {
	mapping map[string]string
	err     error
}

func (f fakeResolver) resolve(appID string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	orgID, ok := f.mapping[appID]
	if !ok {
		return "", errors.New("unknown app")
	}
	return orgID, nil
}

// newStore opens a FileStore rooted at `t.TempDir()` with the given
// resolver mapping. Cleanup happens automatically via t.TempDir().
func newStore(t *testing.T, mapping map[string]string) (*FileStore, string) {
	t.Helper()
	dir := t.TempDir()
	r := fakeResolver{mapping: mapping}
	fs, err := NewFileStore(dir, r.resolve)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	return fs, dir
}

func TestNewFileStore_RequiresResolver(t *testing.T) {
	// Pin at construction time: passing a nil resolver is a
	// programmer error — every read/write depends on it.
	_, err := NewFileStore(t.TempDir(), nil)
	if err == nil {
		t.Fatal("nil resolver should be rejected")
	}
}

func TestWriteReadRoundTrip_UsesOrgNestedPath(t *testing.T) {
	fs, root := newStore(t, map[string]string{"app-1": "org-A"})

	priv := []byte("PRIVATE-MATERIAL")
	pub := []byte("PUBLIC-MATERIAL")
	if err := fs.Write("app-1", "kid-1", priv, pub); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Pin the on-disk layout — callers build expectations around
	// "everything org-scoped lives under organization/<uuid>/".
	expected := filepath.Join(root, "organization", "org-A", "auth", "app-1", "kid-1")
	if _, err := os.Stat(expected); err != nil {
		t.Fatalf("expected dir %s to exist: %v", expected, err)
	}

	gotPriv, err := fs.ReadPrivatePEM("app-1", "kid-1")
	if err != nil {
		t.Fatalf("ReadPrivatePEM: %v", err)
	}
	if string(gotPriv) != string(priv) {
		t.Errorf("private round-trip mismatch: got %q", gotPriv)
	}
	gotPub, err := fs.ReadPublicPEM("app-1", "kid-1")
	if err != nil {
		t.Fatalf("ReadPublicPEM: %v", err)
	}
	if string(gotPub) != string(pub) {
		t.Errorf("public round-trip mismatch: got %q", gotPub)
	}
}

func TestWrite_ResolverErrorPropagates(t *testing.T) {
	// An unknown app must not silently write to a bogus path —
	// better to fail loud so the caller investigates.
	fs, _ := newStore(t, map[string]string{})
	err := fs.Write("unknown-app", "kid-1", []byte("x"), []byte("y"))
	if err == nil {
		t.Fatal("expected error for unknown app")
	}
}

func TestReadPrivatePEM_MissingReturnsErrNotFound(t *testing.T) {
	fs, _ := newStore(t, map[string]string{"app-1": "org-A"})
	_, err := fs.ReadPrivatePEM("app-1", "no-such-kid")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestDelete_RemovesKidDirectory(t *testing.T) {
	fs, root := newStore(t, map[string]string{"app-1": "org-A"})
	if err := fs.Write("app-1", "kid-1", []byte("p"), []byte("q")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := fs.Delete("app-1", "kid-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	path := filepath.Join(root, "organization", "org-A", "auth", "app-1", "kid-1")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected %s to be gone, stat err=%v", path, err)
	}
}

func TestWrite_PermissionsPinnedForPrivatePEM(t *testing.T) {
	// Private material must be chmod 0600 — pinning so a future edit
	// that relaxes permissions (e.g. to 0644 on both) fails this
	// test and is caught in review.
	fs, root := newStore(t, map[string]string{"app-1": "org-A"})
	if err := fs.Write("app-1", "kid-1", []byte("priv"), []byte("pub")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	privPath := filepath.Join(root, "organization", "org-A", "auth", "app-1", "kid-1", "private.pem")
	info, err := os.Stat(privPath)
	if err != nil {
		t.Fatalf("stat private: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("private.pem perm: want 0600, got %o", info.Mode().Perm())
	}
}

// ---------- migration ----------

func TestMigrateFromLegacy_HappyPath(t *testing.T) {
	fs, root := newStore(t, map[string]string{
		"app-1": "org-A",
		"app-2": "org-B",
	})
	legacy := filepath.Join(root, "legacy")

	// Seed two app folders with a kid each.
	seedLegacyApp(t, legacy, "app-1", "kid-1")
	seedLegacyApp(t, legacy, "app-2", "kid-2")

	report, err := fs.MigrateFromLegacy(legacy)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if len(report.Moved) != 2 {
		t.Errorf("moved: want 2, got %v", report.Moved)
	}
	if len(report.Failures) != 0 {
		t.Errorf("failures: want 0, got %v", report.Failures)
	}

	// Both apps should now read back from the org-nested path.
	if data, err := fs.ReadPrivatePEM("app-1", "kid-1"); err != nil || string(data) != "priv-app-1" {
		t.Errorf("app-1 after migration: data=%q err=%v", data, err)
	}
	if data, err := fs.ReadPublicPEM("app-2", "kid-2"); err != nil || string(data) != "pub-app-2" {
		t.Errorf("app-2 after migration: data=%q err=%v", data, err)
	}

	// Legacy root should be removed once empty.
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Errorf("legacy root should be removed; stat err=%v", err)
	}
}

func TestMigrateFromLegacy_MissingLegacyIsNoop(t *testing.T) {
	fs, root := newStore(t, map[string]string{})
	report, err := fs.MigrateFromLegacy(filepath.Join(root, "does-not-exist"))
	if err != nil {
		t.Fatalf("want no error, got %v", err)
	}
	if len(report.Moved) != 0 || len(report.Skipped) != 0 {
		t.Errorf("want empty report, got %+v", report)
	}
}

func TestMigrateFromLegacy_OrphansAreSkipped(t *testing.T) {
	// An app folder whose row no longer exists must not block the
	// migration — record it, leave the folder, move on.
	fs, root := newStore(t, map[string]string{"app-known": "org-A"})
	legacy := filepath.Join(root, "legacy")
	seedLegacyApp(t, legacy, "app-known", "kid-1")
	seedLegacyApp(t, legacy, "app-orphan", "kid-2")

	report, err := fs.MigrateFromLegacy(legacy)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if len(report.Moved) != 1 || report.Moved[0] != "app-known" {
		t.Errorf("moved: want [app-known], got %v", report.Moved)
	}
	if len(report.Skipped) != 1 || report.Skipped[0] != "app-orphan" {
		t.Errorf("skipped: want [app-orphan], got %v", report.Skipped)
	}
	// Orphan left in place.
	orphanPath := filepath.Join(legacy, "app-orphan")
	if _, err := os.Stat(orphanPath); err != nil {
		t.Errorf("orphan should be left in place, stat err=%v", err)
	}
	// Legacy root should NOT be removed — there's still an orphan in it.
	if _, err := os.Stat(legacy); err != nil {
		t.Errorf("legacy root should still exist, stat err=%v", err)
	}
}

func TestMigrateFromLegacy_Idempotent(t *testing.T) {
	// Running the migration a second time after a successful first
	// run must be a clean no-op — not overwrite, not error.
	fs, root := newStore(t, map[string]string{"app-1": "org-A"})
	legacy := filepath.Join(root, "legacy")
	seedLegacyApp(t, legacy, "app-1", "kid-1")

	if _, err := fs.MigrateFromLegacy(legacy); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	// Second run: legacy is gone; should return empty report.
	report, err := fs.MigrateFromLegacy(legacy)
	if err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	if len(report.Moved) != 0 {
		t.Errorf("second run should move nothing, got %v", report.Moved)
	}
}

func TestMigrateFromLegacy_DoesNotOverwriteExistingDestination(t *testing.T) {
	// Safety invariant: if the destination already has material
	// (e.g. from a partial previous migration or a direct generate
	// after boot), the migration must not clobber it.
	fs, root := newStore(t, map[string]string{"app-1": "org-A"})
	legacy := filepath.Join(root, "legacy")
	seedLegacyApp(t, legacy, "app-1", "kid-legacy")

	// Plant different material at the new location up front.
	if err := fs.Write("app-1", "kid-new", []byte("new-priv"), []byte("new-pub")); err != nil {
		t.Fatalf("pre-seed new location: %v", err)
	}

	report, err := fs.MigrateFromLegacy(legacy)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if len(report.Moved) != 0 || len(report.Skipped) != 1 {
		t.Errorf("want 0 moved / 1 skipped, got moved=%v skipped=%v", report.Moved, report.Skipped)
	}
	// Existing material untouched.
	got, err := fs.ReadPrivatePEM("app-1", "kid-new")
	if err != nil || string(got) != "new-priv" {
		t.Errorf("existing material must survive: got %q err=%v", got, err)
	}
}

// seedLegacyApp writes a matched pair of PEMs into the old flat
// layout (<legacy>/<appID>/<kid>/{private,public}.pem) so migration
// tests have realistic input.
func seedLegacyApp(t *testing.T, legacy, appID, kid string) {
	t.Helper()
	dir := filepath.Join(legacy, appID, kid)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir legacy: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "private.pem"), []byte("priv-"+appID), 0o600); err != nil {
		t.Fatalf("write priv: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "public.pem"), []byte("pub-"+appID), 0o644); err != nil {
		t.Fatalf("write pub: %v", err)
	}
}
