package ipguard

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func bodyRequest(method, contentType, body string) *http.Request {
	r := httptest.NewRequest(method, "/x", strings.NewReader(body))
	if contentType != "" {
		r.Header.Set("Content-Type", contentType)
	}
	return r
}

func TestCaptureBodySample_SkipsGetAndHead(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		r := bodyRequest(method, "application/json", `{"password":"hunter2"}`)
		if got := captureBodySample(r); got != nil {
			t.Fatalf("captureBodySample(%s) = %v, want nil", method, *got)
		}
	}
}

func TestCaptureBodySample_SkipsUnrecognizedContentType(t *testing.T) {
	r := bodyRequest(http.MethodPost, "multipart/form-data; boundary=x", "binary junk")
	if got := captureBodySample(r); got != nil {
		t.Fatalf("captureBodySample() = %v, want nil for an uncaptured content type", *got)
	}
}

func TestCaptureBodySample_SkipsEmptyBody(t *testing.T) {
	r := bodyRequest(http.MethodPost, "application/json", "")
	if got := captureBodySample(r); got != nil {
		t.Fatalf("captureBodySample() = %v, want nil for an empty body", *got)
	}
}

func TestCaptureBodySample_RedactsJSONPassword(t *testing.T) {
	r := bodyRequest(http.MethodPost, "application/json", `{"email":"a@x.com","password":"hunter2"}`)
	got := captureBodySample(r)
	if got == nil {
		t.Fatal("captureBodySample() = nil, want a captured sample")
	}
	if strings.Contains(*got, "hunter2") {
		t.Fatalf("captureBodySample() = %q, must not contain the raw password", *got)
	}
	if !strings.Contains(*got, `"password":"[REDACTED]"`) {
		t.Fatalf("captureBodySample() = %q, want redacted password field", *got)
	}
	if !strings.Contains(*got, `"email":"a@x.com"`) {
		t.Fatalf("captureBodySample() = %q, want non-sensitive fields preserved", *got)
	}
}

func TestCaptureBodySample_RedactsFormEncodedPassword(t *testing.T) {
	r := bodyRequest(http.MethodPost, "application/x-www-form-urlencoded", "email=a%40x.com&password=hunter2")
	got := captureBodySample(r)
	if got == nil {
		t.Fatal("captureBodySample() = nil, want a captured sample")
	}
	if strings.Contains(*got, "hunter2") {
		t.Fatalf("captureBodySample() = %q, must not contain the raw password", *got)
	}
	if !strings.Contains(*got, "password="+redactedPlaceholder) {
		t.Fatalf("captureBodySample() = %q, want redacted password field", *got)
	}
}

func TestCaptureBodySample_HonorsCharsetParameter(t *testing.T) {
	r := bodyRequest(http.MethodPost, "application/json; charset=utf-8", `{"foo":"bar"}`)
	got := captureBodySample(r)
	if got == nil {
		t.Fatal("captureBodySample() = nil, want a captured sample despite the charset parameter")
	}
}

func TestCaptureBodySample_TruncatesOversizedBody(t *testing.T) {
	huge := `{"data":"` + strings.Repeat("a", bodySampleCap*2) + `"}`
	r := bodyRequest(http.MethodPost, "text/plain", huge)
	got := captureBodySample(r)
	if got == nil {
		t.Fatal("captureBodySample() = nil, want a truncated sample")
	}
	if !strings.HasSuffix(*got, truncatedSuffix) {
		t.Fatalf("captureBodySample() should end with %q, got suffix %q", truncatedSuffix, (*got)[max(0, len(*got)-20):])
	}
	if len(*got) > bodySampleCap+len(truncatedSuffix) {
		t.Fatalf("captureBodySample() length = %d, want <= %d", len(*got), bodySampleCap+len(truncatedSuffix))
	}
}
