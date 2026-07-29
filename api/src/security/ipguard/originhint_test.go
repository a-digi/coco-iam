package ipguard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
