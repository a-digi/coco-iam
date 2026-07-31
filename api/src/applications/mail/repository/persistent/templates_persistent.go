package persistent

import (
	"database/sql"
	"errors"
	"fmt"
)

var (
	ErrTemplateNotFound      = errors.New("app mail template: not found")
	ErrTemplateDuplicateName = errors.New("app mail template: name already exists")
)

// AppMailTemplateWrite is the full row a Create/Update call writes.
type AppMailTemplateWrite struct {
	ID          string
	Name        string
	Description string
	Subject     string
	TextBody    string
	HTMLBody    string
	IsActive    bool
}

type AppMailTemplatesPersistentRepo struct {
	db    *sql.DB
	appID string
}

func NewAppMailTemplatesPersistentRepo(db *sql.DB, appID string) *AppMailTemplatesPersistentRepo {
	return &AppMailTemplatesPersistentRepo{db: db, appID: appID}
}

// Create inserts a new template, generating an ID if none was
// supplied, and defaulting IsActive to true unless the caller
// explicitly opts out — mirrors the org tier's Create.
func (r *AppMailTemplatesPersistentRepo) Create(t AppMailTemplateWrite) (string, error) {
	if t.ID == "" {
		t.ID = newUUID()
	}
	if !t.IsActive {
		t.IsActive = true
	}
	_, err := r.db.Exec(
		`INSERT INTO app_mail_templates (id, application_id, name, description, subject, text_body, html_body, is_active, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		t.ID, r.appID, t.Name, t.Description, t.Subject, t.TextBody, t.HTMLBody, t.IsActive,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return "", ErrTemplateDuplicateName
		}
		return "", fmt.Errorf("app mail template: create: %w", err)
	}
	return t.ID, nil
}

// Update replaces every mutable field on the row keyed by t.ID — Name
// is immutable, callers already merged the patch via the query repo.
func (r *AppMailTemplatesPersistentRepo) Update(t AppMailTemplateWrite) error {
	res, err := r.db.Exec(
		`UPDATE app_mail_templates
		 SET description = ?, subject = ?, text_body = ?, html_body = ?, is_active = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE id = ? AND application_id = ?`,
		t.Description, t.Subject, t.TextBody, t.HTMLBody, t.IsActive, t.ID, r.appID,
	)
	if err != nil {
		return fmt.Errorf("app mail template: update: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrTemplateNotFound
	}
	return nil
}

func (r *AppMailTemplatesPersistentRepo) Delete(id string) error {
	res, err := r.db.Exec(`DELETE FROM app_mail_templates WHERE id = ? AND application_id = ?`, id, r.appID)
	if err != nil {
		return fmt.Errorf("app mail template: delete: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrTemplateNotFound
	}
	return nil
}
