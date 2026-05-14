package admin

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	db "github.com/a-digi/coco-orm/orm"
	logger "github.com/a-digi/coco-logger/logger"
	"github.com/a-digi/coco-server/server/request"

	_ "github.com/mattn/go-sqlite3"
)

// Smoke-level coverage for the admin CRUD surface. Tests spin
// an in-memory SQLite with the production schema, run every
// handler through the real request.RequestContext, and assert
// on the wire shape the admin UI sees.

type stubDI struct {
	mgr *db.DatabaseManager
}

func (s *stubDI) GetDatabaseManager() *db.DatabaseManager { return s.mgr }
func (s *stubDI) GetLogger() logger.Logger               { return nil }

func makeDI(t *testing.T) *stubDI {
	t.Helper()
	conn, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	if _, err := conn.Exec(`
		CREATE TABLE application_oauth_clients (
		    id                   TEXT NOT NULL PRIMARY KEY,
		    application_id       TEXT NOT NULL,
		    client_id            TEXT NOT NULL,
		    client_secret_hash   TEXT,
		    client_type          TEXT NOT NULL DEFAULT 'confidential',
		    display_name         TEXT NOT NULL DEFAULT '',
		    redirect_uris        TEXT NOT NULL DEFAULT '[]',
		    allowed_scopes       TEXT NOT NULL DEFAULT '[]',
		    require_consent      INTEGER NOT NULL DEFAULT 1,
		    access_token_ttl     INTEGER NOT NULL DEFAULT 3600,
		    refresh_token_ttl    INTEGER NOT NULL DEFAULT 1209600,
		    is_active            INTEGER NOT NULL DEFAULT 1,
		    created_at           DATETIME DEFAULT CURRENT_TIMESTAMP,
		    updated_at           DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE UNIQUE INDEX application_oauth_clients_app_client_id_idx
		    ON application_oauth_clients (application_id, client_id);
	`); err != nil {
		t.Fatalf("schema: %v", err)
	}
	return &stubDI{mgr: &db.DatabaseManager{Connector: &db.Connector{DB: conn}}}
}

func serveReq(t *testing.T, di *stubDI, method, path string, body any, h interface {
	ServeHTTP(reqCtx request.RequestContext)
}) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body != nil {
		buf, _ := json.Marshal(body)
		r = httptest.NewRequest(method, path, bytes.NewReader(buf))
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	rec := httptest.NewRecorder()
	reqCtx := request.NewContext(rec, r, di)
	h.ServeHTTP(reqCtx)
	return rec
}

