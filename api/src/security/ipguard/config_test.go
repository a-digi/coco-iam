package ipguard

import "testing"

func TestLoadConfig_DefaultsWhenSecurityKeyAbsent(t *testing.T) {
	cfg, err := LoadConfig([]byte(`{"port": 2026}`))
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	want := DefaultConfig()
	if cfg.TrustProxyIPHeader != want.TrustProxyIPHeader {
		t.Errorf("TrustProxyIPHeader = %q, want %q", cfg.TrustProxyIPHeader, want.TrustProxyIPHeader)
	}
	if cfg.RateLimit.Global != want.RateLimit.Global {
		t.Errorf("RateLimit.Global = %+v, want %+v", cfg.RateLimit.Global, want.RateLimit.Global)
	}
	if cfg.RateLimit.Sensitive != want.RateLimit.Sensitive {
		t.Errorf("RateLimit.Sensitive = %+v, want %+v", cfg.RateLimit.Sensitive, want.RateLimit.Sensitive)
	}
	if len(cfg.RateLimit.SensitivePaths) != len(want.RateLimit.SensitivePaths) {
		t.Errorf("SensitivePaths len = %d, want %d", len(cfg.RateLimit.SensitivePaths), len(want.RateLimit.SensitivePaths))
	}
}

func TestLoadConfig_PartialOverrideKeepsOtherDefaults(t *testing.T) {
	cfg, err := LoadConfig([]byte(`{"security": {"trust_proxy_ip_header": "X-Real-IP"}}`))
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.TrustProxyIPHeader != "X-Real-IP" {
		t.Errorf("TrustProxyIPHeader = %q, want %q", cfg.TrustProxyIPHeader, "X-Real-IP")
	}
	want := DefaultConfig()
	if cfg.RateLimit.Global != want.RateLimit.Global {
		t.Errorf("RateLimit.Global should keep its default when omitted, got %+v", cfg.RateLimit.Global)
	}
}

func TestLoadConfig_FullOverride(t *testing.T) {
	cfg, err := LoadConfig([]byte(`{
		"security": {
			"trust_proxy_ip_header": "X-Forwarded-For",
			"rate_limit": {
				"global":    {"requests": 100, "window_seconds": 30, "ban_seconds": 600},
				"sensitive": {"requests": 5,   "window_seconds": 120, "ban_seconds": 1800},
				"sensitive_paths": ["/admin/oauth/authenticate"]
			}
		}
	}`))
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.RateLimit.Global != (TierConfig{Requests: 100, WindowSeconds: 30, BanSeconds: 600}) {
		t.Errorf("RateLimit.Global = %+v, unexpected", cfg.RateLimit.Global)
	}
	if len(cfg.RateLimit.SensitivePaths) != 1 || cfg.RateLimit.SensitivePaths[0] != "/admin/oauth/authenticate" {
		t.Errorf("SensitivePaths = %v, want single overridden entry", cfg.RateLimit.SensitivePaths)
	}
}

func TestLoadConfig_InvalidJSON(t *testing.T) {
	if _, err := LoadConfig([]byte(`{not json`)); err == nil {
		t.Fatal("expected an error for malformed JSON")
	}
}

func TestLoadConfig_RejectsZeroRequests(t *testing.T) {
	_, err := LoadConfig([]byte(`{"security": {"rate_limit": {"global": {"requests": 0, "window_seconds": 60, "ban_seconds": 900}}}}`))
	if err == nil {
		t.Fatal("expected an error for zero requests (would lock out every caller instantly)")
	}
}

func TestLoadConfig_RejectsNegativeBanSeconds(t *testing.T) {
	_, err := LoadConfig([]byte(`{"security": {"rate_limit": {"sensitive": {"requests": 5, "window_seconds": 60, "ban_seconds": -1}}}}`))
	if err == nil {
		t.Fatal("expected an error for negative ban_seconds")
	}
}
