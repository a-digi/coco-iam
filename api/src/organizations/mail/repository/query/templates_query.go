package query

import (
	"database/sql"
	"errors"
	"fmt"
)

// ErrTemplateNotFound is returned by Get / GetByName when no row matches.
var ErrTemplateNotFound = errors.New("org mail template: not found")

// OrgMailTemplate mirrors api/src/mail/template.Template.
type OrgMailTemplate struct {
	ID          string
	Name        string
	Description string
	Subject     string
	TextBody    string
	HTMLBody    string
	IsActive    bool
	CreatedAt   string
	UpdatedAt   string
}

// OrgMailTemplateListFilter narrows List — mirrors template.ListFilter.
type OrgMailTemplateListFilter struct {
	NameLike string
	Limit    int
	Offset   int
}

// OrgMailTemplatesQueryRepo reads org_mail_templates from a specific
// org's users.db.
type OrgMailTemplatesQueryRepo struct {
	db *sql.DB
}

func NewOrgMailTemplatesQueryRepo(db *sql.DB) *OrgMailTemplatesQueryRepo {
	return &OrgMailTemplatesQueryRepo{db: db}
}

func (r *OrgMailTemplatesQueryRepo) Get(id string) (*OrgMailTemplate, error) {
	row := r.db.QueryRow(
		`SELECT id, name, description, subject, text_body, html_body, is_active, created_at, updated_at
		 FROM org_mail_templates WHERE id = ?`, id,
	)
	return scanTemplateRow(row)
}

// GetByName is the hot path the scoped resolver uses at send time.
func (r *OrgMailTemplatesQueryRepo) GetByName(name string) (*OrgMailTemplate, error) {
	row := r.db.QueryRow(
		`SELECT id, name, description, subject, text_body, html_body, is_active, created_at, updated_at
		 FROM org_mail_templates WHERE name = ?`, name,
	)
	return scanTemplateRow(row)
}

func (r *OrgMailTemplatesQueryRepo) List(f OrgMailTemplateListFilter) ([]OrgMailTemplate, int, error) {
	var where string
	var args []interface{}
	if f.NameLike != "" {
		where = " WHERE name LIKE ?"
		args = append(args, "%"+f.NameLike+"%")
	}

	var total int
	if err := r.db.QueryRow(`SELECT COUNT(1) FROM org_mail_templates`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("org mail template: count: %w", err)
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
		 FROM org_mail_templates`+where+` ORDER BY name ASC LIMIT ? OFFSET ?`,
		argsWithPage...,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("org mail template: list: %w", err)
	}
	defer rows.Close()

	out := make([]OrgMailTemplate, 0)
	for rows.Next() {
		t, serr := scanTemplateRow(rows)
		if serr != nil {
			return nil, 0, serr
		}
		out = append(out, *t)
	}
	return out, total, rows.Err()
}

type scannableTemplate interface {
	Scan(dest ...interface{}) error
}

func scanTemplateRow(r scannableTemplate) (*OrgMailTemplate, error) {
	var t OrgMailTemplate
	err := r.Scan(
		&t.ID, &t.Name, &t.Description, &t.Subject,
		&t.TextBody, &t.HTMLBody, &t.IsActive,
		&t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTemplateNotFound
		}
		return nil, fmt.Errorf("org mail template: scan: %w", err)
	}
	return &t, nil
}
