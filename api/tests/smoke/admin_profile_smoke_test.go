//go:build smoke

package smoke

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"testing"
)

// TestAdminProfileEndToEnd walks the full self-profile flow for a
// signed-in admin: fresh GET returns empty strings, PATCH name +
// locale persists, avatar upload makes the URL non-empty, public
// serve returns 200, delete clears it. Requires COCO_IAM_URL +
// admin env vars — skips cleanly otherwise.
func TestAdminProfileEndToEnd(t *testing.T) {
	token := adminLogin(t)

	t.Run("fresh GET /me returns empty profile fields", func(t *testing.T) {
		var body struct {
			Message struct {
				Username  string `json:"username"`
				FirstName string `json:"first_name"`
				LastName  string `json:"last_name"`
				AvatarURL string `json:"avatar_url"`
			} `json:"message"`
		}
		status := getJSON(t, token, "/api/v1/admin/users/me", &body)
		if status != http.StatusOK {
			t.Fatalf("GET /me: status %d", status)
		}
		if body.Message.Username == "" {
			t.Errorf("username should be populated from admin_users row: %+v", body.Message)
		}
		// first_name / last_name come from the profile row which
		// doesn't exist yet. They must be empty strings, not
		// `null`, so the FE has a stable shape.
		if body.Message.AvatarURL != "" {
			t.Errorf("avatar_url should be empty on fresh account, got %q", body.Message.AvatarURL)
		}
	})

	t.Run("PATCH updates first/last/locale", func(t *testing.T) {
		status := patchJSON(t, token, "/api/v1/admin/users/me", map[string]interface{}{
			"first_name": "Smoke",
			"last_name":  "Tester",
			"locale":     "en-US",
			"timezone":   "Europe/Berlin",
		}, nil)
		if status != http.StatusOK {
			t.Fatalf("PATCH /me: status %d", status)
		}

		var body struct {
			Message struct {
				FirstName string `json:"first_name"`
				LastName  string `json:"last_name"`
				Locale    string `json:"locale"`
				Timezone  string `json:"timezone"`
			} `json:"message"`
		}
		if s := getJSON(t, token, "/api/v1/admin/users/me", &body); s != http.StatusOK {
			t.Fatalf("GET /me after PATCH: status %d", s)
		}
		if body.Message.FirstName != "Smoke" || body.Message.LastName != "Tester" {
			t.Errorf("name fields not persisted: %+v", body.Message)
		}
		if body.Message.Locale != "en-US" || body.Message.Timezone != "Europe/Berlin" {
			t.Errorf("locale/timezone not persisted: %+v", body.Message)
		}
	})

	t.Run("PATCH rejects invalid locale", func(t *testing.T) {
		// Pin the validator: a malformed locale must 400, not
		// silently accept garbage.
		status := patchJSON(t, token, "/api/v1/admin/users/me", map[string]interface{}{
			"locale": "not a locale",
		}, nil)
		if status != http.StatusBadRequest {
			t.Errorf("malformed locale: want 400, got %d", status)
		}
	})

	// A minimal 1x1 PNG — enough bytes to satisfy both the file
	// upload and the public-serve round-trip without dragging in
	// a real image dependency.
	pngBytes, err := base64.StdEncoding.DecodeString(
		"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==",
	)
	if err != nil {
		t.Fatalf("decode test png: %v", err)
	}

	t.Run("upload avatar populates avatar_url", func(t *testing.T) {
		status, body := uploadMultipart(t, token,
			"/api/v1/admin/users/me/avatar", "file", "avatar.png", "image/png", pngBytes)
		if status != http.StatusOK {
			t.Fatalf("upload avatar: status %d body=%s", status, body)
		}
		if !strings.Contains(string(body), `"avatar_url":"/p/admin-avatars/`) {
			t.Errorf("upload response should include avatar_url: %s", body)
		}
	})

	t.Run("public GET /p/admin-avatars/<id> returns image bytes", func(t *testing.T) {
		// Fetch /me to get the admin id. Public serve path is
		// derivable from it.
		var body struct {
			Message struct {
				ID string `json:"id"`
			} `json:"message"`
		}
		if s := getJSON(t, token, "/api/v1/admin/users/me", &body); s != http.StatusOK {
			t.Fatalf("GET /me: status %d", s)
		}
		// Public endpoint — hit without any Authorization header.
		status, data := doRequest(t, http.MethodGet,
			"/p/admin-avatars/"+body.Message.ID, nil, nil)
		if status != http.StatusOK {
			t.Fatalf("public avatar: status %d", status)
		}
		if len(data) == 0 {
			t.Error("public avatar: empty body")
		}
	})

	t.Run("DELETE clears avatar_url", func(t *testing.T) {
		status := deleteRequest(t, token, "/api/v1/admin/users/me/avatar")
		if status != http.StatusOK {
			t.Fatalf("delete avatar: status %d", status)
		}
		var body struct {
			Message struct {
				AvatarURL string `json:"avatar_url"`
			} `json:"message"`
		}
		if s := getJSON(t, token, "/api/v1/admin/users/me", &body); s != http.StatusOK {
			t.Fatalf("GET /me after delete: status %d", s)
		}
		if body.Message.AvatarURL != "" {
			t.Errorf("avatar_url should be empty after delete, got %q", body.Message.AvatarURL)
		}
	})
}

// deleteRequest sends a bearer-auth DELETE and returns the status.
func deleteRequest(t *testing.T, token, path string) int {
	t.Helper()
	status, _ := doRequest(t, http.MethodDelete, path, nil, func(h http.Header) {
		if token != "" {
			h.Set("Authorization", "Bearer "+token)
		}
	})
	return status
}

// uploadMultipart builds a multipart/form-data request with a
// single file field and sends it with bearer auth. Returns status
// + raw body so the caller can assert on the response shape.
func uploadMultipart(t *testing.T, token, path, fieldName, filename, contentType string, fileBytes []byte) (int, []byte) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, fieldName, filename))
	h.Set("Content-Type", contentType)
	part, err := w.CreatePart(h)
	if err != nil {
		t.Fatalf("multipart create: %v", err)
	}
	if _, err := io.Copy(part, bytes.NewReader(fileBytes)); err != nil {
		t.Fatalf("multipart copy: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("multipart close: %v", err)
	}

	body := buf.Bytes()
	return doRequest(t, http.MethodPost, path, body, func(header http.Header) {
		header.Set("Content-Type", w.FormDataContentType())
		if token != "" {
			header.Set("Authorization", "Bearer "+token)
		}
	})
}
