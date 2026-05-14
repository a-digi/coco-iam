package keys

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// -- File store --------------------------------------------------------

// ResolveOrgIDFunc maps an application id to the organization id that
// owns it. The FileStore calls this on every read/write so the
// on-disk path can nest the PEM material under the correct
// per-organization folder. A missing application or a broken chain
// surfaces as an error from the resolver — PEM ops then fail loud
// rather than silently writing to the wrong location.
type ResolveOrgIDFunc func(appID string) (string, error)

// FileStore persists the actual PEM bytes per `(application_id, kid)`.
// It never interprets the files — that's the caller's job. Private PEMs
// are chmod 0600; public PEMs are chmod 0644.
//
// Layout on disk:
//
//	<Root>/organization/<orgID>/auth/<appID>/<kid>/{private,public}.pem
//
// `Root` is the data base directory (e.g. "./data/db"). Org id is
// resolved per-call from the injected ResolveOrgIDFunc.
type FileStore struct {
	Root       string
	resolveOrg ResolveOrgIDFunc
}

// NewFileStore builds a file store rooted at baseDir. The caller must
// supply a resolver that maps appID → orgID (typically a closure over
// the main DB manager that runs the app→workspace→organization JOIN).
func NewFileStore(baseDir string, resolveOrg ResolveOrgIDFunc) (*FileStore, error) {
	if resolveOrg == nil {
		return nil, fmt.Errorf("keys: resolver is required")
	}
	abs, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, fmt.Errorf("keys: resolve storage root: %w", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("keys: mkdir storage root: %w", err)
	}
	return &FileStore{Root: abs, resolveOrg: resolveOrg}, nil
}

// keyDir returns `<Root>/organization/<orgID>/auth/<appID>/<kid>`.
// Returns the resolver error unchanged so callers can distinguish
// "unknown app" from a filesystem problem.
func (s *FileStore) keyDir(appID, kid string) (string, error) {
	orgID, err := s.resolveOrg(appID)
	if err != nil {
		return "", fmt.Errorf("keys: resolve org for app %s: %w", appID, err)
	}
	if orgID == "" {
		return "", fmt.Errorf("keys: empty org id for app %s", appID)
	}
	return filepath.Join(s.Root, "organization", orgID, "auth", appID, kid), nil
}

// Write drops the two PEM files into
// `<root>/organization/<orgID>/auth/<appID>/<kid>/`.
// Re-writes are idempotent: if the files already exist they're
// replaced. That's by design — admins who regenerate get a fresh
// pair under the same kid only if they're re-invoked on the same
// pending id, which doesn't happen in normal flow.
func (s *FileStore) Write(appID, kid string, privPEM, pubPEM []byte) error {
	if appID == "" || kid == "" {
		return fmt.Errorf("keys: application id and kid required")
	}
	dir, err := s.keyDir(appID, kid)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("keys: mkdir %s: %w", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "private.pem"), privPEM, 0o600); err != nil {
		return fmt.Errorf("keys: write private: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "public.pem"), pubPEM, 0o644); err != nil {
		return fmt.Errorf("keys: write public: %w", err)
	}
	return nil
}

// Delete removes a kid's directory entirely — called from DiscardPending
// (nothing else ever deletes; active/deactivated files stay on disk
// forever).
func (s *FileStore) Delete(appID, kid string) error {
	if appID == "" || kid == "" {
		return fmt.Errorf("keys: application id and kid required")
	}
	dir, err := s.keyDir(appID, kid)
	if err != nil {
		return err
	}
	return os.RemoveAll(dir)
}

func (s *FileStore) ReadPrivatePEM(appID, kid string) ([]byte, error) {
	return s.readPEM(appID, kid, "private.pem")
}

func (s *FileStore) ReadPublicPEM(appID, kid string) ([]byte, error) {
	return s.readPEM(appID, kid, "public.pem")
}

func (s *FileStore) readPEM(appID, kid, name string) ([]byte, error) {
	if appID == "" || kid == "" {
		return nil, fmt.Errorf("keys: application id and kid required")
	}
	dir, err := s.keyDir(appID, kid)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("keys: read %s: %w", name, err)
	}
	return data, nil
}

// MigrationReport summarises what MigrateFromLegacy did. Moved lists
// application ids whose folders were successfully relocated; Skipped
// lists ones the resolver rejected (orphan apps whose row no longer
// exists) or whose destination already held material (previous
// partial migration). Failures carry the raw error for ops triage.
type MigrationReport struct {
	Moved    []string
	Skipped  []string
	Failures map[string]error
}

