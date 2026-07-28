package ipguard

import (
	"encoding/json"
	"net/http"
	"strings"
)

// originHintCandidateHeaders are captured verbatim (no redaction —
// this is transport metadata, not credentials) whenever an attack
// episode opens with a resolved IP that's loopback or private, i.e.
// none of the configured trust headers (ipguard.Config.TrustProxyIPHeaders)
// resolved to anything usable on this request. See
// plan/attack-ip-attribution/plan.md Fix 3: this is the "try other
// ways to find the IP" answer — a lead for manual correlation against
// proxy access logs, not an authenticated fact, and must never be fed
// back into automated banning.
var originHintCandidateHeaders = []string{
	"X-Forwarded-For", "X-Real-IP", "CF-Connecting-IP", "True-Client-IP", "Forwarded",
}

// captureOriginHint returns a small JSON snapshot of every candidate
// header actually present on r, plus Host and User-Agent, or nil if
// none of them carried anything. Captured once, at episode-creation
// time only (see recordAttackHit) — a diagnostic lead, not a per-hit
// log.
func captureOriginHint(r *http.Request) *string {
	hint := make(map[string]string, len(originHintCandidateHeaders)+2)
	for _, header := range originHintCandidateHeaders {
		if v := r.Header.Get(header); v != "" {
			hint[originHintKey(header)] = v
		}
	}
	if r.Host != "" {
		hint["host"] = r.Host
	}
	if ua := r.Header.Get("User-Agent"); ua != "" {
		hint["user_agent"] = ua
	}
	if len(hint) == 0 {
		return nil
	}

	raw, err := json.Marshal(hint)
	if err != nil {
		return nil
	}
	sample := string(raw)
	return &sample
}

// originHintKey converts a canonical header name (e.g.
// "X-Forwarded-For") into the snake_case JSON key used in the
// captured hint (e.g. "x_forwarded_for").
func originHintKey(header string) string {
	return strings.ToLower(strings.ReplaceAll(header, "-", "_"))
}
