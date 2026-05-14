package admin

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	db "github.com/a-digi/coco-orm/orm"
	"github.com/a-digi/coco-server/server/request"
	logger "github.com/a-digi/coco-logger/logger"

	_ "github.com/mattn/go-sqlite3"
)

// Smoke tests exercise the real HTTP handler against an in-
// memory SQLite carrying the production schema. They verify the
// end-to-end wire shape a real admin client would see.

type stubDI struct {
	mgr *db.DatabaseManager
}

func (s *stubDI) GetDatabaseManager() *db.DatabaseManager { return s.mgr }
func (s *stubDI) GetLogger() logger.Logger                { return nil }

func makeDI(t *testing.T) (*stubDI, *sql.DB) {
	t.Helper()
	conn, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	if _, err := conn.Exec(`
		CREATE TABLE application_oauth_providers (
		    id                  TEXT NOT NULL PRIMARY KEY,
		    application_id      TEXT NOT NULL,
		    provider            TEXT NOT NULL,
		    client_id           TEXT NOT NULL,
		    client_secret_enc   TEXT NOT NULL,
		    discovery_url       TEXT NOT NULL DEFAULT '',
		    authorize_url       TEXT NOT NULL DEFAULT '',
		    token_url           TEXT NOT NULL DEFAULT '',
		    userinfo_url        TEXT NOT NULL DEFAULT '',
		    scopes              TEXT NOT NULL DEFAULT '',
		    allow_login         INTEGER NOT NULL DEFAULT 1,
		    allow_registration  INTEGER NOT NULL DEFAULT 0,
		    is_active           INTEGER NOT NULL DEFAULT 1,
		    created_at          DATETIME DEFAULT CURRENT_TIMESTAMP,
		    updated_at          DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE UNIQUE INDEX application_oauth_providers_app_provider_idx
		    ON application_oauth_providers (application_id, provider);
	`); err != nil {
		t.Fatalf("schema: %v", err)
	}
	mgr := &db.DatabaseManager{Connector: &db.Connector{DB: conn}}
	return &stubDI{mgr: mgr}, conn
}

func serve(t *testing.T, method, path string, body any, h http.Handler) *httptest.ResponseRecorder {
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
	h.ServeHTTP(rec, r)
	return rec
}

type genericHandler func(reqCtx request.RequestContext)

func (g genericHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	panic("use serveReq instead — ServeHTTP here would drop the DI context")
}

// serveReq invokes a coco ServeHTTP(reqCtx) with a constructed
// request.RequestContext so the handler can reach DI.
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

