package oauthserver

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/a-digi/coco-iam/src/oauthserver/entity"
)

func revokeForm(extra map[string]string) *http.Request {
	form := url.Values{}
	form.Set("client_id", "cid-1")
	form.Set("client_secret", "secret")
	form.Set("token", "REFRESH-TOKEN-XYZ")
	for k, v := range extra {
		if v == "" {
			form.Del(k)
		} else {
			form.Set(k, v)
		}
	}
	r := httptest.NewRequest(http.MethodPost, "/oauth/revoke", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r
}

func TestRevoke_HappyPathReturns200(t *testing.T) {
	rt := &fakeRefresh{}
	h := &RevokeHandler{
		ApplicationIDFromRequest: func(_ *http.Request) (string, string, error) {
			return "app-1", "org-1", nil
		},
		Clients: &fakeRegistry2{client: sampleClient()},
		Refresh: rt,
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, revokeForm(nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if rt.revokeCalls != 1 {
		t.Errorf("want 1 Revoke call, got %d", rt.revokeCalls)
	}
}

func TestRevoke_UnknownTokenStillReturns200(t *testing.T) {
	// RFC 7009: server returns 200 even for unknown tokens —
	// no enumeration via timing/status differences.
	rt := &fakeRefresh{}
	h := &RevokeHandler{
		ApplicationIDFromRequest: func(_ *http.Request) (string, string, error) {
			return "app-1", "org-1", nil
		},
		Clients: &fakeRegistry2{client: sampleClient()},
		Refresh: rt,
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, revokeForm(map[string]string{"token": "never-issued"}))
	if rec.Code != http.StatusOK {
		t.Errorf("want 200 even for unknown token, got %d", rec.Code)
	}
}

func TestRevoke_RejectsUnknownClient(t *testing.T) {
	h := &RevokeHandler{
		ApplicationIDFromRequest: func(_ *http.Request) (string, string, error) {
			return "app-1", "org-1", nil
		},
		Clients: &fakeRegistry2{err: entity.ErrClientNotFound},
		Refresh: &fakeRefresh{},
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, revokeForm(nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rec.Code)
	}
}

func TestRevoke_MissingTokenReturns400(t *testing.T) {
	h := &RevokeHandler{
		ApplicationIDFromRequest: func(_ *http.Request) (string, string, error) {
			return "app-1", "org-1", nil
		},
		Clients: &fakeRegistry2{client: sampleClient()},
		Refresh: &fakeRefresh{},
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, revokeForm(map[string]string{"token": ""}))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

func TestRevoke_RejectsGet(t *testing.T) {
	h := &RevokeHandler{
		ApplicationIDFromRequest: func(_ *http.Request) (string, string, error) {
			return "app-1", "org-1", nil
		},
		Clients: &fakeRegistry2{client: sampleClient()},
		Refresh: &fakeRefresh{},
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/oauth/revoke", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("want 405, got %d", rec.Code)
	}
}