// MigrateFromLegacy walks the pre-migration tree rooted at
// `legacyRoot` (typically "./data/appkeys"), resolves each app to its
// organization, and moves `<legacyRoot>/<appID>/` to
// `<Root>/organization/<orgID>/auth/<appID>/`.
//
// Idempotent: a missing legacyRoot returns an empty report and no
// error; a destination that already exists is skipped (so a partial
// previous run doesn't double-move). The legacy root is removed when
// empty.
func (s *FileStore) MigrateFromLegacy(legacyRoot string) (MigrationReport, error) {
	report := MigrationReport{Failures: map[string]error{}}

	info, err := os.Stat(legacyRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return report, nil
		}
		return report, fmt.Errorf("keys migration: stat %s: %w", legacyRoot, err)
	}
	if !info.IsDir() {
		return report, fmt.Errorf("keys migration: %s is not a directory", legacyRoot)
	}

	entries, err := os.ReadDir(legacyRoot)
	if err != nil {
		return report, fmt.Errorf("keys migration: read %s: %w", legacyRoot, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		appID := entry.Name()
		orgID, rerr := s.resolveOrg(appID)
		if rerr != nil || orgID == "" {
			// Orphan: app row is gone (or never existed). Leave the
			// folder in place so ops can inspect / archive manually.
			report.Skipped = append(report.Skipped, appID)
			continue
		}
		src := filepath.Join(legacyRoot, appID)
		dst := filepath.Join(s.Root, "organization", orgID, "auth", appID)
		if _, err := os.Stat(dst); err == nil {
			// Destination exists — a previous run already moved this
			// app. Don't overwrite; skip so the existing material
			// stays authoritative.
			report.Skipped = append(report.Skipped, appID)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
			report.Failures[appID] = fmt.Errorf("mkdir parent: %w", err)
			continue
		}
		if err := os.Rename(src, dst); err != nil {
			report.Failures[appID] = fmt.Errorf("rename: %w", err)
			continue
		}
		report.Moved = append(report.Moved, appID)
	}

	// If the legacy root is now empty (no orphan folders left),
	// remove it so a future `ls data/` is tidy. Non-empty = leave
	// alone; ops decides what to do with the stragglers.
	remaining, rerr := os.ReadDir(legacyRoot)
	if rerr == nil && len(remaining) == 0 {
		_ = os.Remove(legacyRoot)
	}
	return report, nil
}

// -- DB store ----------------------------------------------------------

// AppDBResolver maps an application id to the per-org *sql.DB that
// holds the application_keys rows for that application.
type AppDBResolver func(appID string) (*sql.DB, error)

// Store is the DB-backed metadata store for key rows. Pairs with
// FileStore for the actual PEM bytes.
type Store struct {
	resolve AppDBResolver
}

func NewStore(resolve AppDBResolver) *Store {
	return &Store{resolve: resolve}
}

// Insert adds a new row. The caller is expected to have generated the
// PEM files before calling this so the DB row never references missing
// material.
func (s *Store) Insert(row KeyRow) error {
	db, err := s.resolve(row.ApplicationID)
	if err != nil {
		return fmt.Errorf("keys: resolve db for insert: %w", err)
	}
	_, err = db.Exec(
		`INSERT INTO application_keys
		   (id, application_id, status, created_at, activated_at, deactivated_at, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		row.ID, row.ApplicationID, string(row.Status),
		row.CreatedAt, toNullTime(row.ActivatedAt),
		toNullTime(row.DeactivatedAt), toNullTime(row.ExpiresAt),
	)
	if err != nil {
		return fmt.Errorf("keys: insert row: %w", err)
	}
	return nil
}

// UpdateStatus atomically moves a row between lifecycle states. Timestamps
// that are nil on the caller side become SQL NULL in the row so we can
// clear them (e.g., discarding a pending key before insert is never
// needed but we support it for symmetry).
func (s *Store) UpdateStatus(appID, id string, status KeyStatus, activatedAt, deactivatedAt, expiresAt *time.Time) error {
	db, err := s.resolve(appID)
	if err != nil {
		return fmt.Errorf("keys: resolve db for update: %w", err)
	}
	_, err = db.Exec(
		`UPDATE application_keys
		    SET status = ?, activated_at = ?, deactivated_at = ?, expires_at = ?
		  WHERE id = ?`,
		string(status), toNullTime(activatedAt), toNullTime(deactivatedAt), toNullTime(expiresAt), id,
	)
	if err != nil {
		return fmt.Errorf("keys: update row: %w", err)
	}
	return nil
}

// Delete removes one row. Only the pending-discard path uses this;
// active + deactivated rows stay forever.
func (s *Store) Delete(appID, id string) error {
	db, err := s.resolve(appID)
	if err != nil {
		return fmt.Errorf("keys: resolve db for delete: %w", err)
	}
	_, err = db.Exec(`DELETE FROM application_keys WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("keys: delete row: %w", err)
	}
	return nil
}

