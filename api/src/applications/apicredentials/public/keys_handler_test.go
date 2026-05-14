package public

import "testing"

// parseSlugSegments is the only pure helper in the handler — the
// rest is DI-resolver glue. Pin its behaviour so accidental router
// rewrites can't send the handler to the wrong application.

func TestParseSlugSegments_HappyPath(t *testing.T) {
	org, ws, app, ok := parseSlugSegments("/a/acme/prod/web/security-key")
	if !ok {
		t.Fatal("want ok=true")
	}
	if org != "acme" || ws != "prod" || app != "web" {
		t.Errorf("got (%q,%q,%q), want (acme,prod,web)", org, ws, app)
	}
}

func TestParseSlugSegments_PrivateSubpath(t *testing.T) {
	// The /private trailing segment shouldn't affect the triple —
	// both public and private handlers share this parser.
	org, ws, app, ok := parseSlugSegments("/a/acme/prod/web/security-key/private")
	if !ok {
		t.Fatal("want ok=true")
	}
	if org != "acme" || ws != "prod" || app != "web" {
		t.Errorf("private path: got (%q,%q,%q)", org, ws, app)
	}
}

func TestParseSlugSegments_TrailingSlashIgnored(t *testing.T) {
	org, _, _, ok := parseSlugSegments("/a/acme/prod/web/security-key/")
	if !ok || org != "acme" {
		t.Errorf("trailing slash: got ok=%v org=%q", ok, org)
	}
}

func TestParseSlugSegments_MissingSegments(t *testing.T) {
	cases := []string{
		"",
		"/",
		"/a",
		"/a/",
		"/a/acme",
		"/a/acme/prod",
		"/a/acme/prod/",
	}
	for _, path := range cases {
		_, _, _, ok := parseSlugSegments(path)
		if ok {
			t.Errorf("path %q should be rejected", path)
		}
	}
}

func TestParseSlugSegments_WrongPrefix(t *testing.T) {
	// Anything not rooted at `/a/` is a routing mismatch — the
	// router shouldn't invoke the handler for it, but the parser
	// still defends in depth.
	cases := []string{
		"/api/v1/a/acme/prod/web/security-key",
		"/login/a/acme/prod/web",
		"/p/applications/acme/prod/web",
	}
	for _, path := range cases {
		_, _, _, ok := parseSlugSegments(path)
		if ok {
			t.Errorf("path %q should be rejected (wrong prefix)", path)
		}
	}
}

func TestSplitSegments(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"/a/b/c", []string{"a", "b", "c"}},
		{"a/b/c", []string{"a", "b", "c"}},
		{"/a/b/c/", []string{"a", "b", "c"}},
		{"//a//b//", []string{"a", "b"}},
		{"/", []string{}},
		{"", []string{}},
	}
	for _, tc := range cases {
		got := splitSegments(tc.in)
		if len(got) != len(tc.want) {
			t.Errorf("splitSegments(%q): len got %d, want %d (got=%v)", tc.in, len(got), len(tc.want), got)
			continue
		}
		for i, seg := range got {
			if seg != tc.want[i] {
				t.Errorf("splitSegments(%q)[%d]: got %q, want %q", tc.in, i, seg, tc.want[i])
			}
		}
	}
}
