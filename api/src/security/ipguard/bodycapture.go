package ipguard

import (
	"io"
	"net/http"
	"regexp"
	"strings"
)

// bodySampleCap bounds how much of a request body is ever read or
// stored for one target — keeps ip-attacks.db growth bounded per row
// and avoids reading unbounded attacker-controlled input into memory.
// See plan/attack-ip-attribution/plan.md Fix 2.
const bodySampleCap = 2048

const truncatedSuffix = "...[truncated]"

const redactedPlaceholder = "[REDACTED]"

// bodyCaptureContentTypes lists the only content types a body sample
// is ever captured for — structured/text bodies an admin can
// meaningfully read back. Binary bodies (multipart uploads,
// octet-stream, etc.) are never stored.
var bodyCaptureContentTypes = map[string]bool{
	"application/json":                  true,
	"application/x-www-form-urlencoded": true,
	"text/plain":                        true,
}

// sensitiveBodyKeyNames is checked case-insensitively against JSON/form
// keys before a body sample is stored. This matters concretely here:
// SensitivePaths (config.go) — /admin/oauth/authenticate,
// /recovery/reset, /applications/authenticate, etc. — carry plaintext
// credentials in their request bodies, and those are exactly the
// endpoints most likely to trip a ban and get their body captured.
var sensitiveBodyKeyNames = []string{
	"password", "secret", "token", "otp", "code",
	"client_secret", "access_token", "refresh_token", "authorization",
}

var (
	jsonSensitiveKeyPattern = regexp.MustCompile(`(?i)"(` + strings.Join(sensitiveBodyKeyNames, "|") + `)"\s*:\s*"[^"]*"`)
	formSensitiveKeyPattern = regexp.MustCompile(`(?i)(^|&)(` + strings.Join(sensitiveBodyKeyNames, "|") + `)=[^&]*`)
)

// captureBodySample reads up to bodySampleCap bytes of r's body,
// redacts any sensitive-looking JSON/form keys, and returns the
// result — or nil if r carries no body worth capturing (GET/HEAD, an
// unrecognized/binary content type, or an empty body).
//
// Safe to call unconditionally on every recordAttackHit invocation:
// every call site is a terminal path (see recordAttackHit's own doc
// comment) — the request is rejected right there and never forwarded
// to a handler that would also need to read r.Body, so there is no
// need to restore it after reading.
func captureBodySample(r *http.Request) *string {
	if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Body == nil {
		return nil
	}
	contentType := baseContentType(r.Header.Get("Content-Type"))
	if !bodyCaptureContentTypes[contentType] {
		return nil
	}

	raw, err := io.ReadAll(io.LimitReader(r.Body, bodySampleCap+1))
	if err != nil || len(raw) == 0 {
		return nil
	}

	truncated := len(raw) > bodySampleCap
	if truncated {
		raw = raw[:bodySampleCap]
	}

	sample := redactBodySample(contentType, string(raw))
	if truncated {
		sample += truncatedSuffix
	}
	return &sample
}

// baseContentType strips any ";charset=..." (or other) parameter and
// lowercases the result, so "application/json; charset=utf-8" matches
// the same map entry as "application/json".
func baseContentType(contentType string) string {
	if idx := strings.IndexByte(contentType, ';'); idx >= 0 {
		contentType = contentType[:idx]
	}
	return strings.ToLower(strings.TrimSpace(contentType))
}

// redactBodySample replaces the value of any sensitive-looking key
// (see sensitiveBodyKeyNames) in body with redactedPlaceholder. body
// may already be truncated to bodySampleCap bytes, so this is a
// best-effort regex scan tolerant of a cut-off document, not a strict
// parse — a truncated key/value pair simply won't match, which is
// safe (worst case: an already-truncated secret stays partially
// visible, no worse than the truncation itself already exposed).
func redactBodySample(contentType, body string) string {
	switch contentType {
	case "application/json":
		return jsonSensitiveKeyPattern.ReplaceAllString(body, `"$1":"`+redactedPlaceholder+`"`)
	case "application/x-www-form-urlencoded":
		return formSensitiveKeyPattern.ReplaceAllString(body, `$1$2=`+redactedPlaceholder)
	default:
		return body
	}
}
