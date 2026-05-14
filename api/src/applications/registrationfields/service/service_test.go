package service

import (
	"testing"

	"github.com/a-digi/coco-iam/src/applications/registrationfields/entity"
	"github.com/a-digi/coco-iam/src/applications/registrationfields/repository"
)

// ---------- ResolveEffective ----------

func TestResolveEffective_CustomPassesThrough(t *testing.T) {
	min, max := 5, 20
	f := entity.Field{
		ID: "f-1", StepID: "s-1", Source: entity.FieldSourceCustom,
		Name: "promo", Label: "Promo", DataType: "text",
		IsRequired: true, MinValue: &min, MaxValue: &max,
		OptionsJSON: `["a","b"]`, Regex: "^[A-Z]+$",
	}
	got, ok := ResolveEffective(f, nil)
	if !ok {
		t.Fatal("custom field must resolve")
	}
	if got.Name != "promo" || got.Label != "Promo" || got.DataType != "text" {
		t.Errorf("custom passthrough mismatch: %+v", got)
	}
	if !got.IsRequired {
		t.Error("custom IsRequired should survive")
	}
	if got.MinValue == nil || *got.MinValue != 5 {
		t.Errorf("min_value should survive, got %+v", got.MinValue)
	}
	if got.Source != "custom" {
		t.Errorf("source: want custom, got %q", got.Source)
	}
	if got.ProfileFieldID != "" {
		t.Errorf("profile_field_id should be empty for custom, got %q", got.ProfileFieldID)
	}
	if len(got.Options) != 2 || got.Options[0] != "a" {
		t.Errorf("options decode: got %v", got.Options)
	}
	if got.StepID != "s-1" {
		t.Errorf("step id should survive: %q", got.StepID)
	}
}

func TestResolveEffective_ProfileInheritsFromLinked(t *testing.T) {
	// A profile-sourced row with no overrides must take every
	// definition property from the linked profile_fields row.
	f := entity.Field{
		ID: "f-1", StepID: "s-1", Source: entity.FieldSourceProfile,
		ProfileFieldID: strPtr("pf-1"),
	}
	linked := &repository.ProfileField{
		ID: "pf-1", Name: "country", Label: "Country",
		DataType: "text", IsRequired: false,
		OptionsJSON: `["DE","US"]`,
	}
	got, ok := ResolveEffective(f, linked)
	if !ok {
		t.Fatal("linked profile field must resolve")
	}
	if got.Name != "country" || got.Label != "Country" {
		t.Errorf("inheritance failed: %+v", got)
	}
	if got.Source != "profile" || got.ProfileFieldID != "pf-1" {
		t.Errorf("source tracking: %+v", got)
	}
	if got.IsRequired {
		t.Error("required should inherit false from profile")
	}
	if len(got.Options) != 2 {
		t.Errorf("options should inherit, got %v", got.Options)
	}
}

func TestResolveEffective_RequiredOverrideWinsOverProfile(t *testing.T) {
	// Override bumps a non-required profile field to required on
	// registration — the key "ACL"-ish lever admins asked for.
	override := true
	f := entity.Field{
		ID: "f-1", StepID: "s-1", Source: entity.FieldSourceProfile,
		ProfileFieldID: strPtr("pf-1"), RequiredOverride: &override,
	}
	linked := &repository.ProfileField{
		ID: "pf-1", Name: "phone", Label: "Phone",
		DataType: "text", IsRequired: false,
	}
	got, _ := ResolveEffective(f, linked)
	if !got.IsRequired {
		t.Error("override=true should force is_required=true")
	}
}

func TestResolveEffective_RequiredOverrideCanRelax(t *testing.T) {
	// Symmetric: admin can downgrade a profile-required field to
	// optional on registration (e.g. "address is required once
	// they're onboarded, but let them sign up without it first").
	override := false
	f := entity.Field{
		ID: "f-1", StepID: "s-1", Source: entity.FieldSourceProfile,
		ProfileFieldID: strPtr("pf-1"), RequiredOverride: &override,
	}
	linked := &repository.ProfileField{
		ID: "pf-1", Name: "address", DataType: "text", IsRequired: true,
	}
	got, _ := ResolveEffective(f, linked)
	if got.IsRequired {
		t.Error("override=false should relax is_required to false")
	}
}

func TestResolveEffective_ProfileWithoutLinkedIsOrphan(t *testing.T) {
	// Missing profile_fields row → skip. Returning a half-defined
	// field would mis-render the registration form.
	f := entity.Field{
		ID: "f-1", StepID: "s-1", Source: entity.FieldSourceProfile,
		ProfileFieldID: strPtr("pf-MISSING"),
	}
	_, ok := ResolveEffective(f, nil)
	if ok {
		t.Error("orphan profile-link must resolve to !ok")
	}
}

func TestResolveEffective_UnknownSourceTreatedAsOrphan(t *testing.T) {
	// Defence in depth — repository already blocks invalid source
	// on write, but we don't want a bad hand-edit to crash the
	// serialiser.
	f := entity.Field{ID: "f-1", Source: entity.FieldSource("bogus")}
	_, ok := ResolveEffective(f, nil)
	if ok {
		t.Error("unknown source must be treated as orphan")
	}
}

