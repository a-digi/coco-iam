package loginpage

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

const AssetCapBytes int64 = 2 * 1024 * 1024

var allowedMimes = map[string]string{
	"image/png":  "png",
	"image/jpeg": "jpg",
	"image/webp": "webp",
}

func ExtForMime(mime string) string { return allowedMimes[mime] }

var (
	ErrMimeNotAllowed = errors.New("unsupported image type; only PNG, JPEG, and WebP are accepted")
	ErrTooLarge       = errors.New("image is too large")
)

// DetectAndValidateMime sniffs the first bytes and cross-checks
// against the client claim. Discrepancies are refused.
func DetectAndValidateMime(head []byte, claimed string) (string, error) {
	sniffed := http.DetectContentType(head)
	if _, ok := allowedMimes[sniffed]; !ok {
		return "", ErrMimeNotAllowed
	}
	if claimed != "" && claimed != sniffed {
		return "", ErrMimeNotAllowed
	}
	return sniffed, nil
}

// FileStore persists asset bytes under a root directory.
type FileStore struct {
	Root string
}

func NewFileStore(root string) (*FileStore, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("loginpage: create uploads root %q: %w", root, err)
	}
	return &FileStore{Root: root}, nil
}

// Write saves a file under <root>/<appID>/<uuid>.<ext> and returns the
// relative path (appID/uuid.ext) for storage in the DB.
func (s *FileStore) Write(appID, ext string, data []byte) (string, error) {
	dir := filepath.Join(s.Root, appID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("loginpage: create app dir: %w", err)
	}
	name := newID() + "." + ext
	full := filepath.Join(dir, name)
	if err := os.WriteFile(full, data, 0o644); err != nil {
		return "", fmt.Errorf("loginpage: write asset: %w", err)
	}
	return filepath.Join(appID, name), nil
}

// Read returns bytes at the given relative path, with a traversal guard.
func (s *FileStore) Read(rel string) ([]byte, error) {
	full := filepath.Join(s.Root, rel)
	clean := filepath.Clean(full)
	rootClean := filepath.Clean(s.Root) + string(os.PathSeparator)
	if len(clean) < len(rootClean) || clean[:len(rootClean)] != rootClean {
		return nil, errors.New("loginpage: asset path escapes uploads root")
	}
	return os.ReadFile(clean)
}

func (s *FileStore) Delete(rel string) error {
	full := filepath.Join(s.Root, rel)
	if err := os.Remove(full); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("loginpage: delete asset: %w", err)
	}
	return nil
}

func newID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "00000000-0000-0000-0000-000000000000"
	}
	hx := hex.EncodeToString(buf)
	return fmt.Sprintf("%s-%s-%s-%s-%s", hx[:8], hx[8:12], hx[12:16], hx[16:20], hx[20:32])
}
