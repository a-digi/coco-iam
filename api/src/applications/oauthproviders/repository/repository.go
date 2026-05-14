// Package repository wraps the application_oauth_providers table.
// Every read returns the entity.ProviderConfig with the client secret
// already decrypted via the secretbox helper, so the callers
// (handlers, adapter layer) never see ciphertext.
//
// The table now lives in the per-org DB. Callers that know the DB
// upfront use New; callers that receive an appID at request time
// use NewWithResolver so the repository resolves the right DB itself.
package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/a-digi/coco-iam/src/applications/oauthproviders/entity"
	"github.com/a-digi/coco-iam/src/auth/crypto/secretbox"
	"github.com/google/uuid"
)

// DBResolver returns the per-org DB for a given application id.
type DBResolver func(appID string) (*sql.DB, error)

// Repository is the query + persistent facade. Read-only and
// write paths share the same struct because the table is small
// and a split would just double the boilerplate.
type Repository struct {
	db      *sql.DB
	resolve DBResolver
}

// New constructs a Repository over the given *sql.DB directly.
// Use this when the caller already holds the correct per-org DB.
func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// NewWithResolver constructs a Repository that resolves the per-org
// DB by application id at query time. Use this for handlers that
// only know the application id at request time (e.g. public OAuth
// authorize/callback handlers).
func NewWithResolver(resolve DBResolver) *Repository {
	return &Repository{resolve: resolve}
}

// dbFor returns the DB to use for the given application id.
// When db is set directly it is always returned. Otherwise the
// resolver is called.
func (r *Repository) dbFor(appID string) (*sql.DB, error) {
	if r.db != nil {
		return r.db, nil
	}
	if r.resolve == nil {
		return nil, fmt.Errorf("oauthproviders: no db or resolver configured")
	}
	return r.resolve(appID)
}

// ListForApp returns every configured provider for the given
// application id, ordered by created_at. Secrets are decrypted
// before return.
func (r *Repository) ListForApp(applicationID string) ([]entity.ProviderConfig, error) {
	db, err := r.dbFor(applicationID)
	if err != nil {
		return nil, fmt.Errorf("oauthproviders: list: resolve db: %w", err)
	}
	rows, err := db.Query(
		`SELECT id, application_id, provider, client_id, client_secret_enc,
		        discovery_url, authorize_url, token_url, userinfo_url, scopes,
		        allow_login, allow_registration, is_active, created_at, updated_at
		 FROM application_oauth_providers
		 WHERE application_id = ?
		 ORDER BY created_at ASC`,
		applicationID,
	)
	if err != nil {
		return nil, fmt.Errorf("oauthproviders: list: %w", err)
	}
	defer rows.Close()

	out := []entity.ProviderConfig{}
	for rows.Next() {
		cfg, err := scanRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, cfg)
	}
	return out, rows.Err()
}

