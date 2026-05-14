// Package store persists mail metadata in its own SQLite database
// (./data/db/mail.db), distinct from the main users.db. The table is the
// audit source-of-truth for every outbound message; the actual body bytes
// live on the queue's file-on-disk payload so the row stays small.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/a-digi/coco-logger/logger"
	"github.com/a-digi/coco-orm/orm"
)

// Address mirrors mail.Address but lives here so the store package can
// stay free of a dependency on the mail package (avoids an import cycle
// since mail.service depends on store).
type Address struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email"`
}

// Status values recorded on mail_outbound.status.
const (
	StatusQueued       = "queued"
	StatusSending      = "sending"
	StatusSent         = "sent"
	StatusRetrying     = "retrying"
	StatusFailed       = "failed"
	StatusDeadLettered = "dead_lettered"
)

// Row is the in-memory representation of a single mail_outbound row.
type Row struct {
	ID            string             `json:"id"`
	Template      string             `json:"template"`
	Subject       string             `json:"subject"`
	From          Address   `json:"from"`
	To            []Address `json:"to"`
	Cc            []Address `json:"cc,omitempty"`
	Bcc           []Address `json:"bcc,omitempty"`
	Status        string             `json:"status"`
	Attempts      int                `json:"attempts"`
	MaxAttempts   int                `json:"max_attempts"`
	LastError     string             `json:"last_error"`
	NextAttemptAt string             `json:"next_attempt_at,omitempty"`
	CreatedAt     string             `json:"created_at"`
	UpdatedAt     string             `json:"updated_at"`
	SentAt        string             `json:"sent_at,omitempty"`
}

// InsertArgs is the payload MailService uses when persisting a freshly
// enqueued message — body bytes are intentionally out-of-scope here.
type InsertArgs struct {
	ID          string
	Template    string
	Subject     string
	From        Address
	To          []Address
	Cc          []Address
	Bcc         []Address
	MaxAttempts int
}

// ListFilter narrows the admin listing endpoint. Empty fields mean "don't
// filter"; `Statuses` is OR-joined.
type ListFilter struct {
	Statuses []string
	ToLike   string // substring match against to_json
	Limit    int
	Offset   int
}

// Store is the concrete mail.db wrapper.
type Store struct {
	db  *sql.DB
	log logger.Logger
}

// New constructs a Store bound to the given DatabaseManager. Install should
// already have run.
func New(dbm *orm.DatabaseManager, log logger.Logger) *Store {
	return &Store{db: dbm.Connector.DB, log: log}
}

// Insert records a fresh row with status='queued'.
func (s *Store) Insert(args InsertArgs) error {
	toBytes, err := json.Marshal(args.To)
	if err != nil {
		return fmt.Errorf("mail store: marshal to: %w", err)
	}
	ccBytes, err := json.Marshal(args.Cc)
	if err != nil {
		return fmt.Errorf("mail store: marshal cc: %w", err)
	}
	bccBytes, err := json.Marshal(args.Bcc)
	if err != nil {
		return fmt.Errorf("mail store: marshal bcc: %w", err)
	}
	_, err = s.db.Exec(
		`INSERT INTO mail_outbound (
			id, template, subject, from_email, from_name, to_json, cc_json, bcc_json,
			status, attempts, max_attempts, last_error, next_attempt_at,
			created_at, updated_at, sent_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'queued', 0, ?, '', '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, '')`,
		args.ID, args.Template, args.Subject,
		args.From.Email, args.From.Name,
		string(toBytes), string(ccBytes), string(bccBytes),
		args.MaxAttempts,
	)
	if err != nil {
		return fmt.Errorf("mail store: insert: %w", err)
	}
	return nil
}

// MarkSending flips a row to `sending` and bumps attempts. Returns a
// stable-ish snapshot for logging; callers don't need it otherwise.
func (s *Store) MarkSending(id string) error {
	_, err := s.db.Exec(
		`UPDATE mail_outbound
		 SET status = ?, attempts = attempts + 1, last_error = '', updated_at = CURRENT_TIMESTAMP
		 WHERE id = ?`,
		StatusSending, id,
	)
	if err != nil {
		return fmt.Errorf("mail store: mark sending: %w", err)
	}
	return nil
}

// MarkSent flips the row to the terminal `sent` state.
func (s *Store) MarkSent(id string) error {
	_, err := s.db.Exec(
		`UPDATE mail_outbound
		 SET status = ?, last_error = '', sent_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		 WHERE id = ?`,
		StatusSent, id,
	)
	if err != nil {
		return fmt.Errorf("mail store: mark sent: %w", err)
	}
	return nil
}

// MarkFailed records an error on the row. If willRetry is true the status
// moves to `retrying` (with the supplied next_attempt_at); otherwise the
// row is terminal as `dead_lettered`.
func (s *Store) MarkFailed(id, errMsg string, willRetry bool, nextAttempt time.Time) error {
	status := StatusDeadLettered
	var nextStr string
	if willRetry {
		status = StatusRetrying
		nextStr = nextAttempt.UTC().Format(time.RFC3339)
	}
	_, err := s.db.Exec(
		`UPDATE mail_outbound
		 SET status = ?, last_error = ?, next_attempt_at = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE id = ?`,
		status, errMsg, nextStr, id,
	)
	if err != nil {
		return fmt.Errorf("mail store: mark failed: %w", err)
	}
	return nil
}

