package recovery

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// Sentinel errors used by both stores.
var (
	ErrNotFound    = errors.New("recovery: token not found")
	ErrExpired     = errors.New("recovery: token expired")
	ErrAlreadyUsed = errors.New("recovery: token already consumed")
	ErrCooldown    = errors.New("recovery: resend cooldown not elapsed")
)

type scannable interface {
	Scan(dest ...interface{}) error
}

// scanRowWithType scans a row from either admin_password_recoveries or the
// per-org password_recoveries (neither table stores user_type). The caller
// supplies the type so it is injected into the returned Row.
func scanRowWithType(r scannable, userType UserType) (*Row, error) {
	var row Row
	var expires string
	var consumed sql.NullString
	var created sql.NullString
	err := r.Scan(&row.ID, &row.UserID, &row.TokenHash, &expires, &consumed, &created)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("recovery: scan: %w", err)
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
	return time.Time{}, fmt.Errorf("recovery: unparseable timestamp %q", s)
}

func newUUID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "00000000-0000-0000-0000-000000000000"
	}
	hx := hex.EncodeToString(buf)
	return fmt.Sprintf("%s-%s-%s-%s-%s", hx[:8], hx[8:12], hx[12:16], hx[16:20], hx[20:32])
}
