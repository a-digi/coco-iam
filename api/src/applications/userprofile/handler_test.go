package userprofile

import (
	"crypto/rsa"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	profile_entity "github.com/a-digi/coco-iam/src/organizations/profile/entity"
	"github.com/a-digi/coco-server/server/request"
	jwtv5 "github.com/golang-jwt/jwt/v5"
)

// ------- fakes for the four narrow ports ----------------------------

type fakeSlugResolver struct {
	appID, orgID string
	err          error
}

func (f *fakeSlugResolver) ResolveSlugs(_, _, _ string) (string, string, error) {
	return f.appID, f.orgID, f.err
}

type fakeKeyLoader struct {
	pub *rsa.PublicKey
	err error
}

func (f *fakeKeyLoader) LoadPublicKey(_, _ string) (*rsa.PublicKey, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.pub, nil
}

type fakeUserOrgReader struct {
	mapping  map[string]string
	errOnMiss error
}

func (f *fakeUserOrgReader) UserOrg(userID string) (string, error) {
	if orgID, ok := f.mapping[userID]; ok {
		return orgID, nil
	}
	if f.errOnMiss != nil {
		return "", f.errOnMiss
	}
	return "", ErrUserNotFound
}

type fakeProfileReader struct {
	fields []profile_entity.ProfileField
	data   map[string]interface{}
	err    error
}

func (f *fakeProfileReader) LoadProfile(_, _ string) ([]profile_entity.ProfileField, map[string]interface{}, error) {
	return f.fields, f.data, f.err
}

// ------- request helpers --------------------------------------------

// serve invokes the handler against a constructed request and
// returns the recorder. No real DI is required — the handler
// doesn't touch it.
func serve(h *GetMeHandler, method, path, authHeader string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	rec := httptest.NewRecorder()
	reqCtx := request.NewContext(rec, req, nil)
	h.ServeHTTP(reqCtx)
	return rec
}

// signBearer returns an `Authorization: Bearer …` header for an
// RS256 token with the given subject + kid=test-kid. Tests pass
// the matching public key into fakeKeyLoader.
func signBearer(t *testing.T, key *rsa.PrivateKey, sub string) string {
	t.Helper()
	tok := jwtv5.NewWithClaims(jwtv5.SigningMethodRS256, jwtv5.MapClaims{
		"sub":   sub,
		"scope": "user:me",
		"exp":   float64(time.Now().Add(time.Hour).Unix()),
	})
	tok.Header["kid"] = "test-kid"
	signed, err := tok.SignedString(key)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return "Bearer " + signed
}

// ------- tests ------------------------------------------------------

func TestHandler_BadSlugShapeRejects(t *testing.T) {
	// URL doesn't match /a/<org>/<ws>/<app>/… → parseSlugSegments
	// returns ok=false → 401. Collaborators aren't even called.
	key := testKey(t)
	slugs := &fakeSlugResolver{appID: "app-1", orgID: "org-1"}
	h := &GetMeHandler{
		Slugs:    slugs,
		Keys:     &fakeKeyLoader{pub: &key.PublicKey},
		Users:    &fakeUserOrgReader{},
		Profiles: &fakeProfileReader{},
	}
	rec := serve(h, http.MethodGet, "/some/other/path", signBearer(t, key, "user-1"))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("bad slug shape: want 401, got %d", rec.Code)
	}
	if slugs.appID == "" { /* no-op — pinning we constructed the fake */
	}
}

func TestHandler_SlugResolverErrorRejects(t *testing.T) {
	// Unknown org/ws/app combination. The handler must collapse
	// this to 401 — no distinguishing body — so the endpoint
	// can't be used to enumerate which slug triples exist.
	h := &GetMeHandler{
		Slugs:    &fakeSlugResolver{err: errors.New("not found")},
		Keys:     &fakeKeyLoader{},
		Users:    &fakeUserOrgReader{},
		Profiles: &fakeProfileReader{},
	}
	rec := serve(h, http.MethodGet, "/a/acme/prod/web/profile/me",
		"Bearer whatever")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("slug resolver error: want 401, got %d", rec.Code)
	}
}