// Get returns one row by id, or ErrNotFound.
func (s *Store) Get(appID, id string) (KeyRow, error) {
	db, err := s.resolve(appID)
	if err != nil {
		return KeyRow{}, fmt.Errorf("keys: resolve db for get: %w", err)
	}
	row := db.QueryRow(
		`SELECT id, application_id, status, created_at, activated_at, deactivated_at, expires_at
		 FROM application_keys WHERE id = ?`, id,
	)
	return scanKeyRow(row)
}

// List returns every row for one application, newest first, regardless
// of status. Callers filter for expiry / status as needed.
func (s *Store) List(appID string) ([]KeyRow, error) {
	db, err := s.resolve(appID)
	if err != nil {
		return nil, fmt.Errorf("keys: resolve db for list: %w", err)
	}
	rows, err := db.Query(
		`SELECT id, application_id, status, created_at, activated_at, deactivated_at, expires_at
		 FROM application_keys WHERE application_id = ?
		 ORDER BY created_at DESC`, appID,
	)
	if err != nil {
		return nil, fmt.Errorf("keys: list rows: %w", err)
	}
	defer rows.Close()
	var out []KeyRow
	for rows.Next() {
		r, err := scanKeyRowRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Active returns the one active row for an application.
func (s *Store) Active(appID string) (KeyRow, error) {
	db, err := s.resolve(appID)
	if err != nil {
		return KeyRow{}, fmt.Errorf("keys: resolve db for active: %w", err)
	}
	row := db.QueryRow(
		`SELECT id, application_id, status, created_at, activated_at, deactivated_at, expires_at
		 FROM application_keys WHERE application_id = ? AND status = ?
		 LIMIT 1`, appID, string(StatusActive),
	)
	return scanKeyRow(row)
}

// Pending returns the pending row if one exists; ErrNotFound otherwise.
func (s *Store) Pending(appID string) (KeyRow, error) {
	db, err := s.resolve(appID)
	if err != nil {
		return KeyRow{}, fmt.Errorf("keys: resolve db for pending: %w", err)
	}
	row := db.QueryRow(
		`SELECT id, application_id, status, created_at, activated_at, deactivated_at, expires_at
		 FROM application_keys WHERE application_id = ? AND status = ?
		 LIMIT 1`, appID, string(StatusPending),
	)
	return scanKeyRow(row)
}

// -- helpers -----------------------------------------------------------

func toNullTime(t *time.Time) interface{} {
	if t == nil || t.IsZero() {
		return nil
	}
	return *t
}

type rowScanner interface {
	Scan(dest ...interface{}) error
}

func scanKeyRow(row rowScanner) (KeyRow, error) {
	var (
		r                              KeyRow
		status                         string
		createdAt                      sql.NullString
		activatedAt, deactivatedAt, ex sql.NullString
	)
	err := row.Scan(&r.ID, &r.ApplicationID, &status, &createdAt, &activatedAt, &deactivatedAt, &ex)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return KeyRow{}, ErrNotFound
		}
		return KeyRow{}, fmt.Errorf("keys: scan row: %w", err)
	}
	r.Status = KeyStatus(status)
	if createdAt.Valid {
		if t, err := parseSQLTime(createdAt.String); err == nil {
			r.CreatedAt = t
		}
	}
	r.ActivatedAt = ptrTimeFromNullString(activatedAt)
	r.DeactivatedAt = ptrTimeFromNullString(deactivatedAt)
	r.ExpiresAt = ptrTimeFromNullString(ex)
	return r, nil
}

// scanKeyRowRows duplicates scanKeyRow with the *sql.Rows interface.
// The two scanners return different sentinels for "no row" so we keep
// them apart.
func scanKeyRowRows(rows *sql.Rows) (KeyRow, error) {
	var (
		r                              KeyRow
		status                         string
		createdAt                      sql.NullString
		activatedAt, deactivatedAt, ex sql.NullString
	)
	if err := rows.Scan(&r.ID, &r.ApplicationID, &status, &createdAt, &activatedAt, &deactivatedAt, &ex); err != nil {
		return KeyRow{}, fmt.Errorf("keys: scan row: %w", err)
	}
	r.Status = KeyStatus(status)
	if createdAt.Valid {
		if t, err := parseSQLTime(createdAt.String); err == nil {
			r.CreatedAt = t
		}
	}
	r.ActivatedAt = ptrTimeFromNullString(activatedAt)
	r.DeactivatedAt = ptrTimeFromNullString(deactivatedAt)
	r.ExpiresAt = ptrTimeFromNullString(ex)
	return r, nil
}

func ptrTimeFromNullString(s sql.NullString) *time.Time {
	if !s.Valid || s.String == "" {
		return nil
	}
	t, err := parseSQLTime(s.String)
	if err != nil {
		return nil
	}
	return &t
}

func parseSQLTime(s string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05.999999999-07:00", "2006-01-02 15:04:05.999-07:00", "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("keys: unparseable timestamp %q", s)
}
