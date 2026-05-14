package admin

import (
	"strings"
	"testing"

	"github.com/a-digi/coco-iam/src/applications/registrationfields/entity"
)

// fakeProbe is a stub profileFieldProbe for the validator tests —
// lets us pin "this profile_field_id exists" without touching the
// repository.
type fakeProbe struct {
	known map[string]bool
	err   error
}

func (f fakeProbe) ProfileFieldExists(id string) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	return f.known[id], nil
}

func customField(id, stepID, name string) replaceField {
	return replaceField{
		ID: id, OrderIndex: 0, Source: "custom",
		Name: name, Label: name, DataType: "text",
	}
}

func step(id, title string, fields ...replaceField) replaceStep {
	return replaceStep{ID: id, Title: title, OrderIndex: 0, Fields: fields}
}

func TestValidate_HappyPathSingleStepCustom(t *testing.T) {
	_, fields, err := validateAndBuild(
		replacePayload{Steps: []replaceStep{step("s1", "A", customField("f1", "s1", "first"))}},
		"app-1", fakeProbe{},
	)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(fields) != 1 || fields[0].StepID != "s1" || fields[0].Source != entity.FieldSourceCustom {
		t.Errorf("unexpected fields: %+v", fields)
	}
}

func TestValidate_HappyPathMultiStepMixed(t *testing.T) {
	// Two steps, each with one field of a different source. The
	// profile probe confirms the referenced pf-1 exists.
	profileID := "pf-1"
	payload := replacePayload{Steps: []replaceStep{
		step("s1", "A", customField("f1", "s1", "promo")),
		step("s2", "B", replaceField{
			ID: "f2", OrderIndex: 0, Source: "profile",
			ProfileFieldID: &profileID,
		}),
	}}
	probe := fakeProbe{known: map[string]bool{"pf-1": true}}

	steps, fields, err := validateAndBuild(payload, "app-1", probe)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(steps) != 2 {
		t.Fatalf("want 2 steps, got %d", len(steps))
	}
	if len(fields) != 2 {
		t.Fatalf("want 2 fields, got %d", len(fields))
	}
	if fields[0].StepID != "s1" || fields[1].StepID != "s2" {
		t.Errorf("step linkage mismatch: %+v", fields)
	}
}

func TestValidate_EmptyStepsRejected(t *testing.T) {
	// No steps = no valid registration form. Reject loud rather
	// than silently strip the design and leave the UI confused.
	_, _, err := validateAndBuild(replacePayload{Steps: nil}, "app-1", fakeProbe{})
	if err == nil {
		t.Fatal("empty steps must be rejected")
	}
}

func TestValidate_DuplicateStepIDRejected(t *testing.T) {
	_, _, err := validateAndBuild(
		replacePayload{Steps: []replaceStep{
			step("dup", "A", customField("f1", "dup", "first")),
			step("dup", "B", customField("f2", "dup", "second")),
		}},
		"app-1", fakeProbe{},
	)
	if err == nil || !strings.Contains(err.Error(), "duplicate step id") {
		t.Errorf("want duplicate-step-id error, got %v", err)
	}
}

func TestValidate_DuplicateFieldIDRejected(t *testing.T) {
	_, _, err := validateAndBuild(
		replacePayload{Steps: []replaceStep{
			step("s1", "A",
				customField("f-dup", "s1", "first"),
				customField("f-dup", "s1", "second"),
			),
		}},
		"app-1", fakeProbe{},
	)
	if err == nil || !strings.Contains(err.Error(), "duplicate field id") {
		t.Errorf("want duplicate-field-id error, got %v", err)
	}
}

func TestValidate_DuplicateFieldNameRejected(t *testing.T) {
	// Same `name` on two custom fields, even if they live on
	// different steps — the final submission flattens, so a
	// duplicate name would collide regardless of step grouping.
	_, _, err := validateAndBuild(
		replacePayload{Steps: []replaceStep{
			step("s1", "A", customField("f1", "s1", "first")),
			step("s2", "B", customField("f2", "s2", "first")),
		}},
		"app-1", fakeProbe{},
	)
	if err == nil || !strings.Contains(err.Error(), "duplicate name") {
		t.Errorf("want duplicate-name error, got %v", err)
	}
}