func TestHandler_AuthErrorShortCircuits(t *testing.T) {
	// No bearer → authenticateUser returns ReasonMissingHeader →
	// handler 401s without touching the ProfileReader. Pins that
	// the short-circuit works end-to-end through the orchestrator.
	calls := 0
	h := &GetMeHandler{
		Slugs: &fakeSlugResolver{appID: "app-1", orgID: "org-1"},
		Keys:  &fakeKeyLoader{},
		Users: &fakeUserOrgReader{},
		Profiles: &fakeProfileReaderCounting{
			inner: &fakeProfileReader{},
			calls: &calls,
		},
	}
	rec := serve(h, http.MethodGet, "/a/acme/prod/web/profile/me", "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("missing header: want 401, got %d", rec.Code)
	}
	if calls != 0 {
		t.Errorf("profile reader should not be called on auth error, got %d calls", calls)
	}
}

func TestHandler_ProfileReaderErrorIs500(t *testing.T) {
	// Auth succeeded; DB / registry failure must surface as 500,
	// not 401 — the admin can tell from the status whether it's
	// a client issue or an infra issue.
	key := testKey(t)
	h := &GetMeHandler{
		Slugs: &fakeSlugResolver{appID: "app-1", orgID: "org-1"},
		Keys:  &fakeKeyLoader{pub: &key.PublicKey},
		Users: &fakeUserOrgReader{mapping: map[string]string{"user-1": "org-1"}},
		Profiles: &fakeProfileReader{
			err: errors.New("profile store down"),
		},
	}
	rec := serve(h, http.MethodGet, "/a/acme/prod/web/profile/me",
		signBearer(t, key, "user-1"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("profile error: want 500, got %d", rec.Code)
	}
}

func TestHandler_HappyPath(t *testing.T) {
	// Full green flow. Valid bearer + known user + active fields
	// + some values → 200 with the expected wire shape.
	key := testKey(t)
	minv := 0
	h := &GetMeHandler{
		Slugs: &fakeSlugResolver{appID: "app-1", orgID: "org-1"},
		Keys:  &fakeKeyLoader{pub: &key.PublicKey},
		Users: &fakeUserOrgReader{mapping: map[string]string{"user-1": "org-1"}},
		Profiles: &fakeProfileReader{
			fields: []profile_entity.ProfileField{
				{
					ID: "pf-1", Name: "first_name", Label: "First name",
					DataType: "text", OrderIndex: 0, IsActive: true,
					MinValue: &minv,
				},
				{
					ID: "pf-2", Name: "retired", Label: "Retired",
					DataType: "text", OrderIndex: 1, IsActive: false,
				},
				{
					ID: "pf-3", Name: "phone", Label: "Phone",
					DataType: "text", OrderIndex: 2, IsActive: true,
				},
			},
			data: map[string]interface{}{"first_name": "Alice"},
		},
	}

	rec := serve(h, http.MethodGet, "/a/acme/prod/web/profile/me",
		signBearer(t, key, "user-1"))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var body struct {
		Message meResponse `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v (body=%s)", err, rec.Body.String())
	}
	got := body.Message.Fields
	if len(got) != 2 {
		t.Fatalf("want 2 active fields (retired filtered out), got %d: %+v", len(got), got)
	}
	// Ordering: first_name (0) → phone (2).
	if got[0].Name != "first_name" || got[1].Name != "phone" {
		t.Errorf("ordering broken: %v",
			[]string{got[0].Name, got[1].Name})
	}
	// first_name has a value; phone is unset → null.
	if v, ok := got[0].Value.(string); !ok || v != "Alice" {
		t.Errorf("first_name value: got %v", got[0].Value)
	}
	if got[1].Value != nil {
		t.Errorf("phone value: want nil, got %v", got[1].Value)
	}
	// The retired field must not leak through.
	for _, f := range got {
		if f.Name == "retired" {
			t.Errorf("inactive field leaked into response: %+v", f)
		}
	}
	// Response body never contains the bare word "unauthorized"
	// on a happy path — cheap guard against an accidental
	// double-write that combined success + error branches.
	if strings.Contains(rec.Body.String(), "unauthorized") {
		t.Errorf("response leaked auth-error text on happy path: %s", rec.Body.String())
	}
}

// fakeProfileReaderCounting counts how many times LoadProfile was
// called so a test can assert "never called on short-circuit".
type fakeProfileReaderCounting struct {
	inner *fakeProfileReader
	calls *int
}

func (f *fakeProfileReaderCounting) LoadProfile(orgID, userID string) ([]profile_entity.ProfileField, map[string]interface{}, error) {
	*f.calls++
	return f.inner.LoadProfile(orgID, userID)
}
