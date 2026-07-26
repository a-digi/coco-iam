package firewall

import (
	"testing"
	"time"
)

func TestNoopBanner_NeverAvailableAndNeverErrors(t *testing.T) {
	b := NewNoopBanner("test detail")
	if b.Available() {
		t.Fatal("NoopBanner.Available() must always be false")
	}
	if b.Name() != "none" {
		t.Fatalf("Name() = %q, want %q", b.Name(), "none")
	}
	if b.Detail() != "test detail" {
		t.Fatalf("Detail() = %q, want %q", b.Detail(), "test detail")
	}
	if err := b.Ban("203.0.113.7", time.Minute); err != nil {
		t.Fatalf("Ban() on a no-op backend must never error, got %v", err)
	}
	if err := b.Unban("203.0.113.7"); err != nil {
		t.Fatalf("Unban() on a no-op backend must never error, got %v", err)
	}
}

func TestValidateIP_AcceptsValidAddresses(t *testing.T) {
	for _, ip := range []string{"203.0.113.7", "10.0.0.1", "2001:db8::1", "::1"} {
		if err := validateIP(ip); err != nil {
			t.Errorf("validateIP(%q) error = %v, want nil", ip, err)
		}
	}
}

func TestValidateIP_RejectsInvalidAndInjectionAttempts(t *testing.T) {
	for _, ip := range []string{
		"", "not-an-ip", "203.0.113.7; rm -rf /", "203.0.113.7 -j ACCEPT",
		"$(reboot)", "203.0.113.7`whoami`", "203.0.113.999",
	} {
		if err := validateIP(ip); err == nil {
			t.Errorf("validateIP(%q) = nil, want an error (must reject non-IP input before it ever reaches a command)", ip)
		}
	}
}

func TestDetect_NeverReturnsNil(t *testing.T) {
	b := Detect(nil)
	if b == nil {
		t.Fatal("Detect() returned nil — must always return a usable Banner, even a NoopBanner")
	}
	if b.Name() == "" {
		t.Fatal("Name() must never be empty")
	}
	// Whatever platform this runs on, Available()/Detail() must be
	// internally consistent: available backends report no detail,
	// unavailable ones must explain why.
	if b.Available() && b.Detail() != "" {
		t.Fatalf("an available backend reported a Detail(): %q", b.Detail())
	}
	if !b.Available() && b.Detail() == "" {
		t.Fatal("an unavailable backend must explain why via Detail()")
	}
}
