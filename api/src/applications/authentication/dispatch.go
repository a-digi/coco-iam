// Package authentication holds the public-app authenticate handler and
// the small HTTP helper that dispatches the per-application redirect
// call after successful password verification.
package authentication

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/a-digi/coco-iam/src/applications/loginpage"
)

// dispatchTimeout caps how long we wait for the target's response.
// Login is already a latency-sensitive user interaction — no point
// letting a slow callback wedge it.
const dispatchTimeout = 10 * time.Second

// dispatchRedirect calls the application's configured URL using the
// admin-picked method, attaching Authorization / X-Login-Secret /
// X-Renew-Token plus any admin-defined custom headers. A 2xx response
// is success; anything else bubbles up as a dispatchError so the
// authenticate handler can respond 502 without leaking the upstream
// body to the end user.
//
// The access token goes in Authorization; the refresh token rides on
// X-Renew-Token so the target can call /oauth/renew when the access
// token expires. The shared secret rides on X-Login-Secret and is
// what the target uses to prove the call came from coco-iam.
func dispatchRedirect(settings loginpage.Settings, accessToken, refreshToken string) (*dispatchResult, error) {
	method := settings.RedirectMethod
	if method != http.MethodPost && method != http.MethodGet {
		return nil, fmt.Errorf("login dispatch: unsupported method %q", method)
	}

	ctx, cancel := context.WithTimeout(context.Background(), dispatchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, settings.RedirectURL, nil)
	if err != nil {
		return nil, fmt.Errorf("login dispatch: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("X-Login-Secret", settings.RedirectSecret)
	req.Header.Set("X-Renew-Token", refreshToken)
	// Admin custom headers apply last — we already block Authorization
	// and the two coco-managed headers at save time, so whatever the
	// admin set here is safe to write.
	for k, v := range settings.CustomHeaders {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: dispatchTimeout}
	resp, err := client.Do(req)
	if err != nil {
		// Unwrap network-level errors (connection refused, DNS failure,
		// timeout) into a human-readable message so the end-user sees
		// "dispatch target unreachable" rather than a raw dial error
		// with internal hostnames / ports.
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			return nil, fmt.Errorf("login dispatch: dispatch target unreachable (%s)", settings.RedirectURL)
		}
		return nil, fmt.Errorf("login dispatch: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("login dispatch: upstream returned %d", resp.StatusCode)
	}
	return &dispatchResult{
		RedirectURL: settings.RedirectURL,
	}, nil
}

type dispatchResult struct {
	// RedirectURL is where the FE should send the browser once it has
	// stored the tokens. For v1 this is always the same URL that was
	// POST/GET'd — the response isn't parsed for overrides.
	RedirectURL string
}
