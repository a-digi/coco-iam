package userprofile

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	clamd "github.com/dutchcoders/go-clamd"
	"github.com/a-digi/coco-iam/src/applications/keys"
	"github.com/a-digi/coco-iam/src/applications/loginpage"
	profile_dbregistry "github.com/a-digi/coco-iam/src/organizations/profile/dbregistry"
	profile_entity "github.com/a-digi/coco-iam/src/organizations/profile/entity"
	users_dbregistry "github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-iam/src/orgrouter"
	"github.com/a-digi/coco-server/server/media"
)

// This file holds the production adapters that connect the
// handler's narrow ports to the concrete services the app wires up
// at startup. Each adapter is a thin closure over one service —
// no business logic, no branching beyond error mapping — so they
// don't carry their own tests.
//
// If an adapter ever grows logic of its own, that logic should
// move into a pure helper and be tested there; adapters must stay
// mechanical.

// -- SlugResolver --------------------------------------------------

// NewLoginpageSlugResolver wraps `*loginpage.Service` as a
// SlugResolver. The production wiring passes the already-resolved
// service from the DI bag.
func NewLoginpageSlugResolver(svc *loginpage.Service) SlugResolver {
	return &loginpageSlugResolver{svc: svc}
}

type loginpageSlugResolver struct {
	svc *loginpage.Service
}

func (r *loginpageSlugResolver) ResolveSlugs(orgSlug, wsSlug, appSlug string) (appID, orgID string, err error) {
	info, err := r.svc.Store.FindBySlugs(orgSlug, wsSlug, appSlug)
	if err != nil {
		return "", "", err
	}
	return info.ID, info.OrganizationID, nil
}

// -- KeyLoader -----------------------------------------------------

// NewKeysServiceKeyLoader wraps `*keys.Service` as a KeyLoader.
// LoadVerifiablePublicKey already enforces the "still verifiable"
// rule (active key, or deactivated key still within its grace
// window); the adapter makes no additional decisions.
func NewKeysServiceKeyLoader(svc *keys.Service) KeyLoader {
	return &keysServiceKeyLoader{svc: svc}
}

type keysServiceKeyLoader struct {
	svc *keys.Service
}

func (l *keysServiceKeyLoader) LoadPublicKey(appID, kid string) (*rsa.PublicKey, error) {
	return l.svc.LoadVerifiablePublicKey(appID, kid)
}

// -- UserOrgReader -------------------------------------------------

// NewOrgRegistryUserOrgReader returns a UserOrgReader that resolves a
// user's org by scanning the per-org DBs via OrgDBFor.
func NewOrgRegistryUserOrgReader(reg *users_dbregistry.OrgUserDBRegistry) UserOrgReader {
	return &orgRegistryUserOrgReader{reg: reg}
}

type orgRegistryUserOrgReader struct {
	reg *users_dbregistry.OrgUserDBRegistry
}

func (r *orgRegistryUserOrgReader) UserOrg(userID string) (string, error) {
	_, orgID, err := orgrouter.OrgDBFor(r.reg, userID)
	if err != nil {
		return "", ErrUserNotFound
	}
	return orgID, nil
}

// -- ProfileReader -------------------------------------------------

// NewOrgRegistryProfileReader wraps the per-org profile DB registry.
// Each call opens (or reuses a cached) `profiles.db` for the target
// org, runs two small SELECTs, and returns the fields + stored
// values. The SQL is narrow enough to inline here — pulling in the
// full `organizations/profile` repository package would drag
// validators the endpoint doesn't need.
func NewOrgRegistryProfileReader(reg *profile_dbregistry.OrgDBRegistry) ProfileReader {
	return &orgRegistryProfileReader{reg: reg}
}

type orgRegistryProfileReader struct {
	reg *profile_dbregistry.OrgDBRegistry
}

