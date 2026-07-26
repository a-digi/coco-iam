package ipguard

import (
	"encoding/json"
	"fmt"
)

// TierConfig is the request-count threshold and ban duration for one
// rate-limit tier.
type TierConfig struct {
	Requests      int `json:"requests"`
	WindowSeconds int `json:"window_seconds"`
	BanSeconds    int `json:"ban_seconds"`
}

// RateLimitConfig holds both tiers described in
// plan/ip-abuse-protection/plan.md: a loose global tier applied to
// every request, and a strict sensitive tier applied additionally to
// SensitivePaths (login/recovery/verify-code endpoints).
type RateLimitConfig struct {
	Global         TierConfig `json:"global"`
	Sensitive      TierConfig `json:"sensitive"`
	SensitivePaths []string   `json:"sensitive_paths"`
}

// Config is the "security" section of config.json. It is parsed
// locally by this package, not added to coco-server's Config struct —
// same pattern already used for the existing "auth" key, which
// NewScopeSecurityLayer re-reads raw from the same file.
type Config struct {
	// TrustProxyIPHeader names the header to trust for the caller's
	// real IP (e.g. "X-Real-IP"). Empty means trust r.RemoteAddr
	// directly — the safe default with no reverse proxy in front.
	// See plan/ip-abuse-protection/plan.md, Open Question 1: only set
	// this once the reverse proxy in front is confirmed to set/
	// overwrite the header itself.
	TrustProxyIPHeader string          `json:"trust_proxy_ip_header"`
	RateLimit          RateLimitConfig `json:"rate_limit"`
}

// DefaultConfig returns the starting values from
// plan/ip-abuse-protection/plan.md section 6 — tune freely, none of
// this is hardcoded into the enforcement logic itself.
func DefaultConfig() Config {
	return Config{
		TrustProxyIPHeader: "",
		RateLimit: RateLimitConfig{
			Global:    TierConfig{Requests: 300, WindowSeconds: 60, BanSeconds: 900},
			Sensitive: TierConfig{Requests: 15, WindowSeconds: 300, BanSeconds: 3600},
			SensitivePaths: []string{
				"/admin/oauth/authenticate", "/admin/oauth/renew",
				"/recovery/request", "/recovery/verify", "/recovery/reset",
				"/activation/verify", "/activation/activate",
				"/applications/authenticate", "/applications/renew",
				"/public/applications/recover/request", "/public/applications/recover/reset",
			},
		},
	}
}

// LoadConfig parses the "security" key out of raw config.json bytes.
// Fields absent from cfgBytes (including the whole "security" key)
// keep their DefaultConfig() value — this lets an operator override
// just one field (e.g. trust_proxy_ip_header) without having to
// restate the whole block.
func LoadConfig(cfgBytes []byte) (Config, error) {
	cfg := DefaultConfig()
	wrapper := struct {
		Security *Config `json:"security"`
	}{Security: &cfg}

	if err := json.Unmarshal(cfgBytes, &wrapper); err != nil {
		return Config{}, fmt.Errorf("could not parse security config: %w", err)
	}

	if err := wrapper.Security.validate(); err != nil {
		return Config{}, err
	}

	return *wrapper.Security, nil
}

func (c Config) validate() error {
	for name, t := range map[string]TierConfig{"global": c.RateLimit.Global, "sensitive": c.RateLimit.Sensitive} {
		if t.Requests <= 0 {
			return fmt.Errorf("security.rate_limit.%s.requests must be > 0, got %d", name, t.Requests)
		}
		if t.WindowSeconds <= 0 {
			return fmt.Errorf("security.rate_limit.%s.window_seconds must be > 0, got %d", name, t.WindowSeconds)
		}
		if t.BanSeconds <= 0 {
			return fmt.Errorf("security.rate_limit.%s.ban_seconds must be > 0, got %d", name, t.BanSeconds)
		}
	}
	return nil
}
