package userprofile

import (
	"testing"

	"github.com/a-digi/coco-iam/src/organizations/profile/entity"
)

func field(name string, order int, opts ...func(*entity.ProfileField)) entity.ProfileField {
	f := entity.ProfileField{
		Name: name, Label: name, DataType: "text",
		OrderIndex: order, IsActive: true,
	}
	for _, o := range opts {
		o(&f)
	}
	return f
}

func inactive(f *entity.ProfileField) { f.IsActive = false }

func TestBuildResponse_EmptyFieldsProducesEmptyList(t *testing.T) {
	out := BuildResponse(nil, nil)
	if len(out) != 0 {
		t.Errorf("want empty, got %v", out)
	}
}

func TestBuildResponse_FieldWithValueRoundTrip(t *testing.T) {
	fields := []entity.ProfileField{field("first_name", 0)}
	data := map[string]interface{}{"first_name": "Alice"}

	out := BuildResponse(fields, data)
	if len(out) != 1 {
		t.Fatalf("want 1 entry, got %d", len(out))
	}
	if got, ok := out[0].Value.(string); !ok || got != "Alice" {
		t.Errorf("value round-trip: got %v", out[0].Value)
	}
}

func TestBuildResponse_MissingValueRendersAsNil(t *testing.T) {
	// Pin the "value: null" contract — the consumer distinguishes
	// "not yet filled in" from "empty string" based on this.
	fields := []entity.ProfileField{field("phone", 0)}
	out := BuildResponse(fields, map[string]interface{}{})
	if out[0].Value != nil {
		t.Errorf("missing value should render as nil, got %v", out[0].Value)
	}
}

func TestBuildResponse_InactiveFieldsAreOmitted(t *testing.T) {
	// Admins retiring a field — the retired field must disappear
	// from the /me response even if the user has a leftover value
	// for it in profile_data. Otherwise users see fields they
	// can't edit anywhere.
	fields := []entity.ProfileField{
		field("first_name", 0),
		field("retired_field", 1, inactive),
		field("last_name", 2),
	}
	data := map[string]interface{}{
		"first_name":    "Alice",
		"retired_field": "ghost",
		"last_name":     "Parker",
	}
	out := BuildResponse(fields, data)
	names := []string{out[0].Name, out[len(out)-1].Name}
	for _, n := range names {
		if n == "retired_field" {
			t.Errorf("inactive field should be filtered out, got %v", names)
		}
	}
	if len(out) != 2 {
		t.Errorf("want 2 active fields, got %d", len(out))
	}
}

func TestBuildResponse_SortsByOrderIndex(t *testing.T) {
	// Pin the ordering contract. The repository returns rows in
	// order_index asc, but the merger must stay correct even if
	// fed out-of-order input.
	fields := []entity.ProfileField{
		field("third", 2),
		field("first", 0),
		field("second", 1),
	}
	out := BuildResponse(fields, nil)
	if out[0].Name != "first" || out[1].Name != "second" || out[2].Name != "third" {
		t.Errorf("ordering broken: %v",
			[]string{out[0].Name, out[1].Name, out[2].Name})
	}
}

func TestBuildResponse_TieBreakByNameIsDeterministic(t *testing.T) {
	// Two fields with the same order_index must come out in a
	// deterministic order across calls so diffs and UI tests don't
	// flap. Name ascending is our tie-break.
	fields := []entity.ProfileField{
		field("zulu", 5),
		field("alpha", 5),
	}
	out := BuildResponse(fields, nil)
	if out[0].Name != "alpha" || out[1].Name != "zulu" {
		t.Errorf("tie-break: %v", []string{out[0].Name, out[1].Name})
	}
}

func TestBuildResponse_NonStringValuesPassThrough(t *testing.T) {
	// number, boolean, etc. must round-trip verbatim so consumers
	// can rely on the JSON type.
	fields := []entity.ProfileField{
		field("age", 0, func(f *entity.ProfileField) { f.DataType = "number" }),
		field("opt_in", 1, func(f *entity.ProfileField) { f.DataType = "select" }),
	}
	data := map[string]interface{}{
		"age":    42.0,
		"opt_in": "yes",
	}
	out := BuildResponse(fields, data)
	if got, ok := out[0].Value.(float64); !ok || got != 42.0 {
		t.Errorf("number round-trip: got %v", out[0].Value)
	}
	if got, ok := out[1].Value.(string); !ok || got != "yes" {
		t.Errorf("select round-trip: got %v", out[1].Value)
	}
}

func TestBuildResponse_NilDataTreatedAsEmpty(t *testing.T) {
	// A brand-new user who's never saved anything has a nil
	// profile_data map. Handler must not panic — the helper is
	// the one that guarantees nil-safety.
	fields := []entity.ProfileField{field("first_name", 0)}
	out := BuildResponse(fields, nil)
	if len(out) != 1 || out[0].Value != nil {
		t.Errorf("nil data: want one entry with nil value, got %+v", out)
	}
}
