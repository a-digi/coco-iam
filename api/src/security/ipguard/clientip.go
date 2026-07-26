package ipguard

import (
	"net"
	"net/http"
	"strings"
)

// ClientIP extracts the caller's address from r.
//
// If trustHeader is empty, RemoteAddr is trusted directly — correct
// when nothing sits in front of this process (local dev, or hitting
// the Go process directly). If trustHeader is set (e.g. "X-Real-IP"),
// its value is trusted as the single upstream-supplied client IP.
//
// trustHeader must only be set once the reverse proxy in front of this
// process is confirmed to set/overwrite that header itself — this
// function has no way to verify the header wasn't supplied by the
// client. See plan/ip-abuse-protection/plan.md, Open Question 1.
func ClientIP(r *http.Request, trustHeader string) string {
	if trustHeader != "" {
		if v := strings.TrimSpace(r.Header.Get(trustHeader)); v != "" {
			// X-Forwarded-For may be a comma-separated chain; the
			// first entry is the original client when exactly one
			// proxy hop is trusted.
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
