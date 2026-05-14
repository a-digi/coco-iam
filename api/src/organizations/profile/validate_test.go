package profile

import (
	"testing"

	"github.com/a-digi/coco-iam/src/organizations/profile/entity"
)

// Write-time validation coverage for the option-backed field types
// added to the profile schema. Exercised through Validate so the
// top-level required / missing / empty rules are exercised too.

func TestValidate_ChoiceAcceptsAllowedOption(t *testing.T) {
	f := entity.ProfileField{
		Name:     "tier",
		DataType: entity.DataTypeChoice,
		IsActive: true,
		Options:  []string{"bronze", "silver", "gold"},
	}
	cleaned, errs := Validate([]entity.ProfileField{f}, map[string]interface{}{"tier": "silver"})
	if len(errs) != 0 {
		t.Fatalf("want no errors, got %v", errs)
	}
	if cleaned["tier"] != "silver" {
		t.Fatalf("expected silver to pass through, got %v", cleaned["tier"])
	}
}

func TestValidate_ChoiceRejectsUnknownOption(t *testing.T) {
	f := entity.ProfileField{
		Name:     "tier",
		DataType: entity.DataTypeChoice,
		IsActive: true,
		Options:  []string{"bronze", "silver"},
	}
	_, errs := Validate([]entity.ProfileField{f}, map[string]interface{}{"tier": "platinum"})
	if len(errs) != 1 || errs[0].Field != "tier" {
		t.Fatalf("want single tier error, got %v", errs)
	}
}

func TestValidate_MultipleAcceptsSubset(t *testing.T) {
	f := entity.ProfileField{
		Name:     "interests",
		DataType: entity.DataTypeMultiple,
		IsActive: true,
		Options:  []string{"sports", "music", "coding"},
	}
	cleaned, errs := Validate([]entity.ProfileField{f}, map[string]interface{}{
		"interests": []interface{}{"music", "coding"},
	})
	if len(errs) != 0 {
		t.Fatalf("want no errors, got %v", errs)
	}
	got, ok := cleaned["interests"].([]string)
	if !ok || len(got) != 2 || got[0] != "music" || got[1] != "coding" {
		t.Fatalf("unexpected cleaned value: %v", cleaned["interests"])
	}
}

func TestValidate_MultipleDeduplicates(t *testing.T) {
	f := entity.ProfileField{
		Name:     "interests",
		DataType: entity.DataTypeMultiple,
		IsActive: true,
		Options:  []string{"sports", "music"},
	}
	cleaned, errs := Validate([]entity.ProfileField{f}, map[string]interface{}{
		"interests": []interface{}{"music", "music", "sports"},
	})
	if len(errs) != 0 {
		t.Fatalf("want no errors, got %v", errs)
	}
	got := cleaned["interests"].([]string)
	if len(got) != 2 {
		t.Fatalf("duplicates should be removed, got %v", got)
	}
}

func TestValidate_MultipleRejectsUnknownEntry(t *testing.T) {
	f := entity.ProfileField{
		Name:     "interests",
		DataType: entity.DataTypeMultiple,
		IsActive: true,
		Options:  []string{"sports", "music"},
	}
	_, errs := Validate([]entity.ProfileField{f}, map[string]interface{}{
		"interests": []interface{}{"music", "chess"},
	})
	if len(errs) != 1 || errs[0].Field != "interests" {
		t.Fatalf("want single interests error, got %v", errs)
	}
}

func TestValidate_MultipleRejectsNonString(t *testing.T) {
	f := entity.ProfileField{
		Name:     "interests",
		DataType: entity.DataTypeMultiple,
		IsActive: true,
		Options:  []string{"sports"},
	}
	_, errs := Validate([]entity.ProfileField{f}, map[string]interface{}{
		"interests": []interface{}{"sports", 42},
	})
	if len(errs) != 1 || errs[0].Field != "interests" {
		t.Fatalf("want type error on interests, got %v", errs)
	}
}

func TestValidate_MultipleRequiredEmptyArrayRejected(t *testing.T) {
	f := entity.ProfileField{
		Name:       "interests",
		DataType:   entity.DataTypeMultiple,
		IsActive:   true,
		IsRequired: true,
		Options:    []string{"sports"},
	}
	_, errs := Validate([]entity.ProfileField{f}, map[string]interface{}{
		"interests": []interface{}{},
	})
	if len(errs) != 1 || errs[0].Message != "required" {
		t.Fatalf("want required error, got %v", errs)
	}
}

func TestValidate_MultipleOptionalEmptyArrayOK(t *testing.T) {
	f := entity.ProfileField{
		Name:     "interests",
		DataType: entity.DataTypeMultiple,
		IsActive: true,
		Options:  []string{"sports"},
	}
	cleaned, errs := Validate([]entity.ProfileField{f}, map[string]interface{}{
		"interests": []interface{}{},
	})
	if len(errs) != 0 {
		t.Fatalf("want no errors, got %v", errs)
	}
	if _, ok := cleaned["interests"]; ok {
		t.Fatalf("empty optional array should be omitted, got %v", cleaned["interests"])
	}
}

func TestValidate_FileType(t *testing.T) {
	// Guard: the file-type branch stays a pass-through for string
	// asset ids. PATCH merge rejects file-type keys before Validate
	// sees them, but the pass-through protects read-back paths.
	f := entity.ProfileField{Name: "passport", DataType: entity.DataTypeFile, IsActive: true}
	cleaned, errs := Validate([]entity.ProfileField{f}, map[string]interface{}{"passport": "abc123"})
	if len(errs) != 0 {
		t.Fatalf("want no errors, got %v", errs)
	}
	if cleaned["passport"] != "abc123" {
		t.Fatalf("expected asset id to pass through, got %v", cleaned["passport"])
	}
}
