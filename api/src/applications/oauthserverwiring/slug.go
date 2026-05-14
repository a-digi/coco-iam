package oauthserverwiring

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/a-digi/coco-iam/src/applications/loginpage"
)

// SlugRouting bundles the URL → (appID, orgID, issuer, base path)
// closures the OAuth server handlers need. Built once at startup
// from a loginpage.Service + the public base URL of this server.
type SlugRouting struct {
	Login          *loginpage.Service
	PublicBaseURL  string // e.g. "https://iam.example"
	LoginPagePath  string // path the SPA serves the login form at, default "/login/a"
}

// NewSlugRouting constructs the routing helpers. publicBase
// trimmed of trailing slash; loginPagePath defaults to
// "/login/a" if empty.
func NewSlugRouting(svc *loginpage.Service, publicBase, loginPagePath string) *SlugRouting {
	if loginPagePath == "" {
		loginPagePath = "/login/a"
	}
	return &SlugRouting{
		Login:         svc,
		PublicBaseURL: strings.TrimRight(publicBase, "/"),
		LoginPagePath: loginPagePath,
	}
}

// ApplicationIDFromRequest reads the slug triple out of the URL
// path and resolves it via loginpage.Service. Returns
// (appID, orgID, nil) on success.
func (r *SlugRouting) ApplicationIDFromRequest(req *http.Request) (string, string, error) {
	if r == nil || r.Login == nil {
		return "", "", errors.New("oauthserverwiring: SlugRouting not configured")
	}
	org, ws, app, ok := parseSlugSegments(req.URL.Path)
	if !ok {
		return "", "", errors.New("oauthserverwiring: URL does not match /a/{org}/{ws}/{app}/...")
	}
	info, err := r.Login.Store.FindBySlugs(org, ws, app)
	if err != nil {
		return "", "", err
	}
	return info.ID, info.OrganizationID, nil
}

// IssuerFromRequest builds the absolute issuer URL the tokens
// advertise. Format: <publicBase>/a/<org>/<ws>/<app>.
func (r *SlugRouting) IssuerFromRequest(req *http.Request, _ string) string {
	org, ws, app, ok := parseSlugSegments(req.URL.Path)
	if !ok {
		return r.PublicBaseURL
	}
	return fmt.Sprintf("%s/a/%s/%s/%s", r.PublicBaseURL, org, ws, app)
}

// BasePathFromRequest returns the slug-triple path prefix for
// the OIDC discovery handler. Format: /a/<org>/<ws>/<app>.
func (r *SlugRouting) BasePathFromRequest(req *http.Request) string {
	org, ws, app, ok := parseSlugSegments(req.URL.Path)
	if !ok {
		return ""
	}
	return fmt.Sprintf("/a/%s/%s/%s", org, ws, app)
}

// LoginRedirectURL returns where the authorize handler should
// 302 the unauthenticated user. The login page picks up the
// return_to query parameter and bounces back to /authorize
// after a successful login.
func (r *SlugRouting) LoginRedirectURL(req *http.Request, returnTo string) string {
	org, ws, app, ok := parseSlugSegments(req.URL.Path)
	if !ok {
		return r.PublicBaseURL + r.LoginPagePath
	}
	target := fmt.Sprintf("%s%s/%s/%s/%s", r.PublicBaseURL, r.LoginPagePath, org, ws, app)
	return target + "?return_to=" + url.QueryEscape(returnTo)
}

// parseSlugSegments extracts (org, ws, app) for any URL of the
// shape `/a/<org>/<ws>/<app>/<tail>`.
func parseSlugSegments(path string) (org, ws, app string, ok bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 4 || parts[0] != "a" {
		return "", "", "", false
	}
	return parts[1], parts[2], parts[3], true
}
