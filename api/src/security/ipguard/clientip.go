package ipguard

import (
	"net"
	"net/http"
	"strings"
)

// ClientIP extracts the caller's address from r.
//
// trustHeaders is an ordered list of header names to try, highest
// priority first (e.g. ["X-Real-IP", "X-Forwarded-For"]) — the first
// one present on r that parses as a valid IP wins. If trustHeaders is
// empty, or none of them yield a valid IP, RemoteAddr is trusted
// directly — correct when nothing sits in front of this process
// (local dev, or hitting the Go process directly), and the necessary
// fallback when a reverse proxy simply didn't set any of the
// configured headers on this particular request (e.g. a request that
// landed on a catch-all/default vhost instead of the one configured
// with the proxy_set_header directive).
//
// trustHeaders must only include a header once the reverse proxy in
// front of this process is confirmed to set/overwrite it itself — this
// function has no way to verify a header wasn't supplied by the
// client. See plan/ip-abuse-protection/plan.md, Open Question 1, and
// plan/attack-ip-attribution/plan.md for why a single header wasn't
// enough in practice.
func ClientIP(r *http.Request, trustHeaders []string) string {
	for _, header := range trustHeaders {
		if v := strings.TrimSpace(r.Header.Get(header)); v != "" {
			// X-Forwarded-For (and similarly-shaped headers) may be a
			// comma-separated chain; the first entry is the original
			// client when exactly one proxy hop is trusted.
			if idx := strings.IndexByte(v, ','); idx >= 0 {
				v = strings.TrimSpace(v[:idx])
			}
			if ip := net.ParseIP(v); ip != nil {
				return ip.String()
			}
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.String()
	}
	return host
}

// isLoopbackOrPrivate reports whether ip parses as a loopback
// (127.0.0.0/8, ::1) or RFC1918/ULA private address. Used to decide
// whether an ip resolved by ClientIP is worth capturing an origin-hint
// diagnostic snapshot for (see recordAttackHit and
// plan/attack-ip-attribution/plan.md Fix 3): a genuine public attacker
// IP needs no such fallback, but a loopback/private result usually
// means none of the configured trust headers resolved on this
// particular request, and the raw headers actually present are the
// only lead left for tracing the real source by hand.
func isLoopbackOrPrivate(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	return parsed.IsLoopback() || parsed.IsPrivate()
}
