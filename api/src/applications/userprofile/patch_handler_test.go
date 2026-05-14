package userprofile

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	profile_entity "github.com/a-digi/coco-iam/src/organizations/profile/entity"
	"github.com/a-digi/coco-server/server/request"
)

// Orchestration coverage for PatchMeHandler. Pure helpers
// (MergeProfileData) are covered in merge_test.go; these tests pin
// that the handler stitches them together correctly and never
// writes on an unvalidated input.

type countingWriter struct {
	calls []writerCall
	err   error
}

type writerCall struct {
	orgID, userID, fieldName string
	value                    any
}

func (w *countingWriter) UpdateFieldValue(orgID, userID, fieldName string, value any) error {
	w.calls = append(w.calls, writerCall{orgID, userID, fieldName, value})
	return w.err
}

func servePatch(h *PatchMeHandler, path, auth, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPatch, path, bytes.NewBufferString(body))
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	rec := httptest.NewRecorder()
	reqCtx := request.NewContext(rec, req, nil)
	h.ServeHTTP(reqCtx)
	return rec
}

func TestPatch_MissingBearerReturns401AndDoesNotWrite(t *testing.T) {
	writer := &countingWriter{}
	h := &PatchMeHandler{
		Slugs:    &fakeSlugResolver{appID: "app-1", orgID: "org-1"},
		Keys:     &fakeKeyLoader{},
		Users:    &fakeUserOrgReader{},
		Profiles: &fakeProfileReader{},
		Writer:   writer,
	}
	rec := servePatch(h, "/a/acme/prod/web/profile/me", "",
		`{"profile_data": {"first_name": "Alice"}}`)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rec.Code)
	}
	if len(writer.calls) != 0 {
		t.Errorf("writer must not be called on auth error, got %d", len(writer.calls))
	}
}

func TestPatch_SlugResolverErrorReturns401(t *testing.T) {
	writer := &countingWriter{}
	h := &PatchMeHandler{
		Slugs:    &fakeSlugResolver{err: errors.New("no such")},
		Keys:     &fakeKeyLoader{},
		Users:    &fakeUserOrgReader{},
		Profiles: &fakeProfileReader{},
		Writer:   writer,
	}
	rec := servePatch(h, "/a/acme/prod/web/profile/me", "Bearer anything",
		`{"profile_data": {}}`)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rec.Code)
	}
	if len(writer.calls) != 0 {
		t.Errorf("writer called despite slug error")
	}
}

func TestPatch_MergeRejectionReturns422(t *testing.T) {
	key := testKey(t)
	writer := &countingWriter{}
	h := &PatchMeHandler{
		Slugs: &fakeSlugResolver{appID: "app-1", orgID: "org-1"},
		Keys:  &fakeKeyLoader{pub: &key.PublicKey},
		Users: &fakeUserOrgReader{mapping: map[string]string{"user-1": "org-1"}},
		Profiles: &fakeProfileReader{
			fields: []profile_entity.ProfileField{
				{Name: "first_name", DataType: profile_entity.DataTypeText, IsActive: true},
			},
		},
		Writer: writer,
	}
	rec := servePatch(h, "/a/acme/prod/web/profile/me",
		signBearer(t, key, "user-1"),
		`{"profile_data": {"nickname": "bob"}}`) // unknown key
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("want 422, got %d body=%s", rec.Code, rec.Body.String())
	}
	if len(writer.calls) != 0 {
		t.Errorf("writer must not be called on validation error")
	}
}

func TestPatch_WriterErrorReturns500(t *testing.T) {
	key := testKey(t)
	writer := &countingWriter{err: errors.New("disk full")}
	h := &PatchMeHandler{
		Slugs: &fakeSlugResolver{appID: "app-1", orgID: "org-1"},
		Keys:  &fakeKeyLoader{pub: &key.PublicKey},
		Users: &fakeUserOrgReader{mapping: map[string]string{"user-1": "org-1"}},
		Profiles: &fakeProfileReader{
			fields: []profile_entity.ProfileField{
				{Name: "first_name", DataType: profile_entity.DataTypeText, IsActive: true},
			},
		},
		Writer: writer,
	}
	rec := servePatch(h, "/a/acme/prod/web/profile/me",
		signBearer(t, key, "user-1"),
		`{"profile_data": {"first_name": "Alice"}}`)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

func TestPatch_HappyPath(t *testing.T) {
	key := testKey(t)
	writer := &countingWriter{}
	h := &PatchMeHandler{
		Slugs: &fakeSlugResolver{appID: "app-1", orgID: "org-1"},
		Keys:  &fakeKeyLoader{pub: &key.PublicKey},
		Users: &fakeUserOrgReader{mapping: map[string]string{"user-1": "org-1"}},
		Profiles: &fakeProfileReader{
			fields: []profile_entity.ProfileField{
				{Name: "first_name", DataType: profile_entity.DataTypeText, IsActive: true},
				{Name: "phone", DataType: profile_entity.DataTypeText, IsActive: true},
			},
			data: map[string]interface{}{"first_name": "Old"},
		},
		Writer: writer,
	}
	rec := servePatch(h, "/a/acme/prod/web/profile/me",
		signBearer(t, key, "user-1"),
		`{"profile_data": {"first_name": "Alice", "phone": "+49 30"}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if len(writer.calls) != 2 {
		t.Fatalf("want 2 writer calls, got %d: %+v", len(writer.calls), writer.calls)
	}
	found := map[string]any{}
	for _, c := range writer.calls {
		if c.orgID != "org-1" || c.userID != "user-1" {
			t.Errorf("wrong scope on writer call: %+v", c)
		}
		found[c.fieldName] = c.value
	}
	if found["first_name"] != "Alice" || found["phone"] != "+49 30" {
		t.Errorf("writer didn't see merged values: %v", found)
	}
}

func TestPatch_ExplicitNullClearsField(t *testing.T) {
	key := testKey(t)
	writer := &countingWriter{}
	h := &PatchMeHandler{
		Slugs: &fakeSlugResolver{appID: "app-1", orgID: "org-1"},
		Keys:  &fakeKeyLoader{pub: &key.PublicKey},
		Users: &fakeUserOrgReader{mapping: map[string]string{"user-1": "org-1"}},
		Profiles: &fakeProfileReader{
			fields: []profile_entity.ProfileField{
				{Name: "phone", DataType: profile_entity.DataTypeText, IsActive: true},
			},
			data: map[string]interface{}{"phone": "+49 30"},
		},
		Writer: writer,
	}
	rec := servePatch(h, "/a/acme/prod/web/profile/me",
		signBearer(t, key, "user-1"),
		`{"profile_data": {"phone": null}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if len(writer.calls) != 1 {
		t.Fatalf("want 1 writer call, got %d", len(writer.calls))
	}
	if writer.calls[0].fieldName != "phone" || writer.calls[0].value != nil {
		t.Errorf("writer should see (phone, nil); got %+v", writer.calls[0])
	}
}
