package template

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/a-digi/coco-orm/orm"
)

// Repository is the CRUD layer over mail.db's mail_templates table.
// Completely independent of the generic ApiResourceHandler (which is
// bound to users.db) so templates stay a mail-exclusive concern.
type Repository struct {
	db *sql.DB
}

// NewRepository binds a Repository to the mail DatabaseManager.
func NewRepository(dbm *orm.DatabaseManager) *Repository {
	return &Repository{db: dbm.Connector.DB}
}

// ErrNotFound is returned by Get / GetByName / Update / Delete.
var ErrNotFound = errors.New("mail template: not found")

// ErrDuplicateName is returned by Create when `name` collides.
var ErrDuplicateName = errors.New("mail template: name already exists")

// Create inserts a new template. Caller is responsible for validating
// `t.Name` against NameFormat before calling.
func (r *Repository) Create(t Template) (Template, error) {
	if t.ID == "" {
		t.ID = newUUID()
	}
	if !t.IsActive {
		// Default new templates to active unless the caller explicitly opts out.
		t.IsActive = true
	}
	_, err := r.db.Exec(
		`INSERT INTO mail_templates (id, name, description, subject, text_body, html_body, is_active, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		t.ID, t.Name, t.Description, t.Subject, t.TextBody, t.HTMLBody, t.IsActive,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return Template{}, ErrDuplicateName
		}
		return Template{}, fmt.Errorf("mail template: create: %w", err)
	}
	created, gerr := r.Get(t.ID)
	if gerr != nil {
		return Template{}, gerr
	}
	return *created, nil
}

// Get fetches by primary key.
func (r *Repository) Get(id string) (*Template, error) {
	row := r.db.QueryRow(
		`SELECT id, name, description, subject, text_body, html_body, is_active, created_at, updated_at
		 FROM mail_templates WHERE id = ?`, id,
	)
	return scanRow(row)
}

// GetByName is the hot path used by the renderer.
func (r *Repository) GetByName(name string) (*Template, error) {
	row := r.db.QueryRow(
		`SELECT id, name, description, subject, text_body, html_body, is_active, created_at, updated_at
		 FROM mail_templates WHERE name = ?`, name,
	)
	return scanRow(row)
}

// List returns paginated rows + total count for the same filter.
func (r *Repository) List(f ListFilter) ([]Template, int, error) {
	var where []string
	var args []interface{}
	if f.NameLike != "" {
		where = append(where, "name LIKE ?")
		args = append(args, "%"+f.NameLike+"%")
	}
	if f.DescriptionLike != "" {
		where = append(where, "description LIKE ?")
		args = append(args, "%"+f.DescriptionLike+"%")
	}
	whereSQL := ""
	if len(where) > 0 {
		whereSQL = " WHERE " + joinAnd(where)
	}

	var total int
	if err := r.db.QueryRow(`SELECT COUNT(1) FROM mail_templates`+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("mail template: count: %w", err)
	}

	limit := f.Limit
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}
	argsWithPage := append(append([]interface{}{}, args...), limit, offset)

	rows, err := r.db.Query(
		`SELECT id, name, description, subject, text_body, html_body, is_active, created_at, updated_at
		 FROM mail_templates`+whereSQL+` ORDER BY name ASC LIMIT ? OFFSET ?`,
		argsWithPage...,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("mail template: list: %w", err)
	}
	defer rows.Close()

	out := make([]Template, 0)
	for rows.Next() {
		t, err := scanRow(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *t)
	}
	return out, total, rows.Err()
}

// Update applies a Patch to the row. Name is immutable.
func (r *Repository) Update(id string, p Patch) (Template, error) {
	existing, err := r.Get(id)
	if err != nil {
		return Template{}, err
	}
	if p.Description != nil {
		existing.Description = *p.Description
	}
	if p.Subject != nil {
		existing.Subject = *p.Subject
	}
	if p.TextBody != nil {
		existing.TextBody = *p.TextBody
	}
	if p.HTMLBody != nil {
		existing.HTMLBody = *p.HTMLBody
	}
	if p.IsActive != nil {
		existing.IsActive = *p.IsActive
	}
	_, err = r.db.Exec(
		`UPDATE mail_templates
		 SET description = ?, subject = ?, text_body = ?, html_body = ?, is_active = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE id = ?`,
		existing.Description, existing.Subject, existing.TextBody, existing.HTMLBody, existing.IsActive, id,
	)
	if err != nil {
		return Template{}, fmt.Errorf("mail template: update: %w", err)
	}
	after, gerr := r.Get(id)
	if gerr != nil {
		return Template{}, gerr
	}
	return *after, nil
}

// Delete hard-deletes by id.
func (r *Repository) Delete(id string) error {
	res, err := r.db.Exec(`DELETE FROM mail_templates WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("mail template: delete: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

type scannable interface {
	Scan(dest ...interface{}) error
}

func scanRow(r scannable) (*Template, error) {
	var t Template
	err := r.Scan(
		&t.ID, &t.Name, &t.Description, &t.Subject,
		&t.TextBody, &t.HTMLBody, &t.IsActive,
		&t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("mail template: scan: %w", err)
	}
	return &t, nil
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

// isUniqueViolation detects the SQLite "UNIQUE constraint failed" error
// without binding to a driver-specific type (keeps the repository easy to
// test with a different sqlite driver if we ever swap).
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return contains(msg, "UNIQUE constraint failed")
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	// tiny inline — avoids importing strings just for one call
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func newUUID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "00000000-0000-0000-0000-000000000000"
	}
	hx := hex.EncodeToString(buf)
	return fmt.Sprintf("%s-%s-%s-%s-%s", hx[:8], hx[8:12], hx[12:16], hx[16:20], hx[20:32])
}
