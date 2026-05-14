package me

import (
	"strings"
	"testing"

	"github.com/a-digi/coco-iam/src/admin/users/profile/entity"
)

func strPtr(s string) *string { return &s }

func TestValidatePatchBody_EmptyIsFine(t *testing.T) {
	// A PATCH with no fields is valid — the merge step will just
	// leave everything unchanged. Handler shouldn't reject.
	if err := validatePatchBody(patchBody{}); err != nil {
		t.Errorf("empty patch should validate, got %v", err)
	}
}

func TestValidatePatchBody_ExplicitEmptyStringsAllowed(t *testing.T) {
	// Important: an admin who wants to CLEAR their last name sends
	// `{"last_name": ""}`. That's different from omitting the
	// field. Must validate.
	body := patchBody{
		FirstName: strPtr(""),
		LastName:  strPtr(""),
		Phone:     strPtr(""),
		Locale:    strPtr(""),
		Timezone:  strPtr(""),
	}
	if err := validatePatchBody(body); err != nil {
		t.Errorf("explicit empty strings should validate, got %v", err)
	}
}

func TestValidatePatchBody_RejectsOverlongStrings(t *testing.T) {
	over := strings.Repeat("a", profileFieldMaxLen+1)
	cases := []patchBody{
		{FirstName: &over},
		{LastName: &over},
		{Phone: &over},
	}
	for i, c := range cases {
		if err := validatePatchBody(c); err == nil {
			t.Errorf("case %d: overlong string should be rejected", i)
		}
	}
}

func TestValidatePatchBody_AcceptsExactLimit(t *testing.T) {
	// Off-by-one check — exactly 120 runes is fine; 121 is not.
	// Previous test covers 121; this pins the boundary.
	exact := strings.Repeat("a", profileFieldMaxLen)
	if err := validatePatchBody(patchBody{FirstName: &exact}); err != nil {
		t.Errorf("exact-limit length should be allowed, got %v", err)
	}
}

func TestValidatePatchBody_LocaleShapes(t *testing.T) {
	cases := []struct {
		name, in string
		wantOK   bool
	}{
		{"empty", "", true},
		{"language only", "en", true},
		{"language + region", "en-US", true},
		{"three-letter", "und", true},
		{"script + region", "zh-Hant-HK", true},
		{"bogus — digits in language", "e1-US", false},
		{"bogus — spaces", "en US", false},
		{"bogus — just dash", "-", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validatePatchBody(patchBody{Locale: &tc.in})
			if (err == nil) != tc.wantOK {
				t.Errorf("locale %q: wantOK=%v got err=%v", tc.in, tc.wantOK, err)
			}
		})
	}
}

func TestValidatePatchBody_TimezoneShapes(t *testing.T) {
	cases := []struct {
		name, in string
		wantOK   bool
	}{
		{"empty", "", true},
		{"UTC-style", "UTC", true},
		{"Europe/Berlin", "Europe/Berlin", true},
		{"three-segment", "America/Argentina/Buenos_Aires", true},
		{"bogus — digits", "Europe/Berlin2", false},
		{"bogus — embedded space", "Europe Berlin", false},
		{"bogus — leading slash", "/Europe/Berlin", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validatePatchBody(patchBody{Timezone: &tc.in})
			if (err == nil) != tc.wantOK {
				t.Errorf("timezone %q: wantOK=%v got err=%v", tc.in, tc.wantOK, err)
			}
		})
	}
}

func TestMergePatch_NoCurrentProfile(t *testing.T) {
	// First-time edit: no existing profile row. The merger must
	// produce a profile keyed to the user id with the incoming
	// fields filled in, others empty.
	out := mergePatch(nil, patchBody{
		FirstName: strPtr("Alice"),
		Locale:    strPtr("en-US"),
	}, "user-1")
	if out.AdminUserID != "user-1" || out.FirstName != "Alice" ||
		out.Locale != "en-US" || out.LastName != "" {
		t.Errorf("unexpected merge: %+v", out)
	}
}

func TestMergePatch_PreservesExistingFieldsOnOmission(t *testing.T) {
	// PATCH semantics: fields not present in the body must keep
	// their current value. Pin this — a regression that accidentally
	// substitutes empty strings would wipe an admin's profile.
	current := &entity.AdminUserProfile{
		AdminUserID: "user-1", FirstName: "Alice", LastName: "P",
		Phone: "+49", Locale: "en-US", Timezone: "Europe/Berlin",
		AvatarAssetID: "user-1.png",
	}
	out := mergePatch(current, patchBody{FirstName: strPtr("Alicia")}, "user-1")
	if out.FirstName != "Alicia" {
		t.Errorf("first_name should update: %q", out.FirstName)
	}
	if out.LastName != "P" || out.Phone != "+49" || out.Locale != "en-US" ||
		out.Timezone != "Europe/Berlin" {
		t.Errorf("omitted fields should survive: %+v", out)
	}
	// Avatar is owned by a different endpoint — must not leak in.
	if out.AvatarAssetID != "user-1.png" {
		t.Errorf("avatar asset id should be preserved: %q", out.AvatarAssetID)
	}
}

func TestMergePatch_ExplicitEmptyStringClearsField(t *testing.T) {
	// Admin sending `{"phone": ""}` must clear the phone — the
	// difference between "omit" and "set to empty" is meaningful.
	current := &entity.AdminUserProfile{
		AdminUserID: "user-1", Phone: "+49",
	}
	out := mergePatch(current, patchBody{Phone: strPtr("")}, "user-1")
	if out.Phone != "" {
		t.Errorf("phone should clear to empty, got %q", out.Phone)
	}
}

func TestMergePatch_TrimsWhitespace(t *testing.T) {
	// Leading/trailing whitespace in names or phone is almost
	// always a paste artefact, never meaningful.
	out := mergePatch(nil, patchBody{
		FirstName: strPtr("  Alice  "),
		Phone:     strPtr("\t+49 30 123 \n"),
	}, "user-1")
	if out.FirstName != "Alice" {
		t.Errorf("first_name should trim: %q", out.FirstName)
	}
	if out.Phone != "+49 30 123" {
		t.Errorf("phone should trim: %q", out.Phone)
	}
}
