// Package repository owns the SQL over the per-org profiles.db
// tables that back the registration schema:
//   - application_registration_steps
//   - application_registration_fields
// Plus a read-only helper for the pre-existing profile_fields table
// so the service can resolve 'profile'-sourced rows without
// cross-package DB access.
//
// No DI — callers pass a *sql.DB resolved from the
// per-org profile registry.
package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/a-digi/coco-iam/src/applications/registrationfields/entity"
)

// ErrNotFound signals a missing profile_fields row referenced from
// a 'profile'-sourced registration field. Handlers translate this
// into a 400 on admin writes and silently skip orphans on public
// reads.
var ErrNotFound = errors.New("registrationfields: not found")

// ProfileField is the projection of profile_fields the service
// needs to resolve 'profile'-sourced rows. The real table has more
// columns (order_index, is_active, created_at, …) but they aren't
// relevant to the registration surface.
type ProfileField struct {
	ID          string
	Name        string
	Label       string
	Description string
	DataType    string
	IsRequired  bool
	MinValue    *int
	MaxValue    *int
	OptionsJSON string
	Regex       string
}

// Repository wraps a single per-org profiles.db.
type Repository struct {
	db *sql.DB
}

// New builds a Repository. Pass the *sql.DB resolved from the
// OrgDBRegistry for the target organisation.
func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// ListStepsForApp returns every step for the application, ordered
// by order_index ascending. An app that's never been configured
// returns an empty slice — not an error.
func (r *Repository) ListStepsForApp(appID string) ([]entity.Step, error) {
	rows, err := r.db.Query(
		`SELECT id, application_id, order_index, title, description, created_at, updated_at
		 FROM application_registration_steps
		 WHERE application_id = ?
		 ORDER BY order_index ASC, id ASC`,
		appID,
	)
	if err != nil {
		return nil, fmt.Errorf("registrationfields: list steps: %w", err)
	}
	defer rows.Close()

	var out []entity.Step
	for rows.Next() {
		var s entity.Step
		var created, updated sql.NullString
		if err := rows.Scan(&s.ID, &s.ApplicationID, &s.OrderIndex, &s.Title, &s.Description, &created, &updated); err != nil {
			return nil, fmt.Errorf("registrationfields: scan step: %w", err)
		}
		if created.Valid {
			if t, perr := parseTime(created.String); perr == nil {
				s.CreatedAt = t
			}
		}
		if updated.Valid {
			if t, perr := parseTime(updated.String); perr == nil {
				s.UpdatedAt = t
			}
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("registrationfields: iterate steps: %w", err)
	}
	return out, nil
}

// ListFieldsForApp returns every field for the application, joined
// through steps so the result is already grouped by (step order,
// field order) — the service can group into steps without a second
// sort. Steps with zero fields simply don't appear in this list;
// the service stitches them back in via ListStepsForApp.
func (r *Repository) ListFieldsForApp(appID string) ([]entity.Field, error) {
	rows, err := r.db.Query(
		`SELECT f.id, f.application_id, f.step_id, f.order_index,
		        f.source, f.profile_field_id, f.required_override,
		        f.name, f.label, f.description, f.data_type, f.is_required,
		        f.min_value, f.max_value, f.options_json, f.regex,
		        f.created_at, f.updated_at
		 FROM application_registration_fields f
		 JOIN application_registration_steps s ON s.id = f.step_id
		 WHERE f.application_id = ?
		 ORDER BY s.order_index ASC, f.order_index ASC, f.id ASC`,
		appID,
	)
	if err != nil {
		return nil, fmt.Errorf("registrationfields: list fields: %w", err)
	}
	defer rows.Close()

	var out []entity.Field
	for rows.Next() {
		f, serr := scanField(rows)
		if serr != nil {
			return nil, serr
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("registrationfields: iterate fields: %w", err)
	}
	return out, nil
}

// ReplaceForApp atomically replaces every step + field for the
// application. In one transaction we delete all existing rows for
// the app, then insert the supplied ones. The caller supplies
// stable IDs; the repository doesn't generate them so drag-to-
// reorder in the admin UI doesn't look like a delete+add in
// subsequent audit queries.
//
// Enforces:
//   - each field's StepID must appear in the steps slice.
//   - step + field order indexes may be whatever the caller sets;
//     the repo doesn't renumber.
func (r *Repository) ReplaceForApp(appID string, steps []entity.Step, fields []entity.Field) error {
	if appID == "" {
		return errors.New("registrationfields: application id is required")
	}

	// Application-level FK check — catches payloads that would
	// otherwise leave fields pointing at steps that won't exist on
	// disk. Done before opening the transaction so the error path
	// doesn't touch any DB state.
	validStepIDs := make(map[string]struct{}, len(steps))
	for _, s := range steps {
		if s.ID == "" {
			return errors.New("registrationfields: step id is required")
		}
		validStepIDs[s.ID] = struct{}{}
	}
	for _, f := range fields {
		if f.ID == "" {
			return errors.New("registrationfields: field id is required")
		}
		if _, ok := validStepIDs[f.StepID]; !ok {
			return fmt.Errorf("registrationfields: field %q references unknown step %q", f.ID, f.StepID)
		}
		if !f.Source.IsValid() {
			return fmt.Errorf("registrationfields: field %q has invalid source %q", f.ID, f.Source)
		}
	}

	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("registrationfields: begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	// Wipe. The order doesn't matter at the engine level (no FKs),
	// but we keep fields-before-steps as a habit.
	if _, err := tx.Exec(
		`DELETE FROM application_registration_fields WHERE application_id = ?`, appID,
	); err != nil {
		return fmt.Errorf("registrationfields: delete fields: %w", err)
	}
	if _, err := tx.Exec(
		`DELETE FROM application_registration_steps WHERE application_id = ?`, appID,
	); err != nil {
		return fmt.Errorf("registrationfields: delete steps: %w", err)
	}

	now := time.Now().UTC().Format("2006-01-02 15:04:05")

	for _, s := range steps {
		if _, err := tx.Exec(
			`INSERT INTO application_registration_steps
			 (id, application_id, order_index, title, description, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			s.ID, appID, s.OrderIndex, s.Title, s.Description, now, now,
		); err != nil {
			return fmt.Errorf("registrationfields: insert step %q: %w", s.ID, err)
		}
	}

	for _, f := range fields {
		if _, err := tx.Exec(
			`INSERT INTO application_registration_fields
			 (id, application_id, step_id, order_index, source,
			  profile_field_id, required_override, name, label,
			  description, data_type, is_required, min_value, max_value,
			  options_json, regex, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			f.ID, appID, f.StepID, f.OrderIndex, string(f.Source),
			nullableString(f.ProfileFieldID),
			nullableBool(f.RequiredOverride),
			f.Name, f.Label, f.Description, f.DataType, f.IsRequired,
			nullableInt(f.MinValue), nullableInt(f.MaxValue),
			defaultString(f.OptionsJSON, "[]"), f.Regex,
			now, now,
		); err != nil {
			return fmt.Errorf("registrationfields: insert field %q: %w", f.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("registrationfields: commit: %w", err)
	}
	return nil
}

// LookupProfileField fetches the profile_fields row the service
// needs to resolve a 'profile'-sourced registration field.
// Returns ErrNotFound if the row is missing or inactive — the
// service treats that as "orphan" and skips the registration row.
func (r *Repository) LookupProfileField(id string) (*ProfileField, error) {
	var out ProfileField
	var minV, maxV sql.NullInt64
	err := r.db.QueryRow(
		`SELECT id, name, label, description, data_type, is_required,
		        min_value, max_value, options_json, regex
		 FROM profile_fields
		 WHERE id = ? AND is_active = 1`,
		id,
	).Scan(&out.ID, &out.Name, &out.Label, &out.Description, &out.DataType, &out.IsRequired,
		&minV, &maxV, &out.OptionsJSON, &out.Regex)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("registrationfields: lookup profile field: %w", err)
	}
	if minV.Valid {
		v := int(minV.Int64)
		out.MinValue = &v
	}
	if maxV.Valid {
		v := int(maxV.Int64)
		out.MaxValue = &v
	}
	return &out, nil
}

// ProfileFieldExists is the cheap existence probe the admin
// validator uses — it doesn't need the full row, just "does
// profile_fields.<id> exist and is it active".
func (r *Repository) ProfileFieldExists(id string) (bool, error) {
	var n int
	if err := r.db.QueryRow(
		`SELECT COUNT(1) FROM profile_fields WHERE id = ? AND is_active = 1`, id,
	).Scan(&n); err != nil {
		return false, fmt.Errorf("registrationfields: profile-field exists: %w", err)
	}
	return n > 0, nil
}

// -- helpers ----------------------------------------------------------

type scanner interface {
	Scan(dest ...interface{}) error
}

func scanField(s scanner) (entity.Field, error) {
	var f entity.Field
	var source string
	var profileID, name, label, dataType sql.NullString
	var requiredOverride sql.NullBool
	var minV, maxV sql.NullInt64
	var created, updated sql.NullString

	err := s.Scan(
		&f.ID, &f.ApplicationID, &f.StepID, &f.OrderIndex,
		&source, &profileID, &requiredOverride,
		&name, &label, &f.Description, &dataType, &f.IsRequired,
		&minV, &maxV, &f.OptionsJSON, &f.Regex,
		&created, &updated,
	)
	if err != nil {
		return entity.Field{}, fmt.Errorf("registrationfields: scan field: %w", err)
	}
	f.Source = entity.FieldSource(source)
	if profileID.Valid {
		s := profileID.String
		f.ProfileFieldID = &s
	}
	if requiredOverride.Valid {
		b := requiredOverride.Bool
		f.RequiredOverride = &b
	}
	if name.Valid {
		f.Name = name.String
	}
	if label.Valid {
		f.Label = label.String
	}
	if dataType.Valid {
		f.DataType = dataType.String
	}
	if minV.Valid {
		v := int(minV.Int64)
		f.MinValue = &v
	}
	if maxV.Valid {
		v := int(maxV.Int64)
		f.MaxValue = &v
	}
	if created.Valid {
		if t, perr := parseTime(created.String); perr == nil {
			f.CreatedAt = t
		}
	}
	if updated.Valid {
		if t, perr := parseTime(updated.String); perr == nil {
			f.UpdatedAt = t
		}
	}
	return f, nil
}

func nullableString(s *string) interface{} {
	if s == nil {
		return nil
	}
	return *s
}
func nullableBool(b *bool) interface{} {
	if b == nil {
		return nil
	}
	return *b
}
func nullableInt(i *int) interface{} {
	if i == nil {
		return nil
	}
	return *i
}
func defaultString(s, def string) string {
	if s == "" {
		return def
	}
	return s
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
	return time.Time{}, fmt.Errorf("registrationfields: unparseable timestamp %q", s)
}
