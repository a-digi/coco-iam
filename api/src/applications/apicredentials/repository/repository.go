// Package repository is the thin CRUD layer over the
// `application_api_credentials` table in the per-org
// api_credentials.db. No DI lookups — the caller passes a
// *sql.DB resolved from the OrgApiCredentialsDBRegistry.
package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/a-digi/coco-iam/src/applications/apicredentials/entity"
)

// ErrNotFound signals no row matched the lookup. Handlers should
// collapse this with every other auth failure into a single generic
// 401 so the endpoint can't be used as an oracle.
var ErrNotFound = errors.New("api-credentials: not found")

// Repository owns the SQL against one per-organization
// api_credentials.db. Construct with New(db).
type Repository struct {
	db *sql.DB
}

// New wraps an opened *sql.DB for one org's api_credentials.db.
func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Insert persists a new credential row. `purposes` is marshalled to
// JSON before storage so the DB always sees a valid `[]string` shape.
func (r *Repository) Insert(c entity.Credential, purposes []string) error {
	if purposes == nil {
		purposes = []string{}
	}
	purposesJSON, err := json.Marshal(purposes)
	if err != nil {
		return fmt.Errorf("api-credentials: marshal purposes: %w", err)
	}
	_, err = r.db.Exec(
		`INSERT INTO application_api_credentials
		 (id, application_id, api_id, secret_hash, label, purposes, expires_at, is_active, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		c.ID, c.ApplicationID, c.APIID, c.SecretHash, c.Label,
		string(purposesJSON),
		c.ExpiresAt.UTC().Format("2006-01-02 15:04:05"),
		c.IsActive,
	)
	if err != nil {
		return fmt.Errorf("api-credentials: insert: %w", err)
	}
	return nil
}

// FindByAPIID returns the row identified by the opaque public
// `api_id`. Missing rows return ErrNotFound. Purposes are unmarshalled
// into a slice so handlers can check permission without re-parsing.
func (r *Repository) FindByAPIID(apiID string) (*entity.Credential, []string, error) {
	row := r.db.QueryRow(
		`SELECT id, application_id, api_id, secret_hash, label, purposes, expires_at, is_active, last_used_at, created_at, revoked_at
		 FROM application_api_credentials WHERE api_id = ?`, apiID,
	)
	return scanRow(row)
}

// ListForApplication returns every row for the given application id,
// newest-first. Purposes are unmarshalled per-row. Used by the admin
// list endpoint.
func (r *Repository) ListForApplication(applicationID string) ([]entity.Credential, [][]string, error) {
	rows, err := r.db.Query(
		`SELECT id, application_id, api_id, secret_hash, label, purposes, expires_at, is_active, last_used_at, created_at, revoked_at
		 FROM application_api_credentials
		 WHERE application_id = ?
		 ORDER BY created_at DESC`,
		applicationID,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("api-credentials: list: %w", err)
	}
	defer rows.Close()

	var creds []entity.Credential
	var purposes [][]string
	for rows.Next() {
		c, p, scanErr := scanRow(rows)
		if scanErr != nil {
			if errors.Is(scanErr, ErrNotFound) {
				continue
			}
			return nil, nil, scanErr
		}
		creds = append(creds, *c)
		purposes = append(purposes, p)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("api-credentials: list iteration: %w", err)
	}
	return creds, purposes, nil
}

// Revoke soft-revokes a credential: sets `is_active = 0` and stamps
// `revoked_at`. The row stays in the table so admins can audit who
// held what. A caller restricted to one org can't revoke another
// org's credential — caller resolved the row via its own per-org DB.
func (r *Repository) Revoke(id string) error {
	res, err := r.db.Exec(
		`UPDATE application_api_credentials
		 SET is_active = 0, revoked_at = CURRENT_TIMESTAMP
		 WHERE id = ? AND is_active = 1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("api-credentials: revoke: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("api-credentials: revoke rows-affected: %w", err)
	}
	if n == 0 {
		// Either the id is wrong or the row is already revoked;
		// either way the caller saw nothing to do. ErrNotFound is the
		// cleanest signal.
		return ErrNotFound
	}
	return nil
}

// TouchLastUsed stamps last_used_at = now() for the given credential.
// Called on every successful auth. Failures are non-fatal at the
// callsite — auth already succeeded, a missed stamp is observability
// only.
func (r *Repository) TouchLastUsed(id string) error {
	_, err := r.db.Exec(
		`UPDATE application_api_credentials SET last_used_at = CURRENT_TIMESTAMP WHERE id = ?`,
		id,
	)
	if err != nil {
		return fmt.Errorf("api-credentials: touch: %w", err)
	}
	return nil
}

// scanner is the narrow interface both *sql.Row and *sql.Rows
// satisfy, letting scanRow serve both single-row lookups and
// iterated list queries.
type scanner interface {
	Scan(dest ...interface{}) error
}

func scanRow(s scanner) (*entity.Credential, []string, error) {
	var c entity.Credential
	var purposesJSON, expiresAt string
	var lastUsed, createdAt, revokedAt sql.NullString
	err := s.Scan(
		&c.ID, &c.ApplicationID, &c.APIID, &c.SecretHash, &c.Label,
		&purposesJSON, &expiresAt, &c.IsActive,
		&lastUsed, &createdAt, &revokedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, ErrNotFound
		}
		return nil, nil, fmt.Errorf("api-credentials: scan: %w", err)
	}

	if t, perr := parseTime(expiresAt); perr == nil {
		c.ExpiresAt = t
	}
	if lastUsed.Valid {
		if t, perr := parseTime(lastUsed.String); perr == nil {
			c.LastUsedAt = &t
		}
	}
	if createdAt.Valid {
		if t, perr := parseTime(createdAt.String); perr == nil {
			c.CreatedAt = t
		}
	}
	if revokedAt.Valid {
		if t, perr := parseTime(revokedAt.String); perr == nil {
			c.RevokedAt = &t
		}
	}

	var purposes []string
	if purposesJSON != "" {
		if jerr := json.Unmarshal([]byte(purposesJSON), &purposes); jerr != nil {
			// Malformed JSON is a data-integrity problem; surface it
			// so the caller doesn't silently treat a broken row as
			// "no purposes" and let an auth check pass.
			return nil, nil, fmt.Errorf("api-credentials: unmarshal purposes: %w", jerr)
		}
	}
	if purposes == nil {
		purposes = []string{}
	}
	return &c, purposes, nil
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
	return time.Time{}, fmt.Errorf("api-credentials: unparseable timestamp %q", s)
}