func (r *orgRegistryProfileReader) LoadProfile(orgID, userID string) ([]profile_entity.ProfileField, map[string]interface{}, error) {
	db, err := r.reg.For(orgID)
	if err != nil {
		return nil, nil, err
	}
	conn := db.Connector.DB

	rows, err := conn.Query(
		`SELECT id, name, label, description, data_type, is_required,
		        min_value, max_value, options_json, regex,
		        accept_mime, max_bytes, order_index,
		        is_active, created_at, updated_at
		 FROM profile_fields
		 WHERE is_active = 1
		 ORDER BY order_index ASC, created_at ASC`,
	)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var fields []profile_entity.ProfileField
	for rows.Next() {
		var f profile_entity.ProfileField
		var optionsJSON string
		var minVal, maxVal sql.NullInt64
		if err := rows.Scan(
			&f.ID, &f.Name, &f.Label, &f.Description, &f.DataType, &f.IsRequired,
			&minVal, &maxVal, &optionsJSON, &f.Regex,
			&f.AcceptMime, &f.MaxBytes, &f.OrderIndex,
			&f.IsActive, &f.CreatedAt, &f.UpdatedAt,
		); err != nil {
			return nil, nil, err
		}
		if minVal.Valid {
			v := int(minVal.Int64)
			f.MinValue = &v
		}
		if maxVal.Valid {
			v := int(maxVal.Int64)
			f.MaxValue = &v
		}
		if optionsJSON != "" {
			var opts []string
			_ = json.Unmarshal([]byte(optionsJSON), &opts)
			f.Options = opts
		}
		fields = append(fields, f)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	var dataJSON string
	err = conn.QueryRow(
		`SELECT profile_data FROM user_profiles WHERE user_id = ? LIMIT 1`, userID,
	).Scan(&dataJSON)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, nil, err
	}
	data := map[string]interface{}{}
	if dataJSON != "" {
		if jerr := json.Unmarshal([]byte(dataJSON), &data); jerr != nil {
			// Malformed JSON in storage is a data-integrity
			// anomaly, not something this adapter should
			// propagate to the caller — the alternative is
			// failing the whole /me endpoint on one bad row.
			// Render as empty; operators will see the bad row
			// via direct DB inspection.
			data = map[string]interface{}{}
		}
	}
	return fields, data, nil
}

// -- Scanner -------------------------------------------------------

// NewMediaScanner returns a Scanner that forwards to the media
// subsystem's magic-byte validator. One-line wrapper; the whole
// scanning + allowlist behaviour lives in `media` so we never
// drift from it.
func NewMediaScanner() Scanner {
	return &mediaScanner{}
}

type mediaScanner struct{}

func (s *mediaScanner) DetectAndValidate(head []byte, claimedMime string) (string, string, error) {
	return media.DetectAndValidateMime(head, claimedMime)
}

// -- FileStore -----------------------------------------------------

// NewPerOrgUserFileStore returns a FileStore rooted at `root`.
// Files land at
//
//	<root>/organization/<orgID>/uploads/users/<userID>/<assetID>.<ext>
//
// The adapter never interprets the asset id or the extension — the
// handler passes values that were either generated server-side or
// read from the repo. Writes go tempfile → rename to keep partial
// files off disk on crash.
func NewPerOrgUserFileStore(root string) FileStore {
	return &perOrgUserFileStore{root: root}
}

type perOrgUserFileStore struct {
	root string
}

func (s *perOrgUserFileStore) dirFor(orgID, userID string) string {
	return filepath.Join(s.root, "organization", orgID, "uploads", "users", userID)
}

func (s *perOrgUserFileStore) pathFor(orgID, userID, assetID, ext string) string {
	name := assetID
	if ext != "" {
		name = assetID + "." + ext
	}
	return filepath.Join(s.dirFor(orgID, userID), name)
}

