package persistent

import (
	"database/sql"
	"errors"
	"fmt"
)

var (
	ErrTemplateNotFound      = errors.New("org mail template: not found")
	ErrTemplateDuplicateName = errors.New("org mail template: name already exists")
)

// OrgMailTemplateWrite is the full row a Create/Update call writes.
type OrgMailTemplateWrite struct {
	ID          string
	Name        string
	Description string
	Subject     string
	TextBody    string
	HTMLBody    string
	IsActive    bool
}

type OrgMailTemplatesPersistentRepo struct {
	db *sql.DB
}

func NewOrgMailTemplatesPersistentRepo(db *sql.DB) *OrgMailTemplatesPersistentRepo {
	return &OrgMailTemplatesPersistentRepo{db: db}
}

// Create inserts a new template, generating an ID if none was
// supplied, and defaulting IsActive to true unless the caller
// explicitly opts out — mirrors template.Repository.Create.
func (r *OrgMailTemplatesPersistentRepo) Create(t OrgMailTemplateWrite) (string, error) {
	if t.ID == "" {
		t.ID = newUUID()
	}
	if !t.IsActive {
		t.IsActive = true
	}
	_, err := r.db.Exec(
		`INSERT INTO org_mail_templates (id, name, description, subject, text_body, html_body, is_active, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		t.ID, t.Name, t.Description, t.Subject, t.TextBody, t.HTMLBody, t.IsActive,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return "", ErrTemplateDuplicateName
		}
		return "", fmt.Errorf("org mail template: create: %w", err)
	}
	return t.ID, nil
}

// Update replaces every mutable field on the row keyed by t.ID — Name
// is immutable, callers already merged the patch via the query repo.
func (r *OrgMailTemplatesPersistentRepo) Update(t OrgMailTemplateWrite) error {
	res, err := r.db.Exec(
		`UPDATE org_mail_templates
		 SET description = ?, subject = ?, text_body = ?, html_body = ?, is_active = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE id = ?`,
		t.Description, t.Subject, t.TextBody, t.HTMLBody, t.IsActive, t.ID,
	)
	if err != nil {
		return fmt.Errorf("org mail template: update: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrTemplateNotFound
	}
	return nil
}

func (r *OrgMailTemplatesPersistentRepo) Delete(id string) error {
	res, err := r.db.Exec(`DELETE FROM org_mail_templates WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("org mail template: delete: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrTemplateNotFound
	}
	return nil
}
