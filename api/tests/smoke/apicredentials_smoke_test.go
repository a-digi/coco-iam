//go:build smoke

package smoke

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// TestAPICredentialsEndToEnd walks the full lifecycle of an
// application API credential: admin seeds org/workspace/app, issues
// a credential, uses it against /a/... endpoints, revokes it, and
// verifies negative cases along the way.
//
// Requires a running server at COCO_IAM_URL and admin env vars —
// each subtest skips cleanly when either is missing.
func TestAPICredentialsEndToEnd(t *testing.T) {
	token := adminLogin(t)

	orgID, orgSlug := createOrg(t, token)
	t.Cleanup(func() { deleteOrg(t, token, orgID) })
	wsID, wsSlug := createWorkspace(t, token, orgID)
	appID, appSlug := createApplication(t, token, wsID)

	// Trigger initial key material so the public endpoints have
	// something to return. The admin keys-list handler runs an
	// `EnsureActive` lazy-heal on first read — easiest way to
	// force key creation without a dedicated endpoint.
	if _, status := getKeys(t, token, appID); status != http.StatusOK {
		t.Fatalf("seed keys: status %d", status)
	}

	publicPath := fmt.Sprintf("/a/%s/%s/%s/security-key",
		url.PathEscape(orgSlug), url.PathEscape(wsSlug), url.PathEscape(appSlug))
	privatePath := publicPath + "/private"

	var apiID, apiSecret string
	t.Run("admin creates credential and receives plaintext secret once", func(t *testing.T) {
		apiID, apiSecret = createCredential(t, token, appID)
		if apiID == "" || apiSecret == "" {
			t.Fatal("expected non-empty api_id + api_secret")
		}
	})

	t.Run("public endpoint returns signing keys", func(t *testing.T) {
		status, body := getWithBasic(t, apiID, apiSecret, publicPath)
		if status != http.StatusOK {
			t.Fatalf("public endpoint: status %d, body=%s", status, body)
		}
		if !strings.Contains(string(body), `"keys"`) {
			t.Errorf("expected keys array in body, got %s", body)
		}
		if !strings.Contains(string(body), "public_key_pem") {
			t.Errorf("expected public_key_pem in body, got %s", body)
		}
		if strings.Contains(string(body), "private_key_pem") {
			t.Errorf("public endpoint should not leak private_key_pem, got %s", body)
		}
	})

	t.Run("private endpoint returns active private key", func(t *testing.T) {
		status, body := getWithBasic(t, apiID, apiSecret, privatePath)
		if status != http.StatusOK {
			t.Fatalf("private endpoint: status %d, body=%s", status, body)
		}
		if !strings.Contains(string(body), "-----BEGIN PRIVATE KEY-----") &&
			!strings.Contains(string(body), "-----BEGIN RSA PRIVATE KEY-----") {
			t.Errorf("private endpoint body missing PEM header: %s", body)
		}
		if !strings.Contains(string(body), `"status":"active"`) {
			t.Errorf("private endpoint should return only the active key, got %s", body)
		}
	})

	t.Run("missing Authorization header is 401", func(t *testing.T) {
		status, _ := doRequest(t, http.MethodGet, publicPath, nil, nil)
		if status != http.StatusUnauthorized {
			t.Errorf("want 401, got %d", status)
		}
	})

	t.Run("wrong secret is 401 (obfuscated, not distinguishable from unknown api_id)", func(t *testing.T) {
		status, _ := getWithBasic(t, apiID, "WRONG-SECRET", publicPath)
		if status != http.StatusUnauthorized {
			t.Errorf("want 401, got %d", status)
		}
		status2, _ := getWithBasic(t, "unknown-api-id", "ignored", publicPath)
		if status2 != http.StatusUnauthorized {
			t.Errorf("want 401 for unknown api_id, got %d", status2)
		}
	})

	t.Run("cross-tenant credential is 401", func(t *testing.T) {
		// Spin up a second org/workspace/app and reuse apiID/apiSecret
		// from the first. The authn layer should reject it because
		// the credential's application_id doesn't match.
		_, otherOrgSlug := createOrg(t, token)
		// Clean up inside this subtest so parent cleanup stays tidy.
		// Fetch the second org's id via the admin list endpoint would
		// be extra; accept the best-effort leak in the test env.
		otherPath := fmt.Sprintf("/a/%s/%s/%s/security-key",
			url.PathEscape(otherOrgSlug), url.PathEscape(wsSlug), url.PathEscape(appSlug))
		status, _ := getWithBasic(t, apiID, apiSecret, otherPath)
		if status != http.StatusUnauthorized {
			t.Errorf("cross-tenant: want 401, got %d", status)
		}
	})

	t.Run("revoked credential is 401", func(t *testing.T) {
		revokeCredential(t, token, appID, apiID)
		status, _ := getWithBasic(t, apiID, apiSecret, publicPath)
		if status != http.StatusUnauthorized {
			t.Errorf("revoked: want 401, got %d", status)
		}
	})
}

// ---------- fixture helpers (single-file so smoke tests stay portable) ----------

type orgCreateResponse struct {
	Message struct {
		ID             string `json:"id"`
		OrganizationID string `json:"organization_id"`
	} `json:"message"`
}

