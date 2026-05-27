package userprofile

import (
	"crypto/rsa"
	"errors"

	"github.com/a-digi/coco-iam/src/organizations/profile/entity"
)

// This file declares the narrow interfaces (ports) the handlers
// depend on. Each interface exposes the minimum set of methods the
// handlers need — so production wiring and test fakes both stay
// small. Depending on these interfaces rather than the concrete
// service types keeps the handlers honest about what they actually
// use (ISP) and lets tests substitute trivial fakes without
// reconstructing the full DI bag (DIP).

// SlugResolver turns (orgSlug, wsSlug, appSlug) into the
// application UUID + its parent organisation UUID. Implemented in
// production by a closure over
// `loginpage.Store.FindBySlugs`. Returning a plain error for any
// miss keeps the interface narrow; the handler collapses that
// into a generic 401.
type SlugResolver interface {
	ResolveSlugs(orgSlug, wsSlug, appSlug string) (appID, orgID string, err error)
}

// KeyLoader returns the RSA public key identified by a
// `(appID, kid)` pair. Implemented in production by a closure
// over `keys.Service.LoadVerifiablePublicKey(appID, kid)` — which
// already encodes the "key still verifiable" rule (active, or
// deactivated but not expired). A token signed by a different
// app's key makes this return an error and the handler 401s,
// which is the cross-app rejection path.
type KeyLoader interface {
	LoadPublicKey(appID, kid string) (*rsa.PublicKey, error)
}

// UserOrgReader returns the organisation id the given user
// belongs to, or ErrUserNotFound when no such user exists. In
// production this is a one-row query on the main DB's `users`
// table. Tests use a map-backed fake.
type UserOrgReader interface {
	UserOrg(userID string) (orgID string, err error)
}

// ProfileReader returns the active profile fields for an
// organisation plus the given user's stored values. In production
// this closes over the per-org profiles.db resolved from the
// org-DB registry. Missing profile row → empty map (not an
// error); the shaping layer renders each field with
// `value: null` in that case.
type ProfileReader interface {
	LoadProfile(orgID, userID string) (fields []entity.ProfileField, data map[string]interface{}, err error)
}

// FileMeta is the row persisted in user_profile_files inside the
// per-org profiles.db. AssetID is the opaque 32-char base64url key
// the handler stores into profile_data.<field_name>. Filename,
// MimeType, SizeBytes and Ext are captured at upload time from the
// media subsystem's scanner + the original multipart header.
type FileMeta struct {
	AssetID   string
	UserID    string
	FieldName string
	Filename  string
	MimeType  string
	SizeBytes int64
	Ext       string
	CreatedAt string
}

// Scanner runs the media subsystem's magic-byte validator over the
// first 512 bytes of an upload. The production adapter forwards to
// `media.DetectAndValidateMime`; the interface exists so handler
// tests can substitute a fake without wiring up the full media
// service.
type Scanner interface {
	DetectAndValidate(head []byte, claimedMime string) (mime, ext string, err error)
}

// FileStore owns the bytes on disk under the per-org, per-user
// path:
//
//	./data/db/organization/<orgID>/uploads/users/<userID>/<assetID>.<ext>
//
// Writes are atomic (tempfile + rename) so a crash never leaves
// half-written bytes on disk. Narrow by design — no listing, no
// folders, no cross-user operations — because the file handlers
// only need these three verbs.
type FileStore interface {
	Save(orgID, userID, assetID, ext string, data []byte) error
	Open(orgID, userID, assetID, ext string) ([]byte, error)
	Delete(orgID, userID, assetID, ext string) error
}

// FileRepo owns the user_profile_files rows in the per-org
// profiles.db. Insert mints the asset_id (the handler never picks
// it) so the opaque-key invariant lives in one place. Lookups are
// always scoped by userID so a leaked asset_id can't be played
// back against a different user's files.
type FileRepo interface {
	Insert(orgID string, meta FileMeta) (assetID string, err error)
	FindByAssetID(orgID, userID, assetID string) (*FileMeta, error)
	FindByField(orgID, userID, fieldName string) (*FileMeta, error)
	DeleteByAssetID(orgID, userID, assetID string) error
}

// FieldConfigReader returns a single field definition by name for
// a given org. Used by the upload handler to read the per-field
// accept_mime / max_bytes overrides before delegating to the
// scanner + store. Deliberately narrower than the full
// organizations/profile repository.
type FieldConfigReader interface {
	FieldByName(orgID, fieldName string) (*entity.ProfileField, error)
}

// ProfileWriter applies a "set one field to v" change to the user's
// profile_data map (upserts the row). The PATCH handler calls this
// once per validated patch; the file upload / delete handlers call
// it once with the new asset_id / nil.
type ProfileWriter interface {
	UpdateFieldValue(orgID, userID, fieldName string, value any) error
}

// VirusScanner scans raw file bytes for malware. A nil VirusScanner on a
// handler means scanning is disabled. ErrVirusFound is returned when the
// bytes match a known signature. Any other non-nil error means the scanner
// daemon is unreachable — callers must treat that as fail-closed (503).
type VirusScanner interface {
	Scan(data []byte) error
}

// ErrVirusFound is the sentinel VirusScanner.Scan returns when bytes match
// a known malware signature. The handler maps it to 422; any other error
// from Scan becomes 503 (scanner unavailable).
var ErrVirusFound = errors.New("file rejected: malware detected")

// ErrUserNotFound is the sentinel UserOrgReader returns when the
// user id doesn't exist. The handler maps this to
// ReasonUnknownUser / 401 specifically — any other error is
// treated as an internal failure and 500s. Keeping the sentinel
// in the port file (not the adapter) lets tests import it
// without pulling in production wiring.
var ErrUserNotFound = errors.New("userprofile: user not found")

// ErrAssetNotFound is the sentinel FileRepo returns when
// FindByAssetID / FindByField / DeleteByAssetID finds no row.
// The serve / delete handlers map this to 404.
var ErrAssetNotFound = errors.New("userprofile: asset not found")

// ErrFieldNotFound is the sentinel FieldConfigReader returns when
// the field name doesn't resolve. Handler maps to 400.
var ErrFieldNotFound = errors.New("userprofile: field not found")
