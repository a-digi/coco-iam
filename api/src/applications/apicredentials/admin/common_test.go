package admin

import (
	"testing"
	"time"

	"github.com/a-digi/coco-iam/src/applications/apicredentials/entity"
)

// newTestCredential returns a fully-populated Credential for tests
// that need a stable fixture. The SecretHash field is deliberately a
// fake string — tests in this package don't exercise bcrypt.
func newTestCredential() entity.Credential {
	return entity.Credential{
		ID:            "cred-1",
		ApplicationID: "app-1",
		APIID:         "api-abc",
		SecretHash:    "fakeHash",
		Label:         "smoke",
		ExpiresAt:     time.Now().Add(time.Hour),
		IsActive:      true,
		CreatedAt:     time.Now(),
	}
}

func TestCredIDFromPath(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"/api/v1/applications/app-1/api-credentials/cred-42/revoke", "cred-42"},
		{"applications/app-1/api-credentials/abc/revoke", "abc"},
		{"/applications/app-1/api-credentials/with-dashes-ok/revoke", "with-dashes-ok"},
		// Malformed: missing trailing /revoke.
		{"/api/v1/applications/app-1/api-credentials/cred-42", ""},
		// Malformed: missing api-credentials marker.
		{"/api/v1/applications/app-1/foo/cred-42/revoke", ""},
		// Empty path.
		{"", ""},
	}
	for _, tc := range cases {
		got := credIDFromPath(tc.path)
		if got != tc.want {
			t.Errorf("credIDFromPath(%q): got %q, want %q", tc.path, got, tc.want)
		}
	}
}