type wireEnvelope struct {
	Success bool `json:"success"`
	Message any  `json:"message"`
	Error   bool `json:"error"`
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

// ------- smoke tests ------------------------------------------------

func TestSmoke_CreateListUpdateDelete(t *testing.T) {
	di, _ := makeDI(t)

	createBody := map[string]any{
		"provider":           "google",
		"client_id":          "client-abc",
		"client_secret":      "secret-xyz",
		"discovery_url":      "https://accounts.google.com/.well-known/openid-configuration",
		"authorize_url":      "",
		"token_url":          "",
		"userinfo_url":       "",
		"scopes":             []string{"openid", "email", "profile"},
		"allow_login":        true,
		"allow_registration": true,
	}

	// Create. The framework's SuccessResponse hardcodes 200 even
	// when the handler asked for 201 — we just confirm success.
	rec := serveReq(t, di, http.MethodPost,
		"/api/v1/applications/{id:app-1}/oauth-providers",
		createBody, &CreateHandler{})
	if rec.Code != http.StatusOK {
		t.Fatalf("create: want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var created adminView
	decodeMessage(t, rec, &created)
	if created.Provider != "google" || created.ClientID != "client-abc" {
		t.Fatalf("created view wrong: %+v", created)
	}
	if created.ClientSecretMask == "secret-xyz" {
		t.Fatalf("plaintext secret leaked in list response")
	}

	// List
	rec = serveReq(t, di, http.MethodGet,
		"/api/v1/applications/{id:app-1}/oauth-providers",
		nil, &ListHandler{})
	if rec.Code != http.StatusOK {
		t.Fatalf("list: want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var list listResponse
	decodeMessage(t, rec, &list)
	if len(list.Providers) != 1 {
		t.Fatalf("list: want 1 provider, got %d", len(list.Providers))
	}

	// Update (rotate secret + disable registration)
	newSecret := "rotated"
	updateBody := map[string]any{
		"client_id":          "client-abc",
		"client_secret":      newSecret,
		"discovery_url":      "https://accounts.google.com/.well-known/openid-configuration",
		"authorize_url":      "",
		"token_url":          "",
		"userinfo_url":       "",
		"scopes":             []string{"openid", "email"},
		"allow_login":        true,
		"allow_registration": false,
		"is_active":          true,
	}
	rec = serveReq(t, di, http.MethodPatch,
		"/api/v1/applications/{id:app-1}/oauth-providers/"+created.ID,
		updateBody, &UpdateHandler{})
	if rec.Code != http.StatusOK {
		t.Fatalf("update: want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var updated adminView
	decodeMessage(t, rec, &updated)
	if updated.AllowRegistration {
		t.Errorf("allow_registration flag did not flip")
	}
	if len(updated.Scopes) != 2 {
		t.Errorf("scopes: got %v", updated.Scopes)
	}

	// Delete
	rec = serveReq(t, di, http.MethodDelete,
		"/api/v1/applications/{id:app-1}/oauth-providers/"+created.ID,
		nil, &DeleteHandler{})
	if rec.Code != http.StatusOK {
		t.Fatalf("delete: want 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	// List again — empty.
	rec = serveReq(t, di, http.MethodGet,
		"/api/v1/applications/{id:app-1}/oauth-providers",
		nil, &ListHandler{})
	decodeMessage(t, rec, &list)
	if len(list.Providers) != 0 {
		t.Errorf("after delete want 0 providers, got %d", len(list.Providers))
	}
}

func TestSmoke_CreateUnknownProviderRejected(t *testing.T) {
	di, _ := makeDI(t)
	rec := serveReq(t, di, http.MethodPost,
		"/api/v1/applications/{id:app-1}/oauth-providers",
		map[string]any{
			"provider":      "facebook",
			"client_id":     "x",
			"client_secret": "y",
		},
		&CreateHandler{})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400 on unknown provider, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSmoke_CreateDuplicateProviderReturns409(t *testing.T) {
	di, _ := makeDI(t)
	body := map[string]any{
		"provider":      "google",
		"client_id":     "x",
		"client_secret": "y",
	}
	serveReq(t, di, http.MethodPost,
		"/api/v1/applications/{id:app-1}/oauth-providers", body, &CreateHandler{})
	rec := serveReq(t, di, http.MethodPost,
		"/api/v1/applications/{id:app-1}/oauth-providers", body, &CreateHandler{})
	if rec.Code != http.StatusConflict {
		t.Errorf("want 409, got %d", rec.Code)
	}
}

func TestSmoke_UpdateMissingIDReturns400(t *testing.T) {
	di, _ := makeDI(t)
	rec := serveReq(t, di, http.MethodPatch,
		"/api/v1/applications/{id:app-1}/oauth-providers/",
		map[string]any{"client_id": "x"},
		&UpdateHandler{})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

func TestSmoke_UpdateUnknownProviderReturns404(t *testing.T) {
	di, _ := makeDI(t)
	rec := serveReq(t, di, http.MethodPatch,
		"/api/v1/applications/{id:app-1}/oauth-providers/does-not-exist",
		map[string]any{"client_id": "x"},
		&UpdateHandler{})
	if rec.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rec.Code)
	}
}

// compile-time pin: generic envelope isn't referenced anywhere
// else; swap it to _ if the assertion helpers ever stop wanting
// the field set.
var _ = wireEnvelope{}
