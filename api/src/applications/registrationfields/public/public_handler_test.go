package public

import "testing"

// parseSlugSegments is the only pure helper in this handler — the
// rest is DI-resolver glue best covered by the smoke test. Pin the
// parser so a routing rewrite or trailing-slash change is caught
// in unit tests.

func TestParseSlugSegments_HappyPath(t *testing.T) {
	org, ws, app, ok := parseSlugSegments("/a/acme/prod/web/registration-fields")
	if !ok || org != "acme" || ws != "prod" || app != "web" {
		t.Errorf("happy path: got (%q,%q,%q,ok=%v)", org, ws, app, ok)
	}
}

func TestParseSlugSegments_TrailingSlashIgnored(t *testing.T) {
	org, _, _, ok := parseSlugSegments("/a/acme/prod/web/registration-fields/")
	if !ok || org != "acme" {
		t.Errorf("trailing slash: ok=%v org=%q", ok, org)
	}
}

func TestParseSlugSegments_MissingSegments(t *testing.T) {
	cases := []string{
		"", "/", "/a", "/a/", "/a/acme", "/a/acme/prod", "/a/acme/prod/",
	}
	for _, path := range cases {
		_, _, _, ok := parseSlugSegments(path)
		if ok {
			t.Errorf("path %q should be rejected", path)
		}
	}
}

func TestParseSlugSegments_WrongPrefix(t *testing.T) {
	cases := []string{
		"/api/v1/applications/acme/prod/web/registration-fields",
		"/login/a/acme/prod/web",
		"/p/applications/acme/prod/web",
	}
	for _, path := range cases {
		_, _, _, ok := parseSlugSegments(path)
		if ok {
			t.Errorf("wrong-prefix path %q should be rejected", path)
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
			t.Errorf("splitSegments(%q): len %d != %d (%v)", tc.in, len(got), len(tc.want), got)
			continue
		}
		for i, seg := range got {
			if seg != tc.want[i] {
				t.Errorf("splitSegments(%q)[%d]: got %q want %q", tc.in, i, seg, tc.want[i])
			}
		}
	}
}
