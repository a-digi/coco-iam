package ipguard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsLoopbackOrPrivate(t *testing.T) {
	cases := []struct {
		ip   string
		want bool
	}{
		{"127.0.0.1", true},
		{"::1", true},
		{"10.0.0.1", true},
		{"172.16.5.9", true},
		{"192.168.1.1", true},
		{"203.0.113.7", false},
		{"91.230.168.240", false},
		{"not-an-ip", false},
	}
	for _, c := range cases {
		if got := isLoopbackOrPrivate(c.ip); got != c.want {
			t.Errorf("isLoopbackOrPrivate(%q) = %v, want %v", c.ip, got, c.want)
		}
	}
}

func TestCaptureOriginHint_NilWhenNoCandidateHeadersPresent(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Host = ""
	if got := captureOriginHint(r); got != nil {
		t.Fatalf("captureOriginHint() = %v, want nil", *got)
	}
}

func TestCaptureOriginHint_CapturesPresentHeadersAndHost(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/wp-admin", nil)
	r.Host = "coco-iam.example.com"
	r.Header.Set("X-Forwarded-For", "198.51.100.9")
	r.Header.Set("User-Agent", "curl/8.0")

	got := captureOriginHint(r)
	if got == nil {
		t.Fatal("captureOriginHint() = nil, want a populated snapshot")
	}

	var hint map[string]string
	if err := json.Unmarshal([]byte(*got), &hint); err != nil {
		t.Fatalf("captureOriginHint() produced invalid JSON: %v", err)
	}
	if hint["x_forwarded_for"] != "198.51.100.9" {
		t.Errorf("hint[x_forwarded_for] = %q, want %q", hint["x_forwarded_for"], "198.51.100.9")
	}
	if hint["host"] != "coco-iam.example.com" {
		t.Errorf("hint[host] = %q, want %q", hint["host"], "coco-iam.example.com")
	}
	if hint["user_agent"] != "curl/8.0" {
		t.Errorf("hint[user_agent] = %q, want %q", hint["user_agent"], "curl/8.0")
	}
	if _, present := hint["x_real_ip"]; present {
		t.Errorf("hint should not contain x_real_ip when the header was never set")
	}
}