// createOrg inserts a fresh organization via the admin REST endpoint
// and returns (uuid, slug).
func createOrg(t *testing.T, token string) (string, string) {
	t.Helper()
	slug := "smoke-org-" + timestamp()
	body := map[string]interface{}{
		"organization_id": slug,
		"title":           "Smoke Org " + slug,
		"is_active":       true,
	}
	var resp orgCreateResponse
	status := postJSON(t, token, "/api/v1/{res:organizations}", body, &resp)
	if status/100 != 2 {
		t.Fatalf("create org: status %d", status)
	}
	return resp.Message.ID, resp.Message.OrganizationID
}

// deleteOrg removes an organization. Best-effort — if this fails the
// parent test already passed or failed; leftover fixtures in dev are
// harmless.
func deleteOrg(t *testing.T, token, orgID string) {
	t.Helper()
	status, _ := doRequest(t, http.MethodDelete,
		"/api/v1/{res:organizations}/{id:"+orgID+"}", nil, func(h http.Header) {
			h.Set("Authorization", "Bearer "+token)
		})
	if status != http.StatusOK && status != http.StatusNoContent {
		t.Logf("cleanup: delete org %s returned %d (non-fatal)", orgID, status)
	}
}

type workspaceCreateResponse struct {
	Message struct {
		ID          string `json:"id"`
		WorkspaceID string `json:"workspace_id"`
	} `json:"message"`
}

func createWorkspace(t *testing.T, token, orgID string) (string, string) {
	t.Helper()
	slug := "smoke-ws-" + timestamp()
	body := map[string]interface{}{
		"workspace_id":    slug,
		"title":           "Smoke WS " + slug,
		"organization_id": orgID,
		"is_active":       true,
	}
	var resp workspaceCreateResponse
	status := postJSON(t, token, "/api/v1/{res:workspaces}", body, &resp)
	if status/100 != 2 {
		t.Fatalf("create workspace: status %d", status)
	}
	return resp.Message.ID, resp.Message.WorkspaceID
}

type appCreateResponse struct {
	Message struct {
		ID       string `json:"id"`
		ClientID string `json:"client_id"`
	} `json:"message"`
}

func createApplication(t *testing.T, token, wsID string) (string, string) {
	t.Helper()
	slug := "smoke-app-" + timestamp()
	body := map[string]interface{}{
		"client_id":    slug,
		"workspace_id": wsID,
		"title":        "Smoke App " + slug,
		"is_active":    true,
	}
	var resp appCreateResponse
	status := postJSON(t, token, "/api/v1/{res:applications}", body, &resp)
	if status/100 != 2 {
		t.Fatalf("create application: status %d", status)
	}
	return resp.Message.ID, resp.Message.ClientID
}

// getKeys calls the admin keys-list endpoint. Its side effect of
// lazily healing a missing active key is what bootstraps the public
// endpoints during smoke tests.
func getKeys(t *testing.T, token, appID string) (interface{}, int) {
	t.Helper()
	var raw interface{}
	status := getJSON(t, token,
		"/api/v1/{res:applications}/{id:"+appID+"}/keys", &raw)
	return raw, status
}

type credentialCreateResponse struct {
	Message struct {
		Credential struct {
			ID    string `json:"id"`
			APIID string `json:"api_id"`
		} `json:"credential"`
		APISecret string `json:"api_secret"`
	} `json:"message"`
}

func createCredential(t *testing.T, token, appID string) (string, string) {
	t.Helper()
	body := map[string]interface{}{
		"label":      "smoke test credential",
		"purposes":   []string{"security_key:read"},
		"expires_at": time.Now().Add(24 * time.Hour).Format(time.RFC3339),
	}
	var resp credentialCreateResponse
	status := postJSON(t, token,
		"/api/v1/{res:applications}/{id:"+appID+"}/api-credentials", body, &resp)
	if status != http.StatusCreated {
		t.Fatalf("create credential: status %d", status)
	}
	return resp.Message.Credential.APIID, resp.Message.APISecret
}

func revokeCredential(t *testing.T, token, appID, apiID string) {
	t.Helper()
	// We need the credential UUID (not api_id) to revoke — fetch the
	// list and match by api_id.
	var list struct {
		Message struct {
			Credentials []struct {
				ID    string `json:"id"`
				APIID string `json:"api_id"`
			} `json:"credentials"`
		} `json:"message"`
	}
	if status := getJSON(t, token,
		"/api/v1/{res:applications}/{id:"+appID+"}/api-credentials", &list); status != http.StatusOK {
		t.Fatalf("list credentials for revoke: status %d", status)
	}
	var credID string
	for _, c := range list.Message.Credentials {
		if c.APIID == apiID {
			credID = c.ID
			break
		}
	}
	if credID == "" {
		t.Fatalf("could not locate credential by api_id %q", apiID)
	}
	status := postJSON(t, token,
		"/api/v1/{res:applications}/{id:"+appID+"}/api-credentials/"+credID+"/revoke",
		struct{}{}, nil)
	if status != http.StatusOK {
		t.Fatalf("revoke credential: status %d", status)
	}
}

// timestamp returns a short monotonic-ish suffix used to make seeded
// slugs unique across concurrent runs.
func timestamp() string {
	return time.Now().UTC().Format("150405.000")
}
