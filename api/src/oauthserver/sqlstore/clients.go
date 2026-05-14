// Package sqlstore hosts the reference SQL implementations of
// the oauthserver ports. They sit alongside the pure library so
// a consumer that uses a generic SQL DB can drop them in; a
// consumer with a non-SQL store provides their own adapters.
//
// All implementations take a plain *sql.DB — no coco-iam-
// specific types, keeping the "library-shape" contract intact.
package sqlstore

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/a-digi/coco-iam/src/oauthserver/entity"
)

// AppDBResolver returns the per-org DB for a given application id.
// Used by NewClientRepoWithResolver for handlers that only know
// the application id at request time.
type AppDBResolver func(appID string) (*sql.DB, error)

// ClientRepo is the SQL-backed ClientRegistry + CRUD surface.
// The non-port CRUD methods (Insert, Update, Delete, ListForApp)
// are used by the admin-side wiring package to implement its
// HTTP handlers.
type ClientRepo struct {
	db      *sql.DB
	resolve AppDBResolver
	// SecretHasher isolates bcrypt so tests can swap in a
	// reversible stub. Production wiring plugs in a bcrypt-
	// based implementation.
	SecretHasher SecretHasher
	// Now overrides time.Now for deterministic tests.
	Now func() time.Time
}

// SecretHasher is the two-function hash + verify surface.
// Production wiring uses bcrypt; tests use a plain-equality
// stub so they stay fast and deterministic.
type SecretHasher interface {
	Hash(plain string) (string, error)
	Verify(hashed, plain string) error
}

// NewClientRepo returns a ClientRepo over the given DB. The
// SecretHasher + Now fields are exposed for dependency-inject;
// callers that leave Now nil get time.Now.
func NewClientRepo(db *sql.DB, h SecretHasher) *ClientRepo {
	return &ClientRepo{db: db, SecretHasher: h, Now: time.Now}
}

// NewClientRepoWithResolver returns a ClientRepo that resolves
// the per-org DB at query time using the given AppDBResolver.
// Use this when the caller only knows the application id at
// request time (e.g. the OAuth server authorize/token endpoints).
func NewClientRepoWithResolver(resolve AppDBResolver, h SecretHasher) *ClientRepo {
	return &ClientRepo{resolve: resolve, SecretHasher: h, Now: time.Now}
}

// dbFor returns the DB to use for the given application id.
func (r *ClientRepo) dbFor(appID string) (*sql.DB, error) {
	if r.db != nil {
		return r.db, nil
	}
	if r.resolve == nil {
		return nil, fmt.Errorf("sqlstore: no db or resolver configured")
	}
	return r.resolve(appID)
}

// -- ClientRegistry implementation --------------------------------

// FindByClientID satisfies oauthserver.ClientRegistry.
func (r *ClientRepo) FindByClientID(ctx context.Context, applicationID, clientID string) (*entity.Client, error) {
	db, err := r.dbFor(applicationID)
	if err != nil {
		return nil, fmt.Errorf("sqlstore: find by client id: resolve db: %w", err)
	}
	row := db.QueryRowContext(ctx,
		`SELECT id, application_id, client_id, client_secret_hash, client_type,
		        display_name, redirect_uris, allowed_scopes, require_consent,
		        access_token_ttl, refresh_token_ttl, is_active, created_at, updated_at
		 FROM application_oauth_clients
		 WHERE application_id = ? AND client_id = ?
		 LIMIT 1`,
		applicationID, clientID,
	)
	c, err := scanClient(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, entity.ErrClientNotFound
		}
		return nil, err
	}
	return c, nil
}

// VerifySecret satisfies oauthserver.ClientRegistry. Public
// clients (no stored secret) MUST receive an empty submission;
// anything else is a spec violation.
func (r *ClientRepo) VerifySecret(_ context.Context, client *entity.Client, submitted string) error {
	if client == nil {
		return entity.ErrClientNotFound
	}
	// Public client: no secret expected. A non-empty submission
	// indicates client confusion and we reject to avoid accepting
	// an empty-string equivalence.
	if client.Type == entity.ClientTypePublic {
		if subtle.ConstantTimeCompare([]byte(submitted), nil) == 0 && submitted != "" {
			return entity.NewOAuthError(entity.ErrCodeInvalidClient, "public client must not send client_secret", 401)
		}
		return nil
	}
	if client.SecretHash == "" {
		return entity.NewOAuthError(entity.ErrCodeInvalidClient, "client has no stored secret", 401)
	}
	if r.SecretHasher == nil {
		return fmt.Errorf("sqlstore: no SecretHasher configured")
	}
	if err := r.SecretHasher.Verify(client.SecretHash, submitted); err != nil {
		return entity.NewOAuthError(entity.ErrCodeInvalidClient, "client authentication failed", 401)
	}
	return nil
}

// -- CRUD for the admin wiring layer -----------------------------

