package scope

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"   ", nil},
		{"openid", []string{"openid"}},
		{"openid profile email", []string{"openid", "profile", "email"}},
		{"openid  openid email", []string{"openid", "email"}}, // dedupe
		{"openid\temail", []string{"openid", "email"}},        // tabs act as separators
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got := Parse(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Parse(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestJoin(t *testing.T) {
	if got := Join([]string{"openid", "profile"}); got != "openid profile" {
		t.Errorf("got %q", got)
	}
	if got := Join(nil); got != "" {
		t.Errorf("empty should yield empty, got %q", got)
	}
}

func TestContains(t *testing.T) {
	scopes := []string{"openid", "profile"}
	if !Contains(scopes, "openid") {
		t.Error("should find openid")
	}
	if Contains(scopes, "email") {
		t.Error("should not find email")
	}
	if Contains(scopes, "OPENID") {
		t.Error("scope match must be case-sensitive per RFC 6749 §3.3")
	}
}

func TestFilterAllowed(t *testing.T) {
	allowed := []string{"openid", "profile"}
	cases := []struct {
		name      string
		requested []string
		want      []string
	}{
		{"full match", []string{"openid", "profile"}, []string{"openid", "profile"}},
		{"partial", []string{"openid", "email", "profile"}, []string{"openid", "profile"}},
		{"none allowed", []string{"email"}, []string{}},
		{"preserves request order", []string{"profile", "openid"}, []string{"profile", "openid"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := FilterAllowed(tc.requested, allowed)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v want %v", got, tc.want)
			}
		})
	}

	// Empty allowlist yields nil — used by FilterAllowed to
	// signal "nothing passes."
	if got := FilterAllowed([]string{"openid"}, nil); got != nil {
		t.Errorf("nil allowlist must yield nil, got %v", got)
	}
}

func TestIsSubset(t *testing.T) {
	if !IsSubset(nil, []string{"a", "b"}) {
		t.Error("empty subset is always a subset")
	}
	if !IsSubset([]string{"a"}, []string{"a", "b"}) {
		t.Error("strict subset")
	}
	if IsSubset([]string{"a", "c"}, []string{"a", "b"}) {
		t.Error("missing element should fail")
	}
	// Duplicates in subset don't matter.
	if !IsSubset([]string{"a", "a"}, []string{"a"}) {
		t.Error("duplicates in subset should be tolerated")
	}
}

func TestTriggersOIDC(t *testing.T) {
	if !TriggersOIDC([]string{"openid", "email"}) {
		t.Error("openid should trigger OIDC")
	}
	if TriggersOIDC([]string{"profile", "email"}) {
		t.Error("no openid should not trigger OIDC")
	}
}

func TestTriggersOfflineAccess(t *testing.T) {
	if !TriggersOfflineAccess([]string{"openid", "offline_access"}) {
		t.Error("offline_access should trigger refresh-token issue")
	}
	if TriggersOfflineAccess([]string{"openid"}) {
		t.Error("absent offline_access should not trigger it")
	}
}

func TestClaimsFor(t *testing.T) {
	profile := ClaimsFor([]string{"profile"})
	if !containsAll(profile, []string{"name", "given_name", "family_name", "picture"}) {
		t.Errorf("profile claims missing expected fields: %v", profile)
	}
	email := ClaimsFor([]string{"email"})
	if !containsAll(email, []string{"email", "email_verified"}) {
		t.Errorf("email claims missing expected fields: %v", email)
	}
	// Unknown scope contributes nothing.
	if got := ClaimsFor([]string{"app:custom"}); len(got) != 0 {
		t.Errorf("unknown scope should add no claims, got %v", got)
	}
}

func containsAll(haystack, needles []string) bool {
	set := map[string]struct{}{}
	for _, h := range haystack {
		set[h] = struct{}{}
	}
	for _, n := range needles {
		if _, ok := set[n]; !ok {
			return false
		}
	}
	return true
}