// FindByID returns one row by its primary key, scoped to
// application id so a leaked id from a different app can't read
// another app's provider config. Returns ErrProviderNotFound
// when missing.
func (r *Repository) FindByID(applicationID, id string) (*entity.ProviderConfig, error) {
	db, err := r.dbFor(applicationID)
	if err != nil {
		return nil, fmt.Errorf("oauthproviders: find by id: resolve db: %w", err)
	}
	rows, err := db.Query(
		`SELECT id, application_id, provider, client_id, client_secret_enc,
		        discovery_url, authorize_url, token_url, userinfo_url, scopes,
		        allow_login, allow_registration, is_active, created_at, updated_at
		 FROM application_oauth_providers
		 WHERE id = ? AND application_id = ?
		 LIMIT 1`,
		id, applicationID,
	)
	if err != nil {
		return nil, fmt.Errorf("oauthproviders: find by id: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, entity.ErrProviderNotFound
	}
	cfg, err := scanRow(rows)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// FindByProvider resolves (application, provider) → row. Used by
// the authorize / callback handlers at request time.
func (r *Repository) FindByProvider(applicationID string, provider entity.Provider) (*entity.ProviderConfig, error) {
	db, err := r.dbFor(applicationID)
	if err != nil {
		return nil, fmt.Errorf("oauthproviders: find by provider: resolve db: %w", err)
	}
	rows, err := db.Query(
		`SELECT id, application_id, provider, client_id, client_secret_enc,
		        discovery_url, authorize_url, token_url, userinfo_url, scopes,
		        allow_login, allow_registration, is_active, created_at, updated_at
		 FROM application_oauth_providers
		 WHERE application_id = ? AND provider = ?
		 LIMIT 1`,
		applicationID, string(provider),
	)
	if err != nil {
		return nil, fmt.Errorf("oauthproviders: find by provider: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, entity.ErrProviderNotFound
	}
	cfg, err := scanRow(rows)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// InsertInput carries the plaintext fields a handler has
// validated. The repository encrypts the client secret before
// persisting. UUID for `id` is minted here so callers don't
// import google/uuid themselves.
type InsertInput struct {
	ApplicationID     string
	Provider          entity.Provider
	ClientID          string
	ClientSecret      string
	DiscoveryURL      string
	AuthorizeURL      string
	TokenURL          string
	UserinfoURL       string
	Scopes            []string
	AllowLogin        bool
	AllowRegistration bool
}

// Insert persists a new provider. Returns ErrDuplicateProvider
// when the (application_id, provider) pair is already configured.
func (r *Repository) Insert(in InsertInput) (entity.ProviderConfig, error) {
	db, err := r.dbFor(in.ApplicationID)
	if err != nil {
		return entity.ProviderConfig{}, fmt.Errorf("oauthproviders: insert: resolve db: %w", err)
	}
	enc, err := secretbox.Encrypt(in.ClientSecret)
	if err != nil {
		return entity.ProviderConfig{}, fmt.Errorf("oauthproviders: encrypt secret: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	id := uuid.New().String()
	_, err = db.Exec(
		`INSERT INTO application_oauth_providers
		 (id, application_id, provider, client_id, client_secret_enc,
		  discovery_url, authorize_url, token_url, userinfo_url, scopes,
		  allow_login, allow_registration, is_active, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?)`,
		id, in.ApplicationID, string(in.Provider), in.ClientID, enc,
		in.DiscoveryURL, in.AuthorizeURL, in.TokenURL, in.UserinfoURL,
		strings.Join(in.Scopes, " "),
		boolToInt(in.AllowLogin), boolToInt(in.AllowRegistration),
		now, now,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return entity.ProviderConfig{}, entity.ErrDuplicateProvider
		}
		return entity.ProviderConfig{}, fmt.Errorf("oauthproviders: insert: %w", err)
	}
	return entity.ProviderConfig{
		ID:                id,
		ApplicationID:     in.ApplicationID,
		Provider:          in.Provider,
		ClientID:          in.ClientID,
		ClientSecret:      in.ClientSecret,
		DiscoveryURL:      in.DiscoveryURL,
		AuthorizeURL:      in.AuthorizeURL,
		TokenURL:          in.TokenURL,
		UserinfoURL:       in.UserinfoURL,
		Scopes:            in.Scopes,
		AllowLogin:        in.AllowLogin,
		AllowRegistration: in.AllowRegistration,
		IsActive:          true,
		CreatedAt:         now,
		UpdatedAt:         now,
	}, nil
}

// UpdateInput carries the mutable fields of a row. A nil
// ClientSecret pointer leaves the stored secret untouched — the
// admin UI edits other fields without requiring a secret
// re-entry.
type UpdateInput struct {
	ClientID          string
	ClientSecret      *string
	DiscoveryURL      string
	AuthorizeURL      string
	TokenURL          string
	UserinfoURL       string
	Scopes            []string
	AllowLogin        bool
	AllowRegistration bool
	IsActive          bool
}

// Update applies the UpdateInput to the identified row, scoped
// by application id. Returns ErrProviderNotFound when no row
// matches.
func (r *Repository) Update(applicationID, id string, in UpdateInput) (entity.ProviderConfig, error) {
	db, err := r.dbFor(applicationID)
	if err != nil {
		return entity.ProviderConfig{}, fmt.Errorf("oauthproviders: update: resolve db: %w", err)
	}
	existing, err := r.FindByID(applicationID, id)
	if err != nil {
		return entity.ProviderConfig{}, err
	}
	secretEncUpdate := ""
	if in.ClientSecret != nil {
		enc, err := secretbox.Encrypt(*in.ClientSecret)
		if err != nil {
			return entity.ProviderConfig{}, fmt.Errorf("oauthproviders: encrypt secret: %w", err)
		}
		secretEncUpdate = enc
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if secretEncUpdate != "" {
		_, err = db.Exec(
			`UPDATE application_oauth_providers SET
			    client_id = ?, client_secret_enc = ?,
			    discovery_url = ?, authorize_url = ?, token_url = ?, userinfo_url = ?,
			    scopes = ?, allow_login = ?, allow_registration = ?, is_active = ?,
			    updated_at = ?
			 WHERE id = ? AND application_id = ?`,
			in.ClientID, secretEncUpdate,
			in.DiscoveryURL, in.AuthorizeURL, in.TokenURL, in.UserinfoURL,
			strings.Join(in.Scopes, " "),
			boolToInt(in.AllowLogin), boolToInt(in.AllowRegistration), boolToInt(in.IsActive),
			now,
			id, applicationID,
		)
	} else {
		_, err = db.Exec(
			`UPDATE application_oauth_providers SET
			    client_id = ?,
			    discovery_url = ?, authorize_url = ?, token_url = ?, userinfo_url = ?,
			    scopes = ?, allow_login = ?, allow_registration = ?, is_active = ?,
			    updated_at = ?
			 WHERE id = ? AND application_id = ?`,
			in.ClientID,
			in.DiscoveryURL, in.AuthorizeURL, in.TokenURL, in.UserinfoURL,
			strings.Join(in.Scopes, " "),
			boolToInt(in.AllowLogin), boolToInt(in.AllowRegistration), boolToInt(in.IsActive),
			now,
			id, applicationID,
		)
	}
	if err != nil {
		return entity.ProviderConfig{}, fmt.Errorf("oauthproviders: update: %w", err)
	}
	// Return the merged view. Secret stays unchanged when pointer was nil.
	updated := *existing
	updated.ClientID = in.ClientID
	if in.ClientSecret != nil {
		updated.ClientSecret = *in.ClientSecret
	}
	updated.DiscoveryURL = in.DiscoveryURL
	updated.AuthorizeURL = in.AuthorizeURL
	updated.TokenURL = in.TokenURL
	updated.UserinfoURL = in.UserinfoURL
	updated.Scopes = in.Scopes
	updated.AllowLogin = in.AllowLogin
	updated.AllowRegistration = in.AllowRegistration
	updated.IsActive = in.IsActive
	updated.UpdatedAt = now
	return updated, nil
}

// Delete removes the provider row, scoped by application id. No
// error is returned when the row is already absent — delete is
// idempotent.
func (r *Repository) Delete(applicationID, id string) error {
	db, err := r.dbFor(applicationID)
	if err != nil {
		return fmt.Errorf("oauthproviders: delete: resolve db: %w", err)
	}
	_, err = db.Exec(
		`DELETE FROM application_oauth_providers
		 WHERE id = ? AND application_id = ?`,
		id, applicationID,
	)
	if err != nil {
		return fmt.Errorf("oauthproviders: delete: %w", err)
	}
	return nil
}

// -- helpers -------------------------------------------------------

// scanRow consumes one row from either QueryRow or a rows cursor
// after rows.Next() has returned true.
func scanRow(src interface{ Scan(dest ...interface{}) error }) (entity.ProviderConfig, error) {
	var cfg entity.ProviderConfig
	var scopesRaw string
	var providerStr string
	var allowLogin, allowRegistration, isActive int
	var secretEnc string
	if err := src.Scan(
		&cfg.ID, &cfg.ApplicationID, &providerStr, &cfg.ClientID, &secretEnc,
		&cfg.DiscoveryURL, &cfg.AuthorizeURL, &cfg.TokenURL, &cfg.UserinfoURL, &scopesRaw,
		&allowLogin, &allowRegistration, &isActive, &cfg.CreatedAt, &cfg.UpdatedAt,
	); err != nil {
		return entity.ProviderConfig{}, fmt.Errorf("oauthproviders: scan: %w", err)
	}
	plaintext, err := secretbox.Decrypt(secretEnc)
	if err != nil {
		return entity.ProviderConfig{}, fmt.Errorf("oauthproviders: decrypt secret (row %s): %w", cfg.ID, err)
	}
	cfg.Provider = entity.Provider(providerStr)
	cfg.ClientSecret = plaintext
	if scopesRaw != "" {
		cfg.Scopes = strings.Fields(scopesRaw)
	} else {
		cfg.Scopes = []string{}
	}
	cfg.AllowLogin = allowLogin != 0
	cfg.AllowRegistration = allowRegistration != 0
	cfg.IsActive = isActive != 0
	return cfg, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// isUniqueViolation recognises SQLite's unique-index error so
// Insert can report ErrDuplicateProvider rather than leak the
// raw SQL text.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToUpper(err.Error())
	return strings.Contains(msg, "UNIQUE") ||
		strings.Contains(msg, "CONSTRAINT FAILED")
}

// compile-time check the sentinel doesn't drift from errors.Is usage.
var _ = errors.Is