// InsertInput is the plaintext shape the admin create handler
// already has in hand. The repo hashes the secret before
// persisting.
type InsertInput struct {
	ApplicationID   string
	ClientID        string
	ClientSecret    string // plaintext, hashed before storage; empty for public
	Type            entity.ClientType
	DisplayName     string
	RedirectURIs    []string
	AllowedScopes   []string
	RequireConsent  bool
	AccessTokenTTL  int
	RefreshTokenTTL int
	IsActive        bool
}

// Insert persists a new client. Returns ErrDuplicateClient when
// (application_id, client_id) is already registered.
func (r *ClientRepo) Insert(ctx context.Context, id string, in InsertInput) (*entity.Client, error) {
	db, err := r.dbFor(in.ApplicationID)
	if err != nil {
		return nil, fmt.Errorf("sqlstore: insert client: resolve db: %w", err)
	}
	now := r.Now().UTC().Format(time.RFC3339)
	hash := ""
	if in.Type == entity.ClientTypeConfidential {
		if r.SecretHasher == nil {
			return nil, fmt.Errorf("sqlstore: no SecretHasher configured")
		}
		if strings.TrimSpace(in.ClientSecret) == "" {
			return nil, entity.NewOAuthError(entity.ErrCodeInvalidRequest,
				"confidential clients require a non-empty client_secret", 400)
		}
		hash, err = r.SecretHasher.Hash(in.ClientSecret)
		if err != nil {
			return nil, fmt.Errorf("sqlstore: hash secret: %w", err)
		}
	}
	urisJSON, _ := json.Marshal(in.RedirectURIs)
	scopesJSON, _ := json.Marshal(in.AllowedScopes)
	_, err = db.ExecContext(ctx,
		`INSERT INTO application_oauth_clients
		 (id, application_id, client_id, client_secret_hash, client_type,
		  display_name, redirect_uris, allowed_scopes, require_consent,
		  access_token_ttl, refresh_token_ttl, is_active, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?)`,
		id, in.ApplicationID, in.ClientID, hashOrNil(hash),
		string(in.Type), in.DisplayName, string(urisJSON), string(scopesJSON),
		boolToInt(in.RequireConsent), in.AccessTokenTTL, in.RefreshTokenTTL,
		now, now,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, entity.ErrDuplicateClient
		}
		return nil, fmt.Errorf("sqlstore: insert client: %w", err)
	}
	return r.FindByID(ctx, in.ApplicationID, id)
}

// UpdateInput carries the mutable fields of a client. A nil
// ClientSecret pointer means "leave the stored secret alone";
// an explicit empty string is rejected for confidential clients
// (use Delete instead).
type UpdateInput struct {
	ClientSecret    *string
	DisplayName     string
	RedirectURIs    []string
	AllowedScopes   []string
	RequireConsent  bool
	AccessTokenTTL  int
	RefreshTokenTTL int
	IsActive        bool
}

