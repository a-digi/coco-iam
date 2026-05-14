package activation

import "testing"

// These tests pin composeRedirectURL against exactly the strings the
// per-app login route accepts. A change to the URL shape (for example
// the segment separator or the `/login/a/` prefix) should fail these
// tests on purpose so the admin/activate flow and the public login
// route stay in lockstep.

func TestComposeRedirectURL(t *testing.T) {
	cases := []struct {
		name string
		row  *Row
		want string
	}{
		{
			name: "all slugs set",
			row:  &Row{RedirectOrgSlug: "acme", RedirectWorkspaceSlug: "prod", RedirectClientID: "web"},
			want: "/login/a/acme/prod/web",
		},
		{
			name: "empty org yields empty url",
			row:  &Row{RedirectOrgSlug: "", RedirectWorkspaceSlug: "prod", RedirectClientID: "web"},
			want: "",
		},
		{
			name: "empty workspace yields empty url",
			row:  &Row{RedirectOrgSlug: "acme", RedirectWorkspaceSlug: "", RedirectClientID: "web"},
			want: "",
		},
		{
			name: "empty client id yields empty url",
			row:  &Row{RedirectOrgSlug: "acme", RedirectWorkspaceSlug: "prod", RedirectClientID: ""},
			want: "",
		},
		{
			name: "all empty yields empty url",
			row:  &Row{},
			want: "",
		},
		{
			name: "slug with space is path-escaped",
			row:  &Row{RedirectOrgSlug: "my org", RedirectWorkspaceSlug: "prod", RedirectClientID: "web"},
			want: "/login/a/my%20org/prod/web",
		},
		{
			name: "slug with slash is path-escaped",
			row:  &Row{RedirectOrgSlug: "acme", RedirectWorkspaceSlug: "prod/v2", RedirectClientID: "web"},
			want: "/login/a/acme/prod%2Fv2/web",
		},
		{
			name: "plus sign in client id is path-escaped",
			row:  &Row{RedirectOrgSlug: "acme", RedirectWorkspaceSlug: "prod", RedirectClientID: "web+mobile"},
			want: "/login/a/acme/prod/web+mobile",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := composeRedirectURL(tc.row)
			if got != tc.want {
				t.Errorf("composeRedirectURL: got %q, want %q", got, tc.want)
			}
		})
	}
}
