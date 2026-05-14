// Package avatar owns the tiny per-admin avatar blob store at
// ./data/uploads/admin-avatars/. One file per admin
// (<admin_user_id>.<ext>); uploads overwrite any prior file
// atomically via tempfile + rename so a crashed upload can't leave
// the admin with a half-written image.
package avatar

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// AllowedExtensions is the whitelist the upload handler enforces
// via this package. Lowercase, no leading dot.
var AllowedExtensions = map[string]struct{}{
	"png":  {},
	"jpg":  {},
	"jpeg": {},
	"webp": {},
	"gif":  {},
}

// ErrInvalidExtension signals the caller tried to save or serve a
// file whose extension isn't on the whitelist. Translates to 415
// in the upload handler.
var ErrInvalidExtension = errors.New("avatar: extension not allowed")

// ErrNotFound signals the requested asset id has no file on disk.
// The public serve handler translates this to 404.
var ErrNotFound = errors.New("avatar: not found")

// FileStore persists the avatar bytes. Every caller shares a single
// FileStore constructed at startup.
type FileStore struct {
	Root string
}

// New builds a FileStore rooted at the given absolute or
// process-wd-relative directory. The directory is created lazily on
// first write; missing at construction time is not an error.
func New(root string) (*FileStore, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("avatar: resolve root: %w", err)
	}
	return &FileStore{Root: abs}, nil
}

// AssetID composes the stored filename for an admin + file
// extension. Kept as a package-level function so handlers don't
// hard-code the format.
func AssetID(adminUserID, ext string) string {
	return adminUserID + "." + strings.ToLower(strings.TrimPrefix(ext, "."))
}

// ExtensionOf extracts the extension from an asset id (or empty if
// the id is malformed). Used by the serve handler to set
// Content-Type.
func ExtensionOf(assetID string) string {
	idx := strings.LastIndex(assetID, ".")
	if idx < 0 || idx == len(assetID)-1 {
		return ""
	}
	return strings.ToLower(assetID[idx+1:])
}

// Save writes the bytes from src into
// <Root>/<adminUserID>.<ext>, atomically replacing any prior file.
// Extension is the file type without the leading dot.
// Overwrites are safe for crashes (temp file + rename).
func (s *FileStore) Save(adminUserID string, src io.Reader, ext string) (assetID string, err error) {
	if adminUserID == "" {
		return "", errors.New("avatar: admin user id required")
	}
	ext = strings.ToLower(strings.TrimPrefix(ext, "."))
	if _, ok := AllowedExtensions[ext]; !ok {
		return "", ErrInvalidExtension
	}
	if err := os.MkdirAll(s.Root, 0o755); err != nil {
		return "", fmt.Errorf("avatar: mkdir root: %w", err)
	}
	assetID = AssetID(adminUserID, ext)
	finalPath := filepath.Join(s.Root, assetID)

	// Tempfile in the same directory so os.Rename stays a cheap
	// inode-level move on the same filesystem.
	tmp, err := os.CreateTemp(s.Root, "avatar-*."+ext)
	if err != nil {
		return "", fmt.Errorf("avatar: create temp: %w", err)
	}
	tmpPath := tmp.Name()
	// Remove the temp file on any failure exit — caller never sees
	// it. On success the rename below consumes it.
	cleanupTmp := true
	defer func() {
		if cleanupTmp {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := io.Copy(tmp, src); err != nil {
		tmp.Close()
		return "", fmt.Errorf("avatar: copy: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("avatar: close temp: %w", err)
	}
	if err := os.Chmod(tmpPath, 0o644); err != nil {
		return "", fmt.Errorf("avatar: chmod: %w", err)
	}
	// If the admin previously uploaded a different extension
	// (e.g. .png then .jpg), we leave the old file behind so the
	// serve endpoint still finds the NEW asset id. Upload
	// callers are responsible for deleting the prior asset if
	// they care about disk tidiness — the handler does this via
	// Delete(prevAssetID) after successful Save.
	if err := os.Rename(tmpPath, finalPath); err != nil {
		return "", fmt.Errorf("avatar: rename: %w", err)
	}
	cleanupTmp = false
	return assetID, nil
}

// Open returns an io.ReadCloser for the given asset id. The caller
// MUST close it. A missing file returns ErrNotFound.
func (s *FileStore) Open(assetID string) (io.ReadCloser, error) {
	if assetID == "" {
		return nil, ErrNotFound
	}
	f, err := os.Open(filepath.Join(s.Root, assetID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("avatar: open: %w", err)
	}
	return f, nil
}

// Delete removes the file for the given asset id. A missing file is
// not an error — the operation is idempotent so the delete handler
// stays simple.
func (s *FileStore) Delete(assetID string) error {
	if assetID == "" {
		return nil
	}
	path := filepath.Join(s.Root, assetID)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("avatar: delete: %w", err)
	}
	return nil
}