// Update applies an UpdateInput to the identified row, scoped by
// application id. Returns ErrClientNotFound when the row doesn't
// exist under that application.
func (r *ClientRepo) Update(ctx context.Context, applicationID, id string, in UpdateInput) (*entity.Client, error) {
	db, err := r.dbFor(applicationID)
	if err != nil {
		return nil, fmt.Errorf("sqlstore: update client: resolve db: %w", err)
	}
	existing, err := r.FindByID(ctx, applicationID, id)
	if err != nil {
		return nil, err
	}
	hash := ""
	rotate := false
	if in.ClientSecret != nil {
		if existing.Type == entity.ClientTypePublic {
			return nil, entity.NewOAuthError(entity.ErrCodeInvalidRequest,
				"public clients can't hold a secret", 400)
		}
		if strings.TrimSpace(*in.ClientSecret) == "" {
			return nil, entity.NewOAuthError(entity.ErrCodeInvalidRequest,
				"client_secret may not be blank — delete the client instead", 400)
		}
		if r.SecretHasher == nil {
			return nil, fmt.Errorf("sqlstore: no SecretHasher configured")
		}
		hash, err = r.SecretHasher.Hash(*in.ClientSecret)
		if err != nil {
			return nil, fmt.Errorf("sqlstore: hash secret: %w", err)
		}
		rotate = true
	}
	urisJSON, _ := json.Marshal(in.RedirectURIs)
	scopesJSON, _ := json.Marshal(in.AllowedScopes)
	now := r.Now().UTC().Format(time.RFC3339)
	if rotate {
		_, err = db.ExecContext(ctx,
			`UPDATE application_oauth_clients SET
			   client_secret_hash = ?,
			   display_name = ?, redirect_uris = ?, allowed_scopes = ?,
			   require_consent = ?, access_token_ttl = ?, refresh_token_ttl = ?,
			   is_active = ?, updated_at = ?
			 WHERE id = ? AND application_id = ?`,
			hash,
			in.DisplayName, string(urisJSON), string(scopesJSON),
			boolToInt(in.RequireConsent), in.AccessTokenTTL, in.RefreshTokenTTL,
			boolToInt(in.IsActive), now,
			id, applicationID,
		)
	} else {
		_, err = db.ExecContext(ctx,
			`UPDATE application_oauth_clients SET
			   display_name = ?, redirect_uris = ?, allowed_scopes = ?,
			   require_consent = ?, access_token_ttl = ?, refresh_token_ttl = ?,
			   is_active = ?, updated_at = ?
			 WHERE id = ? AND application_id = ?`,
			in.DisplayName, string(urisJSON), string(scopesJSON),
			boolToInt(in.RequireConsent), in.AccessTokenTTL, in.RefreshTokenTTL,
			boolToInt(in.IsActive), now,
			id, applicationID,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("sqlstore: update client: %w", err)
	}
	return r.FindByID(ctx, applicationID, id)
}

// Delete removes a client row. Returns no error when the row is
// already absent — delete is idempotent.
func (r *ClientRepo) Delete(ctx context.Context, applicationID, id string) error {
	db, err := r.dbFor(applicationID)
	if err != nil {
		return fmt.Errorf("sqlstore: delete client: resolve db: %w", err)
	}
	_, err = db.ExecContext(ctx,
		`DELETE FROM application_oauth_clients
		 WHERE id = ? AND application_id = ?`,
		id, applicationID,
	)
	if err != nil {
		return fmt.Errorf("sqlstore: delete client: %w", err)
	}
	return nil
}

// FindByID is the admin-side "read this row by primary key"
// helper. Scoped by application id so a leaked id from a
// different app can't be used to read another app's clients.
func (r *ClientRepo) FindByID(ctx context.Context, applicationID, id string) (*entity.Client, error) {
	db, err := r.dbFor(applicationID)
	if err != nil {
		return nil, fmt.Errorf("sqlstore: find by id: resolve db: %w", err)
	}
	row := db.QueryRowContext(ctx,
		`SELECT id, application_id, client_id, client_secret_hash, client_type,
		        display_name, redirect_uris, allowed_scopes, require_consent,
		        access_token_ttl, refresh_token_ttl, is_active, created_at, updated_at
		 FROM application_oauth_clients
		 WHERE id = ? AND application_id = ?
		 LIMIT 1`,
		id, applicationID,
	)
	c, err := scanClient(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, entity.ErrClientNotFound
		}
		return nil, err
	}
	return c, nil
}

// ListForApp returns every client registered under the given
// application, ordered by created_at. Secret hashes are
// returned — the wiring layer masks them before exposing to the
// admin UI.
func (r *ClientRepo) ListForApp(ctx context.Context, applicationID string) ([]entity.Client, error) {
	db, err := r.dbFor(applicationID)
	if err != nil {
		return nil, fmt.Errorf("sqlstore: list clients: resolve db: %w", err)
	}
	rows, err := db.QueryContext(ctx,
		`SELECT id, application_id, client_id, client_secret_hash, client_type,
		        display_name, redirect_uris, allowed_scopes, require_consent,
		        access_token_ttl, refresh_token_ttl, is_active, created_at, updated_at
		 FROM application_oauth_clients
		 WHERE application_id = ?
		 ORDER BY created_at ASC`,
		applicationID,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlstore: list clients: %w", err)
	}
	defer rows.Close()

	out := []entity.Client{}
	for rows.Next() {
		c, err := scanClient(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

// -- helpers -----------------------------------------------------

type rowScanner interface {
	Scan(dest ...interface{}) error
}

func scanClient(row rowScanner) (*entity.Client, error) {
	var (
		c              entity.Client
		secretHash     sql.NullString
		urisRaw        string
		scopesRaw      string
		requireConsent int
		isActive       int
		clientType     string
	)
	err := row.Scan(
		&c.ID, &c.ApplicationID, &c.ClientID, &secretHash, &clientType,
		&c.DisplayName, &urisRaw, &scopesRaw, &requireConsent,
		&c.AccessTokenTTL, &c.RefreshTokenTTL, &isActive, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	c.Type = entity.ClientType(clientType)
	if secretHash.Valid {
		c.SecretHash = secretHash.String
	}
	if urisRaw != "" {
		_ = json.Unmarshal([]byte(urisRaw), &c.RedirectURIs)
	}
	if scopesRaw != "" {
		_ = json.Unmarshal([]byte(scopesRaw), &c.AllowedScopes)
	}
	c.RequireConsent = requireConsent != 0
	c.IsActive = isActive != 0
	return &c, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func hashOrNil(h string) interface{} {
	if h == "" {
		return nil
	}
	return h
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToUpper(err.Error())
	return strings.Contains(msg, "UNIQUE") || strings.Contains(msg, "CONSTRAINT FAILED")
}

// compile-time check: *ClientRepo satisfies the ClientRegistry
// interface declared in the parent package. Prevents drift
// between the two if either side is edited later.
var _ interface {
	FindByClientID(ctx context.Context, applicationID, clientID string) (*entity.Client, error)
	VerifySecret(ctx context.Context, client *entity.Client, submittedSecret string) error
} = (*ClientRepo)(nil)
