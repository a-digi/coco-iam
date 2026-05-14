package activation

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// Sentinel errors used by the public flow handlers.
var (
	ErrNotFound        = errors.New("activation: token not found")
	ErrExpired         = errors.New("activation: token expired")
	ErrAlreadyUsed     = errors.New("activation: token already consumed")
	ErrCooldown        = errors.New("activation: resend cooldown not elapsed")
	ErrNoActivePending = errors.New("activation: no pending activation for user")
)

// Row is the in-memory representation of an activation row.
// UserType is injected by the store that reads the row (not stored in DB).
type Row struct {
	ID               string
	UserID           string
	UserType         UserType
	TokenHash        string
	TempPasswordHash string
	ExpiresAt        time.Time
	ConsumedAt       *time.Time
	CreatedAt        time.Time
	// Redirect* are optional slugs stamped at Start time. When all
	// three are non-empty the activate flow composes a per-app login
	// URL from them. Empty strings indicate no redirect.
	RedirectOrgSlug       string
	RedirectWorkspaceSlug string
	RedirectClientID      string
}

type scannable interface {
	Scan(dest ...interface{}) error
}

// scanRow scans 10 columns (id, user_id, token_hash, temp_password_hash,
// expires_at, consumed_at, created_at, redirect_*) from r and injects
// userType. Neither admin_activations nor per-org user_activations stores
// the type column — it is known from which table was queried.
func scanRow(r scannable, userType UserType) (*Row, error) {
	var row Row
	var expires string
	var consumed sql.NullString
	var created sql.NullString
	var redirectOrg, redirectWs, redirectClient sql.NullString
	err := r.Scan(
		&row.ID, &row.UserID, &row.TokenHash, &row.TempPasswordHash,
		&expires, &consumed, &created,
		&redirectOrg, &redirectWs, &redirectClient,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("activation: scan: %w", err)
	}
	row.UserType = userType
	if t, perr := parseTime(expires); perr == nil {
		row.ExpiresAt = t
	}
	if consumed.Valid {
		if t, perr := parseTime(consumed.String); perr == nil {
			row.ConsumedAt = &t
		}
	}
	if created.Valid {
		if t, perr := parseTime(created.String); perr == nil {
			row.CreatedAt = t
		}
	}
	if redirectOrg.Valid {
		row.RedirectOrgSlug = redirectOrg.String
	}
	if redirectWs.Valid {
		row.RedirectWorkspaceSlug = redirectWs.String
	}
	if redirectClient.Valid {
		row.RedirectClientID = redirectClient.String
	}
	return &row, nil
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
	return time.Time{}, fmt.Errorf("activation: unparseable timestamp %q", s)
}

func newUUID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "00000000-0000-0000-0000-000000000000"
	}
	hx := hex.EncodeToString(buf)
	return fmt.Sprintf("%s-%s-%s-%s-%s", hx[:8], hx[8:12], hx[12:16], hx[16:20], hx[20:32])
}