func (s *perOrgUserFileStore) Save(orgID, userID, assetID, ext string, data []byte) error {
	dir := s.dirFor(orgID, userID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("userprofile: mkdir %q: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".upload-*")
	if err != nil {
		return fmt.Errorf("userprofile: open tempfile: %w", err)
	}
	tmpPath := tmp.Name()
	// Best-effort cleanup if anything below fails before rename.
	removeTemp := true
	defer func() {
		if removeTemp {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("userprofile: write tempfile: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("userprofile: close tempfile: %w", err)
	}
	if err := os.Rename(tmpPath, s.pathFor(orgID, userID, assetID, ext)); err != nil {
		return fmt.Errorf("userprofile: rename: %w", err)
	}
	removeTemp = false
	return nil
}

func (s *perOrgUserFileStore) Open(orgID, userID, assetID, ext string) ([]byte, error) {
	f, err := os.Open(s.pathFor(orgID, userID, assetID, ext))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrAssetNotFound
		}
		return nil, fmt.Errorf("userprofile: open: %w", err)
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("userprofile: read: %w", err)
	}
	return data, nil
}

func (s *perOrgUserFileStore) Delete(orgID, userID, assetID, ext string) error {
	if err := os.Remove(s.pathFor(orgID, userID, assetID, ext)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("userprofile: delete: %w", err)
	}
	return nil
}

// -- FileRepo ------------------------------------------------------

// NewOrgFileRepo wraps the per-org DB registry. Every call opens (or
// reuses the cached) profiles.db for the target org and runs a
// single-statement SQL against user_profile_files. asset_id is
// generated here via 24-byte crypto/rand → 32-char base64url so the
// opaque-key invariant stays in one place.
func NewOrgFileRepo(reg *profile_dbregistry.OrgDBRegistry) FileRepo {
	return &orgFileRepo{reg: reg}
}

type orgFileRepo struct {
	reg *profile_dbregistry.OrgDBRegistry
}

func newAssetID() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func (r *orgFileRepo) Insert(orgID string, meta FileMeta) (string, error) {
	db, err := r.reg.For(orgID)
	if err != nil {
		return "", err
	}
	assetID := meta.AssetID
	if assetID == "" {
		assetID, err = newAssetID()
		if err != nil {
			return "", fmt.Errorf("userprofile: generate asset id: %w", err)
		}
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = db.Connector.DB.Exec(
		`INSERT INTO user_profile_files
		 (asset_id, user_id, field_name, filename, mime_type, size_bytes, ext, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		assetID, meta.UserID, meta.FieldName, meta.Filename, meta.MimeType, meta.SizeBytes, meta.Ext, now,
	)
	if err != nil {
		return "", fmt.Errorf("userprofile: insert file meta: %w", err)
	}
	return assetID, nil
}

func (r *orgFileRepo) FindByAssetID(orgID, userID, assetID string) (*FileMeta, error) {
	db, err := r.reg.For(orgID)
	if err != nil {
		return nil, err
	}
	return scanOne(db.Connector.DB.QueryRow(
		`SELECT asset_id, user_id, field_name, filename, mime_type, size_bytes, ext, created_at
		 FROM user_profile_files
		 WHERE asset_id = ? AND user_id = ? LIMIT 1`,
		assetID, userID,
	))
}

func (r *orgFileRepo) FindByField(orgID, userID, fieldName string) (*FileMeta, error) {
	db, err := r.reg.For(orgID)
	if err != nil {
		return nil, err
	}
	return scanOne(db.Connector.DB.QueryRow(
		`SELECT asset_id, user_id, field_name, filename, mime_type, size_bytes, ext, created_at
		 FROM user_profile_files
		 WHERE user_id = ? AND field_name = ?
		 ORDER BY created_at DESC
		 LIMIT 1`,
		userID, fieldName,
	))
}

func (r *orgFileRepo) DeleteByAssetID(orgID, userID, assetID string) error {
	db, err := r.reg.For(orgID)
	if err != nil {
		return err
	}
	res, err := db.Connector.DB.Exec(
		`DELETE FROM user_profile_files WHERE asset_id = ? AND user_id = ?`,
		assetID, userID,
	)
	if err != nil {
		return fmt.Errorf("userprofile: delete file meta: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrAssetNotFound
	}
	return nil
}

func scanOne(row *sql.Row) (*FileMeta, error) {
	var m FileMeta
	if err := row.Scan(
		&m.AssetID, &m.UserID, &m.FieldName, &m.Filename, &m.MimeType, &m.SizeBytes, &m.Ext, &m.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAssetNotFound
		}
		return nil, err
	}
	return &m, nil
}

// -- VirusScanner -------------------------------------------------

// NewClamdVirusScanner returns a VirusScanner backed by a running clamd
// daemon at the given socket path (e.g. "/var/run/clamav/clamd.ctl").
// Bytes are streamed via INSTREAM so they never touch the filesystem.
func NewClamdVirusScanner(socketPath string) VirusScanner {
	return &clamdVirusScanner{socketPath: socketPath}
}

type clamdVirusScanner struct {
	socketPath string
}

func (s *clamdVirusScanner) Scan(data []byte) error {
	c := clamd.NewClamd(s.socketPath)
	results, err := c.ScanStream(bytes.NewReader(data), make(chan bool))
	if err != nil {
		return fmt.Errorf("userprofile: clamd unavailable: %w", err)
	}
	for result := range results {
		if result.Status == clamd.RES_FOUND {
			return fmt.Errorf("%w: %s", ErrVirusFound, result.Description)
		}
	}
	return nil
}

// -- FieldConfigReader --------------------------------------------

// NewOrgFieldConfigReader resolves a single ProfileField by name in
// the per-org profiles.db. Only the columns the upload handler
// actually reads are pulled — accept_mime, max_bytes, data_type,
// is_active — plus the name + id so the response can round-trip.
func NewOrgFieldConfigReader(reg *profile_dbregistry.OrgDBRegistry) FieldConfigReader {
	return &orgFieldConfigReader{reg: reg}
}

type orgFieldConfigReader struct {
	reg *profile_dbregistry.OrgDBRegistry
}

func (r *orgFieldConfigReader) FieldByName(orgID, fieldName string) (*profile_entity.ProfileField, error) {
	db, err := r.reg.For(orgID)
	if err != nil {
		return nil, err
	}
	var f profile_entity.ProfileField
	var optionsJSON string
	var minVal, maxVal sql.NullInt64
	err = db.Connector.DB.QueryRow(
		`SELECT id, name, label, description, data_type, is_required,
		        min_value, max_value, options_json, regex,
		        accept_mime, max_bytes, order_index,
		        is_active, created_at, updated_at
		 FROM profile_fields
		 WHERE name = ? LIMIT 1`,
		fieldName,
	).Scan(
		&f.ID, &f.Name, &f.Label, &f.Description, &f.DataType, &f.IsRequired,
		&minVal, &maxVal, &optionsJSON, &f.Regex,
		&f.AcceptMime, &f.MaxBytes, &f.OrderIndex,
		&f.IsActive, &f.CreatedAt, &f.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrFieldNotFound
		}
		return nil, err
	}
	if minVal.Valid {
		v := int(minVal.Int64)
		f.MinValue = &v
	}
	if maxVal.Valid {
		v := int(maxVal.Int64)
		f.MaxValue = &v
	}
	if optionsJSON != "" {
		var opts []string
		_ = json.Unmarshal([]byte(optionsJSON), &opts)
		f.Options = opts
	}
	return &f, nil
}

// -- ProfileWriter -------------------------------------------------

// NewOrgProfileWriter wraps the per-org DB registry as a
// ProfileWriter. UpdateFieldValue reads the current profile_data,
// overlays the single (fieldName, value) change — with nil meaning
// "remove the key" — and upserts the row. Concurrent writers from
// the same user race on last-writer-wins, which matches the
// PATCH-endpoint contract.
func NewOrgProfileWriter(reg *profile_dbregistry.OrgDBRegistry) ProfileWriter {
	return &orgProfileWriter{reg: reg}
}

type orgProfileWriter struct {
	reg *profile_dbregistry.OrgDBRegistry
}

func (w *orgProfileWriter) UpdateFieldValue(orgID, userID, fieldName string, value any) error {
	db, err := w.reg.For(orgID)
	if err != nil {
		return err
	}
	conn := db.Connector.DB

	var dataJSON string
	err = conn.QueryRow(
		`SELECT profile_data FROM user_profiles WHERE user_id = ? LIMIT 1`, userID,
	).Scan(&dataJSON)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	data := map[string]interface{}{}
	if dataJSON != "" {
		if jerr := json.Unmarshal([]byte(dataJSON), &data); jerr != nil {
			data = map[string]interface{}{}
		}
	}
	if value == nil {
		delete(data, fieldName)
	} else {
		data[fieldName] = value
	}
	blob, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("userprofile: marshal profile_data: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = conn.Exec(
		`INSERT INTO user_profiles (user_id, profile_data, updated_at)
		 VALUES (?, ?, ?)
		 ON CONFLICT(user_id) DO UPDATE SET
		     profile_data = excluded.profile_data,
		     updated_at   = excluded.updated_at`,
		userID, string(blob), now,
	)
	if err != nil {
		return fmt.Errorf("userprofile: upsert user_profiles: %w", err)
	}
	return nil
}