// Requeue resets a terminal (failed / dead_lettered) row back to `queued`
// so it can be re-sent. Returns an error if the row isn't in a terminal
// state.
func (s *Store) Requeue(id string) error {
	res, err := s.db.Exec(
		`UPDATE mail_outbound
		 SET status = 'queued', attempts = 0, last_error = '', next_attempt_at = '', sent_at = '', updated_at = CURRENT_TIMESTAMP
		 WHERE id = ? AND status IN ('failed', 'dead_lettered')`,
		id,
	)
	if err != nil {
		return fmt.Errorf("mail store: requeue: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("mail store: row %q not found or not terminal", id)
	}
	return nil
}

// Get returns a single row, or ErrNoRows if missing.
func (s *Store) Get(id string) (*Row, error) {
	row := s.db.QueryRow(
		`SELECT id, template, subject, from_email, from_name, to_json, cc_json, bcc_json,
		        status, attempts, max_attempts, last_error, next_attempt_at,
		        created_at, updated_at, sent_at
		 FROM mail_outbound WHERE id = ?`,
		id,
	)
	return scanRow(row)
}

// List returns rows matching `f`, newest first. Defaults: limit 50, offset 0.
func (s *Store) List(f ListFilter) ([]Row, error) {
	q := `SELECT id, template, subject, from_email, from_name, to_json, cc_json, bcc_json,
	             status, attempts, max_attempts, last_error, next_attempt_at,
	             created_at, updated_at, sent_at
	      FROM mail_outbound`
	var args []interface{}
	var where []string
	if len(f.Statuses) > 0 {
		placeholders := ""
		for i, st := range f.Statuses {
			if i > 0 {
				placeholders += ","
			}
			placeholders += "?"
			args = append(args, st)
		}
		where = append(where, "status IN ("+placeholders+")")
	}
	if f.ToLike != "" {
		where = append(where, "to_json LIKE ?")
		args = append(args, "%"+f.ToLike+"%")
	}
	if len(where) > 0 {
		q += " WHERE " + joinAnd(where)
	}
	limit := f.Limit
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}
	q += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("mail store: list: %w", err)
	}
	defer rows.Close()

	out := make([]Row, 0, limit)
	for rows.Next() {
		r, err := scanRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

// Count returns the number of rows matching `f` (limit/offset ignored).
func (s *Store) Count(f ListFilter) (int, error) {
	q := `SELECT COUNT(1) FROM mail_outbound`
	var args []interface{}
	var where []string
	if len(f.Statuses) > 0 {
		placeholders := ""
		for i, st := range f.Statuses {
			if i > 0 {
				placeholders += ","
			}
			placeholders += "?"
			args = append(args, st)
		}
		where = append(where, "status IN ("+placeholders+")")
	}
	if f.ToLike != "" {
		where = append(where, "to_json LIKE ?")
		args = append(args, "%"+f.ToLike+"%")
	}
	if len(where) > 0 {
		q += " WHERE " + joinAnd(where)
	}
	var n int
	if err := s.db.QueryRow(q, args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("mail store: count: %w", err)
	}
	return n, nil
}

// CountActive returns the rows that still need work — used by the
// orchestrator to size the worker pool.
func (s *Store) CountActive(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(1) FROM mail_outbound WHERE status IN ('queued','retrying','sending')`,
	).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("mail store: count active: %w", err)
	}
	return n, nil
}

// ResetSending flips any rows stuck in `sending` (e.g. because the process
// crashed mid-send) back to `queued`. Called once on startup.
func (s *Store) ResetSending() error {
	_, err := s.db.Exec(
		`UPDATE mail_outbound SET status = 'queued', updated_at = CURRENT_TIMESTAMP WHERE status = 'sending'`,
	)
	if err != nil {
		return fmt.Errorf("mail store: reset sending: %w", err)
	}
	return nil
}

// PruneTerminal deletes terminal rows older than `olderThan`. Returns the
// number of rows deleted so the caller can log it.
func (s *Store) PruneTerminal(olderThan time.Duration) (int64, error) {
	cutoff := time.Now().UTC().Add(-olderThan).Format("2006-01-02 15:04:05")
	res, err := s.db.Exec(
		`DELETE FROM mail_outbound
		 WHERE status IN ('sent','failed','dead_lettered')
		   AND created_at < ?`,
		cutoff,
	)
	if err != nil {
		return 0, fmt.Errorf("mail store: prune: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

type scannable interface {
	Scan(dest ...interface{}) error
}

func scanRow(r scannable) (*Row, error) {
	var row Row
	var toJSON, ccJSON, bccJSON string
	if err := r.Scan(
		&row.ID, &row.Template, &row.Subject,
		&row.From.Email, &row.From.Name,
		&toJSON, &ccJSON, &bccJSON,
		&row.Status, &row.Attempts, &row.MaxAttempts, &row.LastError, &row.NextAttemptAt,
		&row.CreatedAt, &row.UpdatedAt, &row.SentAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, fmt.Errorf("mail store: scan row: %w", err)
	}
	if toJSON != "" {
		_ = json.Unmarshal([]byte(toJSON), &row.To)
	}
	if ccJSON != "" {
		_ = json.Unmarshal([]byte(ccJSON), &row.Cc)
	}
	if bccJSON != "" {
		_ = json.Unmarshal([]byte(bccJSON), &row.Bcc)
	}
	return &row, nil
}

func joinAnd(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += " AND "
		}
		out += p
	}
	return out
}