func TestValidate_TwoProfileRefsToSameFieldRejected(t *testing.T) {
	// Two registration rows pointing at the same profile field
	// would submit the same profile value twice. Treat it as a
	// duplicate-name to preserve the "no duplicates" invariant.
	profileID := "pf-shared"
	probe := fakeProbe{known: map[string]bool{"pf-shared": true}}
	_, _, err := validateAndBuild(
		replacePayload{Steps: []replaceStep{
			step("s1", "A", replaceField{ID: "f1", Source: "profile", ProfileFieldID: &profileID}),
			step("s2", "B", replaceField{ID: "f2", Source: "profile", ProfileFieldID: &profileID}),
		}},
		"app-1", probe,
	)
	if err == nil || !strings.Contains(err.Error(), "duplicate name") {
		t.Errorf("want duplicate-name error for shared profile ref, got %v", err)
	}
}

func TestValidate_InvalidSourceRejected(t *testing.T) {
	_, _, err := validateAndBuild(
		replacePayload{Steps: []replaceStep{step("s1", "A", replaceField{
			ID: "f1", Source: "bogus", Name: "x", Label: "X", DataType: "text",
		})}},
		"app-1", fakeProbe{},
	)
	if err == nil || !strings.Contains(err.Error(), "invalid source") {
		t.Errorf("want invalid-source error, got %v", err)
	}
}

func TestValidate_CustomRequiresNameLabelType(t *testing.T) {
	cases := []struct {
		name string
		f    replaceField
	}{
		{"missing name", replaceField{ID: "f1", Source: "custom", Label: "X", DataType: "text"}},
		{"missing label", replaceField{ID: "f1", Source: "custom", Name: "x", DataType: "text"}},
		{"missing data_type", replaceField{ID: "f1", Source: "custom", Name: "x", Label: "X"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := validateAndBuild(
				replacePayload{Steps: []replaceStep{step("s1", "A", tc.f)}},
				"app-1", fakeProbe{},
			)
			if err == nil {
				t.Errorf("%s: expected rejection", tc.name)
			}
		})
	}
}

func TestValidate_ProfileRefRequiresID(t *testing.T) {
	cases := []replaceField{
		{ID: "f1", Source: "profile"},                  // nil profile_field_id
		{ID: "f1", Source: "profile", ProfileFieldID: strPtr("")}, // empty profile_field_id
	}
	for _, f := range cases {
		_, _, err := validateAndBuild(
			replacePayload{Steps: []replaceStep{step("s1", "A", f)}},
			"app-1", fakeProbe{},
		)
		if err == nil {
			t.Errorf("profile source without id should be rejected: %+v", f)
		}
	}
}

func TestValidate_UnknownProfileFieldIDRejected(t *testing.T) {
	profileID := "pf-missing"
	probe := fakeProbe{known: map[string]bool{"pf-real": true}}
	_, _, err := validateAndBuild(
		replacePayload{Steps: []replaceStep{step("s1", "A", replaceField{
			ID: "f1", Source: "profile", ProfileFieldID: &profileID,
		})}},
		"app-1", probe,
	)
	if err == nil || !strings.Contains(err.Error(), "unknown profile_field_id") {
		t.Errorf("want unknown-profile-id error, got %v", err)
	}
}

func TestValidate_MissingStepIDOnFieldStillHandled(t *testing.T) {
	// The on-disk step_id is set by the validator itself (from the
	// enclosing step.ID), so a missing step.ID must be rejected
	// before we generate bad rows.
	_, _, err := validateAndBuild(
		replacePayload{Steps: []replaceStep{
			{ID: "", Title: "A", Fields: []replaceField{customField("f1", "", "x")}},
		}},
		"app-1", fakeProbe{},
	)
	if err == nil || !strings.Contains(err.Error(), "step id") {
		t.Errorf("want step-id-required error, got %v", err)
	}
}

// strPtr builds a *string for table-test readability.
func strPtr(s string) *string { return &s }
