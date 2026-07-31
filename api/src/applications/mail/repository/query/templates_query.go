package query

import (
	"database/sql"
	"errors"
	"fmt"
)

// ErrTemplateNotFound is returned by Get / GetByName when no row matches.
var ErrTemplateNotFound = errors.New("app mail template: not found")

// AppMailTemplate mirrors api/src/mail/template.Template.
type AppMailTemplate struct {
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

// AppMailTemplateListFilter narrows List — mirrors template.ListFilter.
type AppMailTemplateListFilter struct {
	NameLike string
	Limit    int
	Offset   int
}

// AppMailTemplatesQueryRepo reads app_mail_templates scoped to a
// specific application.
type AppMailTemplatesQueryRepo struct {
	db    *sql.DB
	appID string
}

func NewAppMailTemplatesQueryRepo(db *sql.DB, appID string) *AppMailTemplatesQueryRepo {
	return &AppMailTemplatesQueryRepo{db: db, appID: appID}
}

func (r *AppMailTemplatesQueryRepo) Get(id string) (*AppMailTemplate, error) {
	row := r.db.QueryRow(
		`SELECT id, name, description, subject, text_body, html_body, is_active, created_at, updated_at
		 FROM app_mail_templates WHERE id = ? AND application_id = ?`, id, r.appID,
	)
	return scanTemplateRow(row)
}

// GetByName is the hot path the scoped resolver uses at send time.
func (r *AppMailTemplatesQueryRepo) GetByName(name string) (*AppMailTemplate, error) {
	row := r.db.QueryRow(
		`SELECT id, name, description, subject, text_body, html_body, is_active, created_at, updated_at
		 FROM app_mail_templates WHERE name = ? AND application_id = ?`, name, r.appID,
	)
	return scanTemplateRow(row)
}

func (r *AppMailTemplatesQueryRepo) List(f AppMailTemplateListFilter) ([]AppMailTemplate, int, error) {
	where := " WHERE application_id = ?"
	args := []interface{}{r.appID}
	if f.NameLike != "" {
		where += " AND name LIKE ?"
		args = append(args, "%"+f.NameLike+"%")
	}

	var total int
	if err := r.db.QueryRow(`SELECT COUNT(1) FROM app_mail_templates`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("app mail template: count: %w", err)
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
		 FROM app_mail_templates`+where+` ORDER BY name ASC LIMIT ? OFFSET ?`,
		argsWithPage...,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("app mail template: list: %w", err)
	}
	defer rows.Close()

	out := make([]AppMailTemplate, 0)
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

func scanTemplateRow(r scannableTemplate) (*AppMailTemplate, error) {
	var t AppMailTemplate
	err := r.Scan(
		&t.ID, &t.Name, &t.Description, &t.Subject,
		&t.TextBody, &t.HTMLBody, &t.IsActive,
		&t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTemplateNotFound
		}
		return nil, fmt.Errorf("app mail template: scan: %w", err)
	}
	return &t, nil
}
