package avatar

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newStore(t *testing.T) *FileStore {
	t.Helper()
	root := t.TempDir()
	fs, err := New(root)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	return fs
}

func TestAssetID_Shape(t *testing.T) {
	cases := []struct {
		admin, ext, want string
	}{
		{"abc", "png", "abc.png"},
		{"abc", ".PNG", "abc.png"}, // strip leading dot, lowercase
		{"abc", "JPG", "abc.jpg"},
	}
	for _, tc := range cases {
		if got := AssetID(tc.admin, tc.ext); got != tc.want {
			t.Errorf("AssetID(%q,%q): got %q want %q", tc.admin, tc.ext, got, tc.want)
		}
	}
}

func TestExtensionOf(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"abc.png", "png"},
		{"abc.JPG", "jpg"},
		{"no-extension", ""},
		{"trailing.", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := ExtensionOf(tc.in); got != tc.want {
			t.Errorf("ExtensionOf(%q): got %q want %q", tc.in, got, tc.want)
		}
	}
}

func TestSave_WritesFileAtExpectedPath(t *testing.T) {
	fs := newStore(t)
	payload := []byte("FAKE_PNG_BYTES")

	assetID, err := fs.Save("user-1", bytes.NewReader(payload), "png")
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if assetID != "user-1.png" {
		t.Errorf("asset id: want user-1.png, got %q", assetID)
	}
	got, err := os.ReadFile(filepath.Join(fs.Root, assetID))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("round-trip: got %q want %q", got, payload)
	}
}

func TestSave_RejectsUnknownExtension(t *testing.T) {
	// Whitelist is the only defence against an admin uploading,
	// say, an executable disguised as an image. Must reject before
	// anything touches disk.
	fs := newStore(t)
	_, err := fs.Save("user-1", bytes.NewReader([]byte("bad")), "exe")
	if !errors.Is(err, ErrInvalidExtension) {
		t.Errorf("want ErrInvalidExtension, got %v", err)
	}
	// Nothing should have been written.
	entries, _ := os.ReadDir(fs.Root)
	if len(entries) > 0 {
		t.Errorf("no files should be written on rejection, found %d entries", len(entries))
	}
}

func TestSave_OverwriteIsAtomic(t *testing.T) {
	// Two sequential saves with the same extension replace the
	// file in place. Catch a regression where the rename path
	// accidentally appended instead.
	fs := newStore(t)
	first := []byte("AAAAAAAA")
	second := []byte("BBBBBBBB")

	if _, err := fs.Save("user-1", bytes.NewReader(first), "png"); err != nil {
		t.Fatalf("first save: %v", err)
	}
	if _, err := fs.Save("user-1", bytes.NewReader(second), "png"); err != nil {
		t.Fatalf("second save: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(fs.Root, "user-1.png"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(got, second) {
		t.Errorf("second save should overwrite: got %q", got)
	}
}

func TestSave_DoesNotLeaveTempFile(t *testing.T) {
	// If the rename succeeded, no "avatar-*.png" tempfile should
	// linger in the root — the defer-cleanup path only runs on
	// failure and the rename consumes the temp on success.
	fs := newStore(t)
	if _, err := fs.Save("user-1", bytes.NewReader([]byte("x")), "png"); err != nil {
		t.Fatalf("save: %v", err)
	}
	entries, _ := os.ReadDir(fs.Root)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "avatar-") {
			t.Errorf("stray temp file: %s", e.Name())
		}
	}
}

func TestOpen_ReturnsErrNotFoundForMissing(t *testing.T) {
	fs := newStore(t)
	_, err := fs.Open("user-ghost.png")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestOpen_RoundTrip(t *testing.T) {
	fs := newStore(t)
	payload := []byte("PNGDATA")
	assetID, err := fs.Save("user-1", bytes.NewReader(payload), "png")
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	rc, err := fs.Open(assetID)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer rc.Close()
	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("round-trip mismatch: got %q", got)
	}
}

func TestDelete_IdempotentOnMissing(t *testing.T) {
	// Clear-avatar handler calls Delete first then clears the DB
	// row. If Delete errored on missing files, a broken state
	// (DB says avatar exists, file already gone) would be
	// unrecoverable. Must be a silent no-op.
	fs := newStore(t)
	if err := fs.Delete("nonexistent.png"); err != nil {
		t.Errorf("delete on missing file should be no-op, got %v", err)
	}
}

func TestDelete_RemovesFile(t *testing.T) {
	fs := newStore(t)
	assetID, _ := fs.Save("user-1", bytes.NewReader([]byte("x")), "png")
	if err := fs.Delete(assetID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := os.Stat(filepath.Join(fs.Root, assetID)); !os.IsNotExist(err) {
		t.Errorf("file should be gone, stat err=%v", err)
	}
}
