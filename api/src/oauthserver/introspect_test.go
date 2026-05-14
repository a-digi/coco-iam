package oauthserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/a-digi/coco-iam/src/oauthserver/entity"
)

func introspectForm(extra map[string]string) *http.Request {
	form := url.Values{}
	form.Set("client_id", "cid-1")
	form.Set("client_secret", "secret")
	form.Set("token", "the-token")
	for k, v := range extra {
		if v == "" {
			form.Del(k)
		} else {
			form.Set(k, v)
		}
	}
	r := httptest.NewRequest(http.MethodPost, "/oauth/introspect", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r
}

func TestIntrospect_ActiveAccessToken(t *testing.T) {
	h := &IntrospectHandler{
		ApplicationIDFromRequest: func(_ *http.Request) (string, string, error) {
			return "app-1", "org-1", nil
		},
		Clients:  &fakeRegistry2{client: sampleClient()},
		Refresh:  &fakeRefresh{},
		Verifier: &fakeVerifier{user: "user-1", scopes: []string{"openid", "email"}},
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, introspectForm(nil))
	var out IntrospectionResponse
	json.Unmarshal(rec.Body.Bytes(), &out)
	if !out.Active {
		t.Fatalf("expected active=true, got %+v", out)
	}
	if out.Sub != "user-1" {
		t.Errorf("sub: %q", out.Sub)
	}
	if out.TokenType != "Bearer" {
		t.Errorf("token_type: %q", out.TokenType)
	}
	if !strings.Contains(out.Scope, "openid") {
		t.Errorf("scope: %q", out.Scope)
	}
}

func TestIntrospect_ActiveRefreshToken(t *testing.T) {
	h := &IntrospectHandler{
		ApplicationIDFromRequest: func(_ *http.Request) (string, string, error) {
			return "app-1", "org-1", nil
		},
		Clients: &fakeRegistry2{client: sampleClient()},
		Refresh: &fakeRefresh{findRec: &entity.RefreshToken{
			ClientRowID: "client-row-1", UserID: "user-1",
			Scopes: []string{"openid"},
		}},
		Verifier: &fakeVerifier{}, // will fail VerifyAccessToken
	}
	// hint=refresh_token so we attempt refresh first.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, introspectForm(map[string]string{"token_type_hint": "refresh_token"}))
	var out IntrospectionResponse
	json.Unmarshal(rec.Body.Bytes(), &out)
	if !out.Active {
		t.Fatalf("expected active=true, got %+v", out)
	}
	if out.TokenType != "refresh_token" {
		t.Errorf("token_type: %q", out.TokenType)
	}
}

func TestIntrospect_UnknownTokenInactive(t *testing.T) {
	h := &IntrospectHandler{
		ApplicationIDFromRequest: func(_ *http.Request) (string, string, error) {
			return "app-1", "org-1", nil
		},
		Clients:  &fakeRegistry2{client: sampleClient()},
		Refresh:  &fakeRefresh{findErr: entity.ErrRefreshNotFound},
		Verifier: &fakeVerifier{err: ErrAccessTokenInvalid},
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, introspectForm(nil))
	var out IntrospectionResponse
	json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Active {
		t.Errorf("unknown token must be inactive, got %+v", out)
	}
}

func TestIntrospect_RefreshTokenForDifferentClientReportsInactive(t *testing.T) {
	h := &IntrospectHandler{
		ApplicationIDFromRequest: func(_ *http.Request) (string, string, error) {
			return "app-1", "org-1", nil
		},
		Clients: &fakeRegistry2{client: sampleClient()},
		Refresh: &fakeRefresh{findRec: &entity.RefreshToken{
			ClientRowID: "OTHER-CLIENT", UserID: "user-1",
			Scopes: []string{"openid"},
		}},
		Verifier: &fakeVerifier{err: ErrAccessTokenInvalid},
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, introspectForm(map[string]string{"token_type_hint": "refresh_token"}))
	var out IntrospectionResponse
	json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Active {
		t.Errorf("token belonging to another client should be inactive, got %+v", out)
	}
}

func TestIntrospect_RejectsUnauthenticatedClient(t *testing.T) {
	h := &IntrospectHandler{
		ApplicationIDFromRequest: func(_ *http.Request) (string, string, error) {
			return "app-1", "org-1", nil
		},
		Clients:  &fakeRegistry2{err: entity.ErrClientNotFound},
		Refresh:  &fakeRefresh{},
		Verifier: &fakeVerifier{},
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, introspectForm(nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rec.Code)
	}
}

func TestIntrospect_RejectsGet(t *testing.T) {
	h := &IntrospectHandler{
		ApplicationIDFromRequest: func(_ *http.Request) (string, string, error) {
			return "app-1", "org-1", nil
		},
		Clients:  &fakeRegistry2{client: sampleClient()},
		Refresh:  &fakeRefresh{},
		Verifier: &fakeVerifier{},
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/oauth/introspect", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("want 405, got %d", rec.Code)
	}
}