func TestResolveEffective_ProfileDescriptionInheritsWhenBlank(t *testing.T) {
	// Registration row has no description → fall back to profile's.
	// Non-blank registration description would win (next test).
	f := entity.Field{
		ID: "f-1", Source: entity.FieldSourceProfile,
		ProfileFieldID: strPtr("pf-1"),
	}
	linked := &repository.ProfileField{
		ID: "pf-1", Name: "x", Label: "X", DataType: "text",
		Description: "profile-level help text",
	}
	got, _ := ResolveEffective(f, linked)
	if got.Description != "profile-level help text" {
		t.Errorf("should inherit, got %q", got.Description)
	}
}

func TestResolveEffective_RegistrationDescriptionWinsWhenSet(t *testing.T) {
	f := entity.Field{
		ID: "f-1", Source: entity.FieldSourceProfile,
		ProfileFieldID: strPtr("pf-1"),
		Description:    "registration-specific help",
	}
	linked := &repository.ProfileField{
		ID: "pf-1", Name: "x", Label: "X", DataType: "text",
		Description: "profile-level help text",
	}
	got, _ := ResolveEffective(f, linked)
	if got.Description != "registration-specific help" {
		t.Errorf("registration desc should win, got %q", got.Description)
	}
}

// ---------- GroupIntoSteps ----------

func TestGroupIntoSteps_AllFieldsMatchSteps(t *testing.T) {
	steps := []entity.Step{
		{ID: "s-1", OrderIndex: 0, Title: "A"},
		{ID: "s-2", OrderIndex: 1, Title: "B"},
	}
	fields := []EffectiveField{
		{ID: "f-1", StepID: "s-1"},
		{ID: "f-2", StepID: "s-2"},
		{ID: "f-3", StepID: "s-1"},
	}
	grouped, orphans := GroupIntoSteps(steps, fields)
	if len(orphans) != 0 {
		t.Errorf("no orphans expected, got %v", orphans)
	}
	if len(grouped) != 2 {
		t.Fatalf("want 2 step groups, got %d", len(grouped))
	}
	if len(grouped[0].Fields) != 2 || len(grouped[1].Fields) != 1 {
		t.Errorf("field grouping mismatch: %v, %v",
			fieldIDsOf(grouped[0].Fields), fieldIDsOf(grouped[1].Fields))
	}
}

func TestGroupIntoSteps_OrphanFieldDroppedWithID(t *testing.T) {
	steps := []entity.Step{{ID: "s-1"}}
	fields := []EffectiveField{
		{ID: "f-1", StepID: "s-1"},
		{ID: "f-orphan", StepID: "s-gone"},
	}
	grouped, orphans := GroupIntoSteps(steps, fields)
	if len(orphans) != 1 || orphans[0] != "f-orphan" {
		t.Errorf("orphans: want [f-orphan], got %v", orphans)
	}
	if len(grouped) != 1 || len(grouped[0].Fields) != 1 {
		t.Errorf("non-orphan must survive: %v", grouped)
	}
}

func TestGroupIntoSteps_PreservesStepOrder(t *testing.T) {
	// Steps are returned in the order supplied — the repository
	// already sorts by order_index, so this test just pins that
	// GroupIntoSteps doesn't reshuffle.
	steps := []entity.Step{
		{ID: "s-first", OrderIndex: 0},
		{ID: "s-second", OrderIndex: 1},
		{ID: "s-third", OrderIndex: 2},
	}
	grouped, _ := GroupIntoSteps(steps, nil)
	if len(grouped) != 3 {
		t.Fatalf("want 3 groups, got %d", len(grouped))
	}
	if grouped[0].ID != "s-first" || grouped[2].ID != "s-third" {
		t.Errorf("order not preserved: %v", []string{grouped[0].ID, grouped[1].ID, grouped[2].ID})
	}
}

func TestGroupIntoSteps_EmptyStepsYieldEmptyGroup(t *testing.T) {
	// A step with no fields should still appear in the output
	// (empty fields slice) so consumers render the step header
	// even if it's a "review your info" placeholder.
	steps := []entity.Step{{ID: "s-1", Title: "Review"}}
	grouped, _ := GroupIntoSteps(steps, nil)
	if len(grouped) != 1 {
		t.Fatalf("want 1 group, got %d", len(grouped))
	}
	if grouped[0].Title != "Review" {
		t.Errorf("step title must survive: %q", grouped[0].Title)
	}
	if len(grouped[0].Fields) != 0 {
		t.Errorf("empty fields expected, got %v", grouped[0].Fields)
	}
}

// ---------- helpers ----------

func strPtr(s string) *string { return &s }

func fieldIDsOf(fs []EffectiveField) []string {
	out := make([]string, len(fs))
	for i, f := range fs {
		out[i] = f.ID
	}
	return out
}