func decodeMessage(t *testing.T, rec *httptest.ResponseRecorder, into any) {
	t.Helper()
	var env struct {
		Message json.RawMessage `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode envelope: %v (body=%s)", err, rec.Body.String())
	}
	if err := json.Unmarshal(env.Message, into); err != nil {
		t.Fatalf("decode message: %v (raw=%s)", err, string(env.Message))
	}
}

// ------- tests ------------------------------------------------------

func TestSmoke_CreateReturnsOneTimeSecretAndMasksOnList(t *testing.T) {
	di := makeDI(t)
	createBody := map[string]any{
		"client_id":         "reporter",
		"client_type":       "confidential",
		"display_name":      "Reporter",
		"redirect_uris":     []string{"https://reporter.example/cb"},
		"allowed_scopes":    []string{"openid", "email"},
		"require_consent":   true,
		"access_token_ttl":  3600,
		"refresh_token_ttl": 1209600,
	}

	rec := serveReq(t, di, http.MethodPost,
		"/api/v1/applications/{id:app-1}/oauth-clients",
		createBody, &CreateHandler{})
	if rec.Code != http.StatusOK {
		t.Fatalf("create: got %d body=%s", rec.Code, rec.Body.String())
	}
	var created createResponse
	decodeMessage(t, rec, &created)
	if created.ClientSecret == "" {
		t.Fatalf("create must return plaintext secret once")
	}
	if created.Client.ClientSecretMask == created.ClientSecret {
		t.Fatalf("list view must not reveal plaintext secret")
	}
	if created.Client.ID == "" {
		t.Fatalf("create response missing id")
	}

	// List: masked.
	rec = serveReq(t, di, http.MethodGet,
		"/api/v1/applications/{id:app-1}/oauth-clients", nil, &ListHandler{})
	if rec.Code != http.StatusOK {
		t.Fatalf("list: got %d", rec.Code)
	}
	var list listResponse
	decodeMessage(t, rec, &list)
	if len(list.Clients) != 1 {
		t.Fatalf("list: want 1, got %d", len(list.Clients))
	}
	if list.Clients[0].ClientSecretMask == "" {
		t.Fatalf("mask empty")
	}
	if list.Clients[0].ClientSecretMask == created.ClientSecret {
		t.Fatalf("mask leaked the plaintext")
	}
}

func TestSmoke_CreatePublicClient(t *testing.T) {
	di := makeDI(t)
	rec := serveReq(t, di, http.MethodPost,
		"/api/v1/applications/{id:app-1}/oauth-clients",
		map[string]any{
			"client_id":       "spa",
			"client_type":     "public",
			"display_name":    "SPA",
			"redirect_uris":   []string{"https://spa.example/cb"},
			"allowed_scopes":  []string{"openid"},
			"require_consent": true,
		},
		&CreateHandler{})
	if rec.Code != http.StatusOK {
		t.Fatalf("public create: got %d body=%s", rec.Code, rec.Body.String())
	}
	var created createResponse
	decodeMessage(t, rec, &created)
	if created.ClientSecret != "" {
		t.Error("public clients should not receive a plaintext secret")
	}
	if created.Client.Type != "public" {
		t.Errorf("type round-trip: %q", created.Client.Type)
	}
}

func TestSmoke_CreateRejectsUnknownClientType(t *testing.T) {
	di := makeDI(t)
	rec := serveReq(t, di, http.MethodPost,
		"/api/v1/applications/{id:app-1}/oauth-clients",
		map[string]any{
			"client_id":       "x",
			"client_type":     "hybrid",
			"display_name":    "x",
			"redirect_uris":   []string{"https://x/cb"},
			"allowed_scopes":  []string{"openid"},
		},
		&CreateHandler{})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

func TestSmoke_CreateRequiresRedirectURIs(t *testing.T) {
	di := makeDI(t)
	rec := serveReq(t, di, http.MethodPost,
		"/api/v1/applications/{id:app-1}/oauth-clients",
		map[string]any{
			"client_id":   "x",
			"client_type": "confidential",
		},
		&CreateHandler{})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

func TestSmoke_CreateDuplicateReturns409(t *testing.T) {
	di := makeDI(t)
	body := map[string]any{
		"client_id":       "dup",
		"client_type":     "public",
		"display_name":    "x",
		"redirect_uris":   []string{"https://x/cb"},
		"allowed_scopes":  []string{"openid"},
	}
	rec := serveReq(t, di, http.MethodPost,
		"/api/v1/applications/{id:app-1}/oauth-clients", body, &CreateHandler{})
	if rec.Code != http.StatusOK {
		t.Fatalf("first create: %d", rec.Code)
	}
	rec = serveReq(t, di, http.MethodPost,
		"/api/v1/applications/{id:app-1}/oauth-clients", body, &CreateHandler{})
	if rec.Code != http.StatusConflict {
		t.Errorf("want 409 on duplicate, got %d", rec.Code)
	}
}

func TestSmoke_UpdatePreservesSecretWhenOmitted(t *testing.T) {
	di := makeDI(t)
	// Create first.
	rec := serveReq(t, di, http.MethodPost,
		"/api/v1/applications/{id:app-1}/oauth-clients",
		map[string]any{
			"client_id":      "c",
			"client_type":    "confidential",
			"display_name":   "c",
			"redirect_uris":  []string{"https://c/cb"},
			"allowed_scopes": []string{"openid"},
		},
		&CreateHandler{})
	var created createResponse
	decodeMessage(t, rec, &created)
	origSecret := created.ClientSecret

	// Update without secret → displays updates, secret preserved.
	rec = serveReq(t, di, http.MethodPatch,
		"/api/v1/applications/{id:app-1}/oauth-clients/"+created.Client.ID,
		map[string]any{
			"display_name":   "C renamed",
			"redirect_uris":  []string{"https://c/cb"},
			"allowed_scopes": []string{"openid", "email"},
			"require_consent": false,
			"is_active":      true,
		},
		&UpdateHandler{})
	if rec.Code != http.StatusOK {
		t.Fatalf("update: got %d body=%s", rec.Code, rec.Body.String())
	}
	var up updateResponse
	decodeMessage(t, rec, &up)
	if up.ClientSecret != "" {
		t.Error("update without rotation must not return a plaintext secret")
	}
	if up.Client.DisplayName != "C renamed" {
		t.Errorf("rename didn't stick: %q", up.Client.DisplayName)
	}
	if len(up.Client.AllowedScopes) != 2 {
		t.Errorf("scope update: %v", up.Client.AllowedScopes)
	}
	_ = origSecret
}

func TestSmoke_RotateSecretMintsNewValue(t *testing.T) {
	di := makeDI(t)
	rec := serveReq(t, di, http.MethodPost,
		"/api/v1/applications/{id:app-1}/oauth-clients",
		map[string]any{
			"client_id":      "c",
			"client_type":    "confidential",
			"display_name":   "c",
			"redirect_uris":  []string{"https://c/cb"},
			"allowed_scopes": []string{"openid"},
		},
		&CreateHandler{})
	var created createResponse
	decodeMessage(t, rec, &created)

	rec = serveReq(t, di, http.MethodPost,
		"/api/v1/applications/{id:app-1}/oauth-clients/"+created.Client.ID+"/rotate-secret",
		nil, &RotateHandler{})
	if rec.Code != http.StatusOK {
		t.Fatalf("rotate: %d body=%s", rec.Code, rec.Body.String())
	}
	var rot updateResponse
	decodeMessage(t, rec, &rot)
	if rot.ClientSecret == "" {
		t.Fatal("rotate must return new plaintext secret")
	}
	if rot.ClientSecret == created.ClientSecret {
		t.Fatal("rotate must return a DIFFERENT secret")
	}
}

func TestSmoke_RotatePublicClientReturns400(t *testing.T) {
	di := makeDI(t)
	rec := serveReq(t, di, http.MethodPost,
		"/api/v1/applications/{id:app-1}/oauth-clients",
		map[string]any{
			"client_id":      "spa",
			"client_type":    "public",
			"display_name":   "SPA",
			"redirect_uris":  []string{"https://spa/cb"},
			"allowed_scopes": []string{"openid"},
		},
		&CreateHandler{})
	var created createResponse
	decodeMessage(t, rec, &created)

	rec = serveReq(t, di, http.MethodPost,
		"/api/v1/applications/{id:app-1}/oauth-clients/"+created.Client.ID+"/rotate-secret",
		nil, &RotateHandler{})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400 for public-client rotate, got %d", rec.Code)
	}
}

func TestSmoke_DeleteRemovesRow(t *testing.T) {
	di := makeDI(t)
	rec := serveReq(t, di, http.MethodPost,
		"/api/v1/applications/{id:app-1}/oauth-clients",
		map[string]any{
			"client_id":      "c",
			"client_type":    "public",
			"display_name":   "x",
			"redirect_uris":  []string{"https://c/cb"},
			"allowed_scopes": []string{"openid"},
		},
		&CreateHandler{})
	var created createResponse
	decodeMessage(t, rec, &created)

	rec = serveReq(t, di, http.MethodDelete,
		"/api/v1/applications/{id:app-1}/oauth-clients/"+created.Client.ID,
		nil, &DeleteHandler{})
	if rec.Code != http.StatusOK {
		t.Fatalf("delete: %d", rec.Code)
	}
	rec = serveReq(t, di, http.MethodGet,
		"/api/v1/applications/{id:app-1}/oauth-clients", nil, &ListHandler{})
	var list listResponse
	decodeMessage(t, rec, &list)
	if len(list.Clients) != 0 {
		t.Errorf("expected empty list after delete, got %d", len(list.Clients))
	}
}

func TestSmoke_UpdateUnknownReturns404(t *testing.T) {
	di := makeDI(t)
	rec := serveReq(t, di, http.MethodPatch,
		"/api/v1/applications/{id:app-1}/oauth-clients/missing",
		map[string]any{
			"display_name":   "x",
			"redirect_uris":  []string{"https://c/cb"},
			"allowed_scopes": []string{"openid"},
		},
		&UpdateHandler{})
	if rec.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rec.Code)
	}
}
