package userprofile

import (
	"testing"

	"github.com/a-digi/coco-iam/src/organizations/profile/entity"
)

// Pure-function coverage for MergeProfileData. No I/O, no ports —
// every branch the PATCH handler depends on is pinned here so the
// handler test can focus on orchestration.

func mkField(name, dt string, required bool) entity.ProfileField {
	return entity.ProfileField{
		Name:       name,
		DataType:   dt,
		IsRequired: required,
		IsActive:   true,
	}
}

func TestMerge_UnknownKeyRejected(t *testing.T) {
	fields := []entity.ProfileField{mkField("first_name", entity.DataTypeText, false)}
	_, errs := MergeProfileData(fields, nil, map[string]any{"nickname": "bob"})
	if len(errs) != 1 || errs[0].Field != "nickname" {
		t.Fatalf("want single unknown-field error on nickname, got %v", errs)
	}
}

func TestMerge_ExplicitNullClearsOptionalField(t *testing.T) {
	fields := []entity.ProfileField{mkField("first_name", entity.DataTypeText, false)}
	current := map[string]any{"first_name": "Alice"}
	merged, errs := MergeProfileData(fields, current, map[string]any{"first_name": nil})
	if len(errs) != 0 {
		t.Fatalf("want no errors, got %v", errs)
	}
	if _, still := merged["first_name"]; still {
		t.Fatalf("first_name should have been cleared, got %v", merged)
	}
}

func TestMerge_FileTypeKeyRejected(t *testing.T) {
	// The single place the "file fields can't be set via JSON" rule
	// is enforced. If this test fails the PATCH endpoint is
	// trustable only as an admin backdoor for asset ids.
	fields := []entity.ProfileField{mkField("passport", entity.DataTypeFile, false)}
	_, errs := MergeProfileData(fields, nil, map[string]any{"passport": "some-asset-id"})
	if len(errs) != 1 || errs[0].Field != "passport" {
		t.Fatalf("want single passport error, got %v", errs)
	}
}

func TestMerge_TextMinLengthViolation(t *testing.T) {
	min := 5
	fields := []entity.ProfileField{{
		Name: "first_name", DataType: entity.DataTypeText, IsActive: true, MinValue: &min,
	}}
	_, errs := MergeProfileData(fields, nil, map[string]any{"first_name": "bob"})
	if len(errs) != 1 || errs[0].Field != "first_name" {
		t.Fatalf("want min-length violation on first_name, got %v", errs)
	}
}

func TestMerge_NumberDateSelectPathsValidated(t *testing.T) {
	fields := []entity.ProfileField{
		{Name: "age", DataType: entity.DataTypeNumber, IsActive: true},
		{Name: "dob", DataType: entity.DataTypeDate, IsActive: true},
		{Name: "tier", DataType: entity.DataTypeSelect, IsActive: true, Options: []string{"bronze", "silver"}},
	}
	merged, errs := MergeProfileData(fields, nil, map[string]any{
		"age":  42,
		"dob":  "1982-03-14",
		"tier": "silver",
	})
	if len(errs) != 0 {
		t.Fatalf("want no errors, got %v", errs)
	}
	if merged["age"].(float64) != 42 || merged["dob"] != "1982-03-14" || merged["tier"] != "silver" {
		t.Fatalf("merged values wrong: %v", merged)
	}
}

func TestMerge_OmittedKeyKeepsPriorValue(t *testing.T) {
	fields := []entity.ProfileField{
		mkField("first_name", entity.DataTypeText, false),
		mkField("phone", entity.DataTypeText, false),
	}
	current := map[string]any{"first_name": "Alice", "phone": "+49 30"}
	merged, errs := MergeProfileData(fields, current, map[string]any{"first_name": "Alicia"})
	if len(errs) != 0 {
		t.Fatalf("want no errors, got %v", errs)
	}
	if merged["first_name"] != "Alicia" {
		t.Fatalf("first_name should be updated: %v", merged)
	}
	if merged["phone"] != "+49 30" {
		t.Fatalf("phone should be preserved: %v", merged)
	}
}

func TestMerge_EmptyPatchIsNoop(t *testing.T) {
	fields := []entity.ProfileField{mkField("first_name", entity.DataTypeText, false)}
	current := map[string]any{"first_name": "Alice"}
	merged, errs := MergeProfileData(fields, current, map[string]any{})
	if len(errs) != 0 {
		t.Fatalf("want no errors, got %v", errs)
	}
	if merged["first_name"] != "Alice" {
		t.Fatalf("empty patch must not change current: %v", merged)
	}
}

func TestMerge_NilCurrentTreatedAsEmpty(t *testing.T) {
	fields := []entity.ProfileField{mkField("first_name", entity.DataTypeText, false)}
	merged, errs := MergeProfileData(fields, nil, map[string]any{"first_name": "Alice"})
	if len(errs) != 0 {
		t.Fatalf("want no errors, got %v", errs)
	}
	if merged["first_name"] != "Alice" {
		t.Fatalf("nil current should accept first write: %v", merged)
	}
}

func TestMerge_NullOnRequiredFieldRejected(t *testing.T) {
	// Clearing a required field through a PATCH is the same
	// invariant a full save would reject.
	fields := []entity.ProfileField{mkField("first_name", entity.DataTypeText, true)}
	_, errs := MergeProfileData(fields, map[string]any{"first_name": "Alice"},
		map[string]any{"first_name": nil})
	if len(errs) != 1 || errs[0].Message != "required" {
		t.Fatalf("want required error, got %v", errs)
	}
}

func TestMerge_WrongTypeOnKnownKeyRejected(t *testing.T) {
	fields := []entity.ProfileField{mkField("first_name", entity.DataTypeText, false)}
	_, errs := MergeProfileData(fields, nil, map[string]any{"first_name": true})
	if len(errs) != 1 || errs[0].Field != "first_name" {
		t.Fatalf("want type error on first_name, got %v", errs)
	}
}

func TestMerge_ChoiceAndMultipleValidatedThroughPatch(t *testing.T) {
	// Choice + Multiple data types flow through the same Validate
	// pathway the PATCH handler hits, so an invalid submission must
	// produce FieldErrors that the handler turns into 422.
	fields := []entity.ProfileField{
		{Name: "tier", DataType: entity.DataTypeChoice, IsActive: true, Options: []string{"bronze", "silver"}},
		{Name: "hobbies", DataType: entity.DataTypeMultiple, IsActive: true, Options: []string{"music", "sports"}},
	}

	// Valid submission — tier picks one, hobbies picks two of two.
	merged, errs := MergeProfileData(fields, nil, map[string]any{
		"tier":    "silver",
		"hobbies": []any{"music", "sports"},
	})
	if len(errs) != 0 {
		t.Fatalf("valid submission produced errors: %v", errs)
	}
	if merged["tier"] != "silver" {
		t.Errorf("tier merged wrong: %v", merged["tier"])
	}
	hobbies, ok := merged["hobbies"].([]string)
	if !ok || len(hobbies) != 2 {
		t.Errorf("hobbies merged wrong: %v", merged["hobbies"])
	}

	// Invalid tier (not in options) and invalid hobbies entry.
	_, errs = MergeProfileData(fields, nil, map[string]any{
		"tier":    "platinum",
		"hobbies": []any{"music", "chess"},
	})
	if len(errs) != 2 {
		t.Fatalf("want two errors (tier + hobbies), got %v", errs)
	}
}
