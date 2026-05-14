//go:build smoke

package smoke

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// TestRegistrationFieldsEndToEnd walks the full admin → public
// contract of the registration schema: admin saves a design,
// public consumer fetches it, admin re-saves a different shape,
// public sees the new shape, allow_registration is flipped off and
// the public endpoint starts returning 404.
//
// Requires a running server at COCO_IAM_URL and admin env vars;
// skips cleanly when unset.
func TestRegistrationFieldsEndToEnd(t *testing.T) {
	token := adminLogin(t)

	orgID, orgSlug := createOrg(t, token)
	t.Cleanup(func() { deleteOrg(t, token, orgID) })
	wsID, wsSlug := createWorkspace(t, token, orgID)
	appID, appSlug := createApplication(t, token, wsID)

	publicPath := fmt.Sprintf("/a/%s/%s/%s/registration-fields",
		url.PathEscape(orgSlug), url.PathEscape(wsSlug), url.PathEscape(appSlug))

	// Turn registration on for this app so the public endpoint
	// stops 404ing. Uses the existing login-settings PATCH path
	// which owns the allow_registration toggle.
	enableRegistration(t, token, appID, true)

	t.Run("admin saves a multi-step design", func(t *testing.T) {
		payload := map[string]interface{}{
			"steps": []map[string]interface{}{
				{
					"id": "step-a", "title": "Your details", "order_index": 0,
					"fields": []map[string]interface{}{
						{
							"id": "f-promo", "order_index": 0, "source": "custom",
							"name": "promo", "label": "Promo code",
							"data_type": "text", "is_required": false,
						},
					},
				},
				{
					"id": "step-b", "title": "Preferences", "order_index": 1,
					"fields": []map[string]interface{}{
						{
							"id": "f-newsletter", "order_index": 0, "source": "custom",
							"name": "newsletter", "label": "Subscribe",
							"data_type": "text", "is_required": false,
						},
					},
				},
			},
		}
		status := putJSON(t, token,
			"/api/v1/{res:applications}/{id:"+appID+"}/registration-fields",
			payload, nil)
		if status != http.StatusOK {
			t.Fatalf("put registration schema: status %d", status)
		}
	})

	t.Run("public endpoint returns both steps and correct assignment", func(t *testing.T) {
		status, body := doRequest(t, http.MethodGet, publicPath, nil, nil)
		if status != http.StatusOK {
			t.Fatalf("public GET: status %d body=%s", status, body)
		}
		if !strings.Contains(string(body), `"step-a"`) || !strings.Contains(string(body), `"step-b"`) {
			t.Errorf("both step ids should appear: %s", body)
		}
		if !strings.Contains(string(body), `"f-promo"`) || !strings.Contains(string(body), `"f-newsletter"`) {
			t.Errorf("both field ids should appear: %s", body)
		}
		if !strings.Contains(string(body), `"registration_type":"legacy"`) {
			t.Errorf("registration_type should default to legacy: %s", body)
		}
	})

	t.Run("admin replaces with a single-step design", func(t *testing.T) {
		payload := map[string]interface{}{
			"steps": []map[string]interface{}{
				{
					"id": "step-solo", "title": "Sign up", "order_index": 0,
					"fields": []map[string]interface{}{
						{
							"id": "f-solo", "order_index": 0, "source": "custom",
							"name": "display_name", "label": "Display name",
							"data_type": "text", "is_required": true,
						},
					},
				},
			},
		}
		status := putJSON(t, token,
			"/api/v1/{res:applications}/{id:"+appID+"}/registration-fields",
			payload, nil)
		if status != http.StatusOK {
			t.Fatalf("put single-step schema: status %d", status)
		}

		_, body := doRequest(t, http.MethodGet, publicPath, nil, nil)
		// Old design must be fully replaced — no ghost of the
		// previous steps/fields.
		if strings.Contains(string(body), "step-a") || strings.Contains(string(body), "f-promo") {
			t.Errorf("previous design should be wiped: %s", body)
		}
		if !strings.Contains(string(body), `"step-solo"`) {
			t.Errorf("new single-step design should be served: %s", body)
		}
	})

	t.Run("public endpoint 404s when registration is disabled", func(t *testing.T) {
		enableRegistration(t, token, appID, false)
		status, _ := doRequest(t, http.MethodGet, publicPath, nil, nil)
		if status != http.StatusNotFound {
			t.Errorf("want 404 after disabling registration, got %d", status)
		}
	})
}

// enableRegistration toggles the application's allow_registration
// flag via the existing login-settings PATCH endpoint. Non-fatal on
// failure — the subtest that needs it will fail with a clear
// message.
func enableRegistration(t *testing.T, token, appID string, on bool) {
	t.Helper()
	payload := map[string]interface{}{"allow_registration": on}
	status := patchJSON(t, token,
		"/api/v1/{res:applications}/{id:"+appID+"}/login-settings",
		payload, nil)
	if status/100 != 2 {
		t.Fatalf("toggle allow_registration=%v: status %d", on, status)
	}
}

// putJSON is the PUT analogue of postJSON in helpers.go.
// Duplicated here so the helpers file stays single-purpose.
func putJSON(t *testing.T, token, path string, body interface{}, out interface{}) int {
	t.Helper()
	return requestJSON(t, http.MethodPut, token, path, body, out)
}

func patchJSON(t *testing.T, token, path string, body interface{}, out interface{}) int {
	t.Helper()
	return requestJSON(t, http.MethodPatch, token, path, body, out)
}

func requestJSON(t *testing.T, method, token, path string, body, out interface{}) int {
	t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	status, data := doRequest(t, method, path, encoded, func(h http.Header) {
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
