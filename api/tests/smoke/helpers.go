//go:build smoke

// Package smoke hosts end-to-end smoke tests that run over real HTTP
// against a running coco-iam server. They are opt-in via the `smoke`
// build tag so `go test ./...` stays fast and doesn't require a live
// server.
//
// See README.md in this folder for how to start a server and run the
// tests locally.
package smoke

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

// Env var names the test harness reads. A missing var causes each
// test to `t.Skip` cleanly so `make test-smoke` never fails on a
// fresh checkout where nothing is configured yet.
const (
	envBaseURL      = "COCO_IAM_URL"
	envAdminUser    = "COCO_IAM_ADMIN_USER"
	envAdminPass    = "COCO_IAM_ADMIN_PASS"
	defaultBaseURL  = "http://localhost:2026"
	requestTimeout  = 10 * time.Second
)

// client is the smoke-test HTTP client: reasonable timeout, no shared
// cookie jar, and no keepalives (each test deserves a clean socket).
var client = &http.Client{Timeout: requestTimeout}

// baseURL returns the server address the tests should hit. Defaults
// to the dev server's fixed port if unset, so `make run-dev &&
// make test-smoke` works with zero env configuration.
func baseURL() string {
	if v := os.Getenv(envBaseURL); v != "" {
		return v
	}
	return defaultBaseURL
}

// requireAdmin fetches (user, pass) from env. If either is missing
// the calling test is skipped with a helpful message — smoke tests
// are opt-in, not mandatory.
func requireAdmin(t *testing.T) (string, string) {
	t.Helper()
	user := os.Getenv(envAdminUser)
	pass := os.Getenv(envAdminPass)
	if user == "" || pass == "" {
		t.Skipf("set %s and %s to run smoke tests against a live server", envAdminUser, envAdminPass)
	}
	return user, pass
}

// tokenResponse mirrors the admin auth endpoint's success body shape
// enough to pull out the access token.
type tokenResponse struct {
	Message struct {
		AccessToken string `json:"access_token"`
	} `json:"message"`
}

// adminLogin calls POST /api/v1/admin/oauth/authenticate with
// username+password and returns the access token. Any failure is
// fatal — subsequent tests can't run without a valid admin session.
func adminLogin(t *testing.T) string {
	t.Helper()
	user, pass := requireAdmin(t)

	body := map[string]string{
		"username": user,
		"password": pass,
	}
	var tok tokenResponse
	status := postJSON(t, "", "/api/v1/admin/oauth/authenticate", body, &tok)
	if status != http.StatusOK {
		t.Fatalf("admin login: status %d", status)
	}
	if tok.Message.AccessToken == "" {
		t.Fatalf("admin login: no access_token in response")
	}
	return tok.Message.AccessToken
}

// doRequest is the low-level HTTP helper every other helper goes
// through. Custom headers can be injected via headerSetter; the
// response body (bytes) + status are returned so callers can assert
// on either.
func doRequest(t *testing.T, method, path string, body []byte, headerSetter func(h http.Header)) (int, []byte) {
	t.Helper()
	url := baseURL() + path
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("build %s %s: %v", method, path, err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if headerSetter != nil {
		headerSetter(req.Header)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v (is the server running at %s ?)", method, path, err, baseURL())
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, data
}

// postJSON posts a JSON body with the optional bearer token and
// unmarshals the response into `out` (if non-nil). Returns the HTTP
// status.
func postJSON(t *testing.T, token, path string, body interface{}, out interface{}) int {
	t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	status, data := doRequest(t, http.MethodPost, path, encoded, func(h http.Header) {
		if token != "" {
			h.Set("Authorization", "Bearer "+token)
		}
	})
	if out != nil && status/100 == 2 {
		if err := json.Unmarshal(data, out); err != nil {
			t.Fatalf("unmarshal %s: %v (body=%s)", path, err, string(data))
		}
	}
	return status
}

// getJSON is the GET analogue of postJSON.
func getJSON(t *testing.T, token, path string, out interface{}) int {
	t.Helper()
	status, data := doRequest(t, http.MethodGet, path, nil, func(h http.Header) {
		if token != "" {
			h.Set("Authorization", "Bearer "+token)
		}
	})
	if out != nil && status/100 == 2 {
		if err := json.Unmarshal(data, out); err != nil {
			t.Fatalf("unmarshal %s: %v (body=%s)", path, err, string(data))
		}
	}
	return status
}

// getWithBasic is the GET call variant that uses HTTP Basic instead
// of bearer — used to hit /a/.../security-key endpoints. Returns
// status + raw bytes so tests can inspect the body shape.
func getWithBasic(t *testing.T, apiID, apiSecret, path string) (int, []byte) {
	t.Helper()
	return doRequest(t, http.MethodGet, path, nil, func(h http.Header) {
		h.Set("Authorization", basicAuth(apiID, apiSecret))
	})
}

// basicAuth builds the `Basic <base64>` header value from the two
// halves of a credential.
func basicAuth(user, pass string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+pass))
}
