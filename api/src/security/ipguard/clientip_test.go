package ipguard

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientIP_TrustsRemoteAddrWhenNoHeaderTrusted(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "203.0.113.7:54321"
	r.Header.Set("X-Real-IP", "9.9.9.9") // must be ignored, header untrusted

	if got := ClientIP(r, nil); got != "203.0.113.7" {
		t.Fatalf("ClientIP() = %q, want %q", got, "203.0.113.7")
	}
}

func TestClientIP_TrustsConfiguredHeader(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "10.0.0.1:12345" // the reverse proxy's own address
	r.Header.Set("X-Real-IP", "203.0.113.7")

	if got := ClientIP(r, []string{"X-Real-IP"}); got != "203.0.113.7" {
		t.Fatalf("ClientIP() = %q, want %q", got, "203.0.113.7")
	}
}

func TestClientIP_TakesFirstHopOfForwardedChain(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "10.0.0.1:12345"
	r.Header.Set("X-Forwarded-For", "203.0.113.7, 10.0.0.2")

	if got := ClientIP(r, []string{"X-Forwarded-For"}); got != "203.0.113.7" {
		t.Fatalf("ClientIP() = %q, want %q", got, "203.0.113.7")
	}
}

func TestClientIP_FallsBackToRemoteAddrWhenHeaderMissing(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "203.0.113.7:54321"
	// X-Real-IP intentionally not set

	if got := ClientIP(r, []string{"X-Real-IP"}); got != "203.0.113.7" {
		t.Fatalf("ClientIP() = %q, want %q", got, "203.0.113.7")
	}
}

func TestClientIP_FallsBackWhenHeaderIsNotAValidIP(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "203.0.113.7:54321"
	r.Header.Set("X-Real-IP", "not-an-ip; rm -rf /")

	if got := ClientIP(r, []string{"X-Real-IP"}); got != "203.0.113.7" {
		t.Fatalf("ClientIP() = %q, want %q (must fall back, not trust garbage)", got, "203.0.113.7")
	}
}

func TestClientIP_HandlesIPv6RemoteAddr(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "[2001:db8::1]:54321"

	if got := ClientIP(r, nil); got != "2001:db8::1" {
		t.Fatalf("ClientIP() = %q, want %q", got, "2001:db8::1")
	}
}

func TestClientIP_FallsThroughToSecondHeaderWhenFirstAbsent(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "10.0.0.1:12345"
	// X-Real-IP intentionally not set — simulates a request that landed
	// on a proxy vhost/location block that doesn't set the primary
	// header, per plan/attack-ip-attribution/plan.md's root-cause
	// diagnosis (mass-scanner traffic hitting a catch-all block).
	r.Header.Set("X-Forwarded-For", "203.0.113.7")

	if got := ClientIP(r, []string{"X-Real-IP", "X-Forwarded-For"}); got != "203.0.113.7" {
		t.Fatalf("ClientIP() = %q, want %q", got, "203.0.113.7")
	}
}

func TestClientIP_PrefersFirstHeaderOverLaterOnesWhenBothPresent(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "10.0.0.1:12345"
	r.Header.Set("X-Real-IP", "203.0.113.7")
	r.Header.Set("X-Forwarded-For", "198.51.100.9")

	if got := ClientIP(r, []string{"X-Real-IP", "X-Forwarded-For"}); got != "203.0.113.7" {
		t.Fatalf("ClientIP() = %q, want %q (higher-priority header should win)", got, "203.0.113.7")
	}
}

func TestClientIP_SkipsInvalidHeaderAndFallsThroughToNextCandidate(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "10.0.0.1:12345"
	r.Header.Set("X-Real-IP", "not-an-ip")
	r.Header.Set("X-Forwarded-For", "203.0.113.7")

	if got := ClientIP(r, []string{"X-Real-IP", "X-Forwarded-For"}); got != "203.0.113.7" {
		t.Fatalf("ClientIP() = %q, want %q", got, "203.0.113.7")
	}
}
