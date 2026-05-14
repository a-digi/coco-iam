package repository

import (
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/a-digi/coco-iam/src/applications/registrationfields/entity"
	_ "github.com/mattn/go-sqlite3"
)

// freshRepo opens an in-memory SQLite with the schema the registration
// tables need: profile_fields (so LookupProfileField can be exercised)
// plus the two registration tables. Schema kept inline so a drift from
// the migration files shows up as a test failure.
func freshRepo(t *testing.T) (*Repository, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	schema := `
		CREATE TABLE profile_fields (
			id TEXT NOT NULL PRIMARY KEY,
			name TEXT NOT NULL,
			label TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			data_type TEXT NOT NULL,
			is_required BOOLEAN NOT NULL DEFAULT 0,
			min_value INTEGER,
			max_value INTEGER,
			options_json TEXT NOT NULL DEFAULT '[]',
			regex TEXT NOT NULL DEFAULT '',
			order_index INTEGER NOT NULL DEFAULT 0,
			is_active BOOLEAN NOT NULL DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE application_registration_steps (
			id TEXT NOT NULL PRIMARY KEY,
			application_id TEXT NOT NULL,
			order_index INTEGER NOT NULL DEFAULT 0,
			title TEXT NOT NULL DEFAULT '',
			description TEXT NOT NULL DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE application_registration_fields (
			id TEXT NOT NULL PRIMARY KEY,
			application_id TEXT NOT NULL,
			step_id TEXT NOT NULL,
			order_index INTEGER NOT NULL DEFAULT 0,
			source TEXT NOT NULL,
			profile_field_id TEXT,
			required_override BOOLEAN,
			name TEXT,
			label TEXT,
			description TEXT NOT NULL DEFAULT '',
			data_type TEXT,
			is_required BOOLEAN NOT NULL DEFAULT 0,
			min_value INTEGER,
			max_value INTEGER,
			options_json TEXT NOT NULL DEFAULT '[]',
			regex TEXT NOT NULL DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return New(db), db
}

func TestReplaceForApp_SingleStepRoundTrip(t *testing.T) {
	// The cheapest happy path: one step, one custom field, one
	// profile-sourced field. Covers insert + ListSteps + ListFields.
	repo, rawDB := freshRepo(t)
	seedProfileField(t, rawDB, "pf-1", "first_name", "First name", "text")

	steps := []entity.Step{
		{ID: "step-1", OrderIndex: 0, Title: "Your details"},
	}
	profileID := "pf-1"
	fields := []entity.Field{
		{ID: "f-custom", StepID: "step-1", OrderIndex: 0,
			Source: entity.FieldSourceCustom,
			Name: "promo", Label: "Promo", DataType: "text"},
		{ID: "f-profile", StepID: "step-1", OrderIndex: 1,
			Source: entity.FieldSourceProfile, ProfileFieldID: &profileID},
	}
	if err := repo.ReplaceForApp("app-1", steps, fields); err != nil {
		t.Fatalf("replace: %v", err)
	}

	gotSteps, err := repo.ListStepsForApp("app-1")
	if err != nil {
		t.Fatalf("list steps: %v", err)
	}
	if len(gotSteps) != 1 || gotSteps[0].ID != "step-1" || gotSteps[0].Title != "Your details" {
		t.Errorf("steps: unexpected %+v", gotSteps)
	}

	gotFields, err := repo.ListFieldsForApp("app-1")
	if err != nil {
		t.Fatalf("list fields: %v", err)
	}
	if len(gotFields) != 2 {
		t.Fatalf("fields: want 2, got %d", len(gotFields))
	}
	// Join-through-steps ordering must match OrderIndex ascending.
	if gotFields[0].ID != "f-custom" || gotFields[1].ID != "f-profile" {
		t.Errorf("fields order: %v", []string{gotFields[0].ID, gotFields[1].ID})
	}
	// Profile-linked field should carry the profile_field_id back.
	if gotFields[1].ProfileFieldID == nil || *gotFields[1].ProfileFieldID != "pf-1" {
		t.Errorf("profile_field_id: got %v", gotFields[1].ProfileFieldID)
	}
}

func TestReplaceForApp_MultiStepRespectsStepOrdering(t *testing.T) {
	// Ordering is one of the trickier invariants: ListFieldsForApp
	// joins through steps and orders by (step.order_index,
	// field.order_index). Pin it with two steps whose order doesn't
	// match their insertion order.
	repo, _ := freshRepo(t)
	steps := []entity.Step{
		{ID: "step-second", OrderIndex: 1, Title: "Preferences"},
		{ID: "step-first", OrderIndex: 0, Title: "Details"},
	}
	fields := []entity.Field{
		{ID: "f-a", StepID: "step-second", OrderIndex: 0,
			Source: entity.FieldSourceCustom, Name: "a", Label: "A", DataType: "text"},
		{ID: "f-b", StepID: "step-first", OrderIndex: 0,
			Source: entity.FieldSourceCustom, Name: "b", Label: "B", DataType: "text"},
	}
	if err := repo.ReplaceForApp("app-1", steps, fields); err != nil {
		t.Fatalf("replace: %v", err)
	}

	gotSteps, _ := repo.ListStepsForApp("app-1")
	if gotSteps[0].ID != "step-first" || gotSteps[1].ID != "step-second" {
		t.Errorf("steps should be ordered by order_index: %v", stepIDs(gotSteps))
	}
	gotFields, _ := repo.ListFieldsForApp("app-1")
	if gotFields[0].ID != "f-b" || gotFields[1].ID != "f-a" {
		t.Errorf("fields should be ordered by (step.order_index, field.order_index): %v", fieldIDs(gotFields))
	}
}

func TestReplaceForApp_OrphanStepIDRejected(t *testing.T) {
	// A field that points to a step that wasn't included in the
	// replace call must be rejected BEFORE the transaction touches
	// the DB — otherwise the delete-all step would strand existing
	// data if the insert later failed.
	repo, _ := freshRepo(t)
	steps := []entity.Step{{ID: "step-1", OrderIndex: 0}}
	fields := []entity.Field{
		{ID: "f-orphan", StepID: "step-MISSING", OrderIndex: 0,
			Source: entity.FieldSourceCustom, Name: "x", Label: "X", DataType: "text"},
	}
	err := repo.ReplaceForApp("app-1", steps, fields)
	if err == nil {
		t.Fatal("expected orphan-step-id to be rejected")
	}
	if !strings.Contains(err.Error(), "unknown step") {
		t.Errorf("error message should mention the offending linkage, got %v", err)
	}

	// Pre-existing data — seed, then try a bad replace, then verify
	// data survived (transaction discipline).
	preSteps := []entity.Step{{ID: "pre-step", OrderIndex: 0}}
	preFields := []entity.Field{
		{ID: "pre-field", StepID: "pre-step", OrderIndex: 0,
			Source: entity.FieldSourceCustom, Name: "pre", Label: "Pre", DataType: "text"},
	}
	if err := repo.ReplaceForApp("app-1", preSteps, preFields); err != nil {
		t.Fatalf("pre-seed: %v", err)
	}
	// Bad payload must NOT destroy the existing data.
	if err := repo.ReplaceForApp("app-1", steps, fields); err == nil {
		t.Fatal("expected second bad replace to fail")
	}
	gotFields, _ := repo.ListFieldsForApp("app-1")
	if len(gotFields) != 1 || gotFields[0].ID != "pre-field" {
		t.Errorf("pre-existing data must survive a rejected replace: got %+v", gotFields)
	}
}

func TestReplaceForApp_InvalidSourceRejected(t *testing.T) {
	repo, _ := freshRepo(t)
	steps := []entity.Step{{ID: "s1"}}
	fields := []entity.Field{
		{ID: "f1", StepID: "s1", Source: entity.FieldSource("bogus"),
			Name: "x", Label: "X", DataType: "text"},
	}
	if err := repo.ReplaceForApp("app-1", steps, fields); err == nil {
		t.Fatal("expected invalid-source rejection")
	}
}

func TestReplaceForApp_WipesPreviousDesign(t *testing.T) {
	// A subsequent replace with a different design must fully
	// overwrite — not merge with — the previous one. Catches a
	// class of bug where the delete-before-insert gets dropped.
	repo, _ := freshRepo(t)
	if err := repo.ReplaceForApp("app-1",
		[]entity.Step{{ID: "s1", OrderIndex: 0}},
		[]entity.Field{
			{ID: "old-field", StepID: "s1", Source: entity.FieldSourceCustom,
				Name: "old", Label: "Old", DataType: "text"},
		},
	); err != nil {
		t.Fatalf("first replace: %v", err)
	}
	if err := repo.ReplaceForApp("app-1",
		[]entity.Step{{ID: "s2", OrderIndex: 0}},
		[]entity.Field{
			{ID: "new-field", StepID: "s2", Source: entity.FieldSourceCustom,
				Name: "new", Label: "New", DataType: "text"},
		},
	); err != nil {
		t.Fatalf("second replace: %v", err)
	}
	steps, _ := repo.ListStepsForApp("app-1")
	fields, _ := repo.ListFieldsForApp("app-1")
	if len(steps) != 1 || steps[0].ID != "s2" {
		t.Errorf("steps should be wiped to [s2]: %v", stepIDs(steps))
	}
	if len(fields) != 1 || fields[0].ID != "new-field" {
		t.Errorf("fields should be wiped to [new-field]: %v", fieldIDs(fields))
	}
}

func TestReplaceForApp_DoesNotTouchOtherApps(t *testing.T) {
	// Multi-tenant isolation: a replace for app-A must not touch
	// rows belonging to app-B, even in the same per-org DB.
	repo, _ := freshRepo(t)
	if err := repo.ReplaceForApp("app-A",
		[]entity.Step{{ID: "a-step", OrderIndex: 0}},
		[]entity.Field{
			{ID: "a-field", StepID: "a-step", Source: entity.FieldSourceCustom,
				Name: "a", Label: "A", DataType: "text"},
		},
	); err != nil {
		t.Fatalf("seed A: %v", err)
	}
	if err := repo.ReplaceForApp("app-B",
		[]entity.Step{{ID: "b-step", OrderIndex: 0}},
		[]entity.Field{
			{ID: "b-field", StepID: "b-step", Source: entity.FieldSourceCustom,
				Name: "b", Label: "B", DataType: "text"},
		},
	); err != nil {
		t.Fatalf("seed B: %v", err)
	}
	// Now re-replace app-A. App-B's data must be untouched.
	if err := repo.ReplaceForApp("app-A", []entity.Step{}, []entity.Field{}); err != nil {
		t.Fatalf("wipe A: %v", err)
	}
	bSteps, _ := repo.ListStepsForApp("app-B")
	bFields, _ := repo.ListFieldsForApp("app-B")
	if len(bSteps) != 1 || len(bFields) != 1 {
		t.Errorf("app-B should be untouched: steps=%v fields=%v", stepIDs(bSteps), fieldIDs(bFields))
	}
}

func TestLookupProfileField_HitAndMiss(t *testing.T) {
	repo, rawDB := freshRepo(t)
	seedProfileField(t, rawDB, "pf-1", "country", "Country", "text")

	got, err := repo.LookupProfileField("pf-1")
	if err != nil {
		t.Fatalf("hit: %v", err)
	}
	if got.Name != "country" {
		t.Errorf("want country, got %q", got.Name)
	}

	_, err = repo.LookupProfileField("pf-missing")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound for missing, got %v", err)
	}
}

func TestLookupProfileField_IgnoresInactive(t *testing.T) {
	// Soft-deleted profile fields (is_active = 0) must look like
	// ErrNotFound to the registration layer — we don't want to
	// merge in a field the admin has marked as disabled.
	repo, rawDB := freshRepo(t)
	_, err := rawDB.Exec(
		`INSERT INTO profile_fields
		 (id, name, label, data_type, is_active, options_json, regex)
		 VALUES (?, ?, ?, ?, 0, '[]', '')`,
		"pf-disabled", "retired", "Retired", "text",
	)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	_, err = repo.LookupProfileField("pf-disabled")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("disabled profile field must read as ErrNotFound, got %v", err)
	}
}

func TestProfileFieldExists_ReturnsFalseForInactive(t *testing.T) {
	repo, rawDB := freshRepo(t)
	seedProfileField(t, rawDB, "pf-active", "a", "A", "text")
	_, err := rawDB.Exec(
		`INSERT INTO profile_fields
		 (id, name, label, data_type, is_active, options_json, regex)
		 VALUES (?, ?, ?, ?, 0, '[]', '')`,
		"pf-inactive", "b", "B", "text",
	)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	if ok, _ := repo.ProfileFieldExists("pf-active"); !ok {
		t.Error("active should exist")
	}
	if ok, _ := repo.ProfileFieldExists("pf-inactive"); ok {
		t.Error("inactive should not exist from the registration layer's POV")
	}
	if ok, _ := repo.ProfileFieldExists("pf-missing"); ok {
		t.Error("missing should not exist")
	}
}

// -- helpers ---------------------------------------------------------

func seedProfileField(t *testing.T, db *sql.DB, id, name, label, dataType string) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO profile_fields
		 (id, name, label, description, data_type, is_required, options_json, regex, is_active)
		 VALUES (?, ?, ?, '', ?, 0, '[]', '', 1)`,
		id, name, label, dataType,
	); err != nil {
		t.Fatalf("seed profile field %q: %v", id, err)
	}
}

func stepIDs(steps []entity.Step) []string {
	out := make([]string, len(steps))
	for i, s := range steps {
		out[i] = s.ID
	}
	return out
}

func fieldIDs(fields []entity.Field) []string {
	out := make([]string, len(fields))
	for i, f := range fields {
		out[i] = f.ID
	}
	return out
}

// Exercise the unused import removal trap without adding a real
// use elsewhere: the test file needs `time` only if we add a
// timestamp check, so keep a reference that compiles.
var _ = time.Now
