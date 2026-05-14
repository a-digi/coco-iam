// Package repository is the thin CRUD layer over admin_user_profiles
// in the main DB. No DI — callers pass a *sql.DB.
package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/a-digi/coco-iam/src/admin/users/profile/entity"
)

// ErrNotFound signals no row matched the lookup. Caller usually
// treats this as "lazy-create an empty profile" rather than an
// error — the GET /me handler returns empty strings in that case
// so the client sees a consistent shape.
var ErrNotFound = errors.New("admin-user-profile: not found")

// Repository wraps a *sql.DB. Constructed per-request from the
// shared main-DB manager.
type Repository struct {
	db *sql.DB
}

// New wraps an opened *sql.DB.
func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// FindByAdminUserID returns the profile for the given admin user.
// Returns ErrNotFound when no row exists yet — handlers decide
// whether to lazy-create or return an empty projection.
func (r *Repository) FindByAdminUserID(adminUserID string) (*entity.AdminUserProfile, error) {
	row := r.db.QueryRow(
		`SELECT admin_user_id, first_name, last_name, phone, avatar_asset_id, locale, timezone, created_at, updated_at
		 FROM admin_user_profiles WHERE admin_user_id = ?`,
		adminUserID,
	)
	return scanRow(row)
}

// Upsert inserts a new profile row or replaces an existing one,
// stamping updated_at on every call. We use INSERT OR REPLACE so
// a PATCH that arrives before an initial profile row exists still
// succeeds in a single query — PUT semantics, not merge semantics.
// The handler is responsible for merging incoming patches onto an
// existing row before calling Upsert when merge semantics are
// needed.
func (r *Repository) Upsert(p *entity.AdminUserProfile) error {
	if p == nil || p.AdminUserID == "" {
		return errors.New("admin-user-profile: admin_user_id is required")
	}
	_, err := r.db.Exec(
		`INSERT INTO admin_user_profiles
		  (admin_user_id, first_name, last_name, phone, avatar_asset_id, locale, timezone, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, COALESCE(
		     (SELECT created_at FROM admin_user_profiles WHERE admin_user_id = ?),
		     CURRENT_TIMESTAMP
		 ), CURRENT_TIMESTAMP)
		 ON CONFLICT(admin_user_id) DO UPDATE SET
		   first_name = excluded.first_name,
		   last_name = excluded.last_name,
		   phone = excluded.phone,
		   avatar_asset_id = excluded.avatar_asset_id,
		   locale = excluded.locale,
		   timezone = excluded.timezone,
		   updated_at = CURRENT_TIMESTAMP`,
		p.AdminUserID, p.FirstName, p.LastName, p.Phone, p.AvatarAssetID, p.Locale, p.Timezone,
		p.AdminUserID,
	)
	if err != nil {
		return fmt.Errorf("admin-user-profile: upsert: %w", err)
	}
	return nil
}

// UpdateAvatarAssetID sets only the avatar_asset_id + updated_at.
// Separate from Upsert because the avatar upload flow knows
// nothing about the other fields and we don't want to stomp them
// (or load them first just to write them back).
func (r *Repository) UpdateAvatarAssetID(adminUserID, assetID string) error {
	if adminUserID == "" {
		return errors.New("admin-user-profile: admin_user_id is required")
	}
	// INSERT OR UPDATE: if no profile row exists yet, create one
	// with all other fields blank and the avatar populated so the
	// 1:1 invariant is maintained even when the admin uploads an
	// avatar as their very first action.
	_, err := r.db.Exec(
		`INSERT INTO admin_user_profiles (admin_user_id, avatar_asset_id, created_at, updated_at)
		 VALUES (?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		 ON CONFLICT(admin_user_id) DO UPDATE SET
		   avatar_asset_id = excluded.avatar_asset_id,
		   updated_at = CURRENT_TIMESTAMP`,
		adminUserID, assetID,
	)
	if err != nil {
		return fmt.Errorf("admin-user-profile: update avatar: %w", err)
	}
	return nil
}

// ClearAvatar sets avatar_asset_id back to the empty string. Caller
// is responsible for deleting the on-disk file separately — the
// repository owns only the DB state.
func (r *Repository) ClearAvatar(adminUserID string) error {
	return r.UpdateAvatarAssetID(adminUserID, "")
}

// -- helpers ----------------------------------------------------------

type scanner interface {
	Scan(dest ...interface{}) error
}

func scanRow(s scanner) (*entity.AdminUserProfile, error) {
	var p entity.AdminUserProfile
	var created, updated sql.NullString
	err := s.Scan(
		&p.AdminUserID, &p.FirstName, &p.LastName, &p.Phone,
		&p.AvatarAssetID, &p.Locale, &p.Timezone,
		&created, &updated,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("admin-user-profile: scan: %w", err)
	}
	if created.Valid {
		if t, perr := parseTime(created.String); perr == nil {
			p.CreatedAt = t
		}
	}
	if updated.Valid {
		if t, perr := parseTime(updated.String); perr == nil {
			p.UpdatedAt = t
		}
	}
	return &p, nil
}

func parseTime(s string) (time.Time, error) {
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("admin-user-profile: unparseable timestamp %q", s)
}
