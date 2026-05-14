// Package public hosts the slug-routed machine-auth endpoints under
// `/a/{orgSlug}/{wsSlug}/{appSlug}/...`. Authentication is HTTP Basic
// against an application API credential; no admin session is
// accepted. The admin session surface lives under /api/v1/.
package public

import (
	"net/http"
	"time"

	"github.com/a-digi/coco-iam/src/applications/apicredentials/authn"
	"github.com/a-digi/coco-iam/src/applications/apicredentials/dbregistry"
	"github.com/a-digi/coco-iam/src/applications/apicredentials/purpose"
	"github.com/a-digi/coco-iam/src/applications/apicredentials/repository"
	"github.com/a-digi/coco-iam/src/applications/keys"
	"github.com/a-digi/coco-iam/src/applications/loginpage"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// genericUnauthorized is the single user-facing error for every auth
// failure on this endpoint. Deliberately vague so attackers can't use
// the response to distinguish "unknown api_id" from "wrong secret"
// from "expired" from "missing purpose".
const genericUnauthorized = "unauthorized"

// GetPublicKeysHandler serves
//
//	GET /a/{orgSlug}/{wsSlug}/{appSlug}/security-key
//
// Returns a list of the application's non-expired signing keys with
// their public PEM and JWK form. The active key is the one that signs
// new tokens; deactivated keys still verify tokens in their grace
// window.
type GetPublicKeysHandler struct{}

func (h *GetPublicKeysHandler) ServeHTTP(reqCtx request.RequestContext) {
	serveKeys(reqCtx, false)
}

// GetPrivateKeyHandler serves
//
//	GET /a/{orgSlug}/{wsSlug}/{appSlug}/security-key/private
//
// Returns only the currently-active private key. Deactivated private
// keys are not exposed — they can't usefully sign anything, and
// handing them out would widen the blast radius of a credential leak.
type GetPrivateKeyHandler struct{}

func (h *GetPrivateKeyHandler) ServeHTTP(reqCtx request.RequestContext) {
	serveKeys(reqCtx, true)
}

// publicKeyEntry is the wire shape of one key on the public endpoint.
// `JWK` is a map not a struct so the fields can mirror the JOSE spec
// verbatim (`kty`, `n`, `e`) without Go-ism renaming.
type publicKeyEntry struct {
	KID           string         `json:"kid"`
	Status        string         `json:"status"`
	Use           string         `json:"use"`
	Alg           string         `json:"alg"`
	PublicKeyPEM  string         `json:"public_key_pem"`
	JWK           map[string]any `json:"jwk"`
	ActivatedAt   *time.Time     `json:"activated_at,omitempty"`
	DeactivatedAt *time.Time     `json:"deactivated_at,omitempty"`
	ExpiresAt     *time.Time     `json:"expires_at,omitempty"`
}

// privateKeyEntry is the wire shape for the private endpoint — one
// entry, active key only. Deliberately separate from publicKeyEntry
// so callers can't mix them up; the field name `private_key_pem`
// also makes the sensitivity obvious in logs and tests.
type privateKeyEntry struct {
	KID           string    `json:"kid"`
	Status        string    `json:"status"`
	Alg           string    `json:"alg"`
	PrivateKeyPEM string    `json:"private_key_pem"`
	ActivatedAt   time.Time `json:"activated_at"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
}

// serveKeys shares the auth + resolution flow between the public and
// private handlers. `wantPrivate` is the only behavioural switch.
func serveKeys(reqCtx request.RequestContext, wantPrivate bool) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	ctx := reqCtx.GetDI()

	orgSlug, wsSlug, appSlug, ok := parseSlugSegments(r.URL.Path)
	if !ok {
		response.ErrorResponse(w, http.StatusUnauthorized, genericUnauthorized)
		return
	}

	loginSvc := resolveLoginPageService(ctx)
	credRegistry := resolveCredRegistry(ctx)
	keysSvc := resolveKeysService(ctx)
	if loginSvc == nil || credRegistry == nil || keysSvc == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "service not available")
		return
	}

	// Resolve slug triple → organization id + application id. Any
	// resolution failure collapses to the same generic 401 as a bad
	// credential — we don't want to tell the caller whether the triple
	// was the problem or the auth material was.
	info, err := loginSvc.Store.FindBySlugs(orgSlug, wsSlug, appSlug)
	if err != nil {
		response.ErrorResponse(w, http.StatusUnauthorized, genericUnauthorized)
		return
	}

	credDB, err := credRegistry.For(info.OrganizationID)
	if err != nil {
		response.ErrorResponse(w, http.StatusUnauthorized, genericUnauthorized)
		return
	}
	repo := repository.New(credDB.Connector.DB)
	cred, err := authn.AuthenticateBasicAuth(
		authn.HeaderFromRequest(r),
		info.ID,
		purpose.SecurityKeyRead,
		time.Now(),
		repo,
	)
	if err != nil {
		response.ErrorResponse(w, http.StatusUnauthorized, genericUnauthorized)
		return
	}

	// Fire-and-forget stamp of last_used_at. A failed stamp is
	// observability loss only — auth already succeeded.
	go func(id string) {
		_ = repo.TouchLastUsed(id)
	}(cred.ID)

	if wantPrivate {
		writePrivateKey(w, keysSvc, info.ID)
		return
	}
	writePublicKeys(w, keysSvc, info.ID)
}

func writePublicKeys(w http.ResponseWriter, keysSvc *keys.Service, appID string) {
	pairs, err := keysSvc.Keypairs(appID, false)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to load keys")
		return
	}
	jwks, err := keysSvc.VerifiableJWKS(appID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to load jwks")
		return
	}
	jwkByKID := make(map[string]map[string]any, len(jwks))
	for _, j := range jwks {
		if kid, ok := j["kid"].(string); ok {
			jwkByKID[kid] = j
		}
	}
	out := make([]publicKeyEntry, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, publicKeyEntry{
			KID:           p.ID,
			Status:        string(p.Status),
			Use:           "sig",
			Alg:           "RS256",
			PublicKeyPEM:  p.PublicPEM,
			JWK:           jwkByKID[p.ID],
			ActivatedAt:   p.ActivatedAt,
			DeactivatedAt: p.DeactivatedAt,
			ExpiresAt:     p.ExpiresAt,
		})
	}
	response.SuccessResponse(w, http.StatusOK, map[string]any{"keys": out})
}

func writePrivateKey(w http.ResponseWriter, keysSvc *keys.Service, appID string) {
	// Active row only, by design — deactivated private keys are
	// never useful for signing and exposing them adds blast radius
	// without upside.
	activeRow, err := keysSvc.ActiveRow(appID)
	if err != nil {
		response.ErrorResponse(w, http.StatusNotFound, "no active signing key")
		return
	}
	pair, err := keysSvc.Keypair(appID, activeRow.ID, true)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to load private key")
		return
	}
	if pair.PrivatePEM == "" {
		response.ErrorResponse(w, http.StatusInternalServerError, "private key material missing on disk")
		return
	}
	activatedAt := time.Time{}
	if pair.ActivatedAt != nil {
		activatedAt = *pair.ActivatedAt
	}
	response.SuccessResponse(w, http.StatusOK, privateKeyEntry{
		KID:           pair.ID,
		Status:        string(pair.Status),
		Alg:           "RS256",
		PrivateKeyPEM: pair.PrivatePEM,
		ActivatedAt:   activatedAt,
		ExpiresAt:     pair.ExpiresAt,
	})
}

// parseSlugSegments extracts (orgSlug, wsSlug, appSlug) from a path
// shaped like `/a/<org>/<ws>/<app>/security-key[/private]`. A malformed
// path returns ok=false; the caller collapses that to a generic 401 so
// path-shape guessing isn't a usable oracle.
func parseSlugSegments(path string) (org, ws, app string, ok bool) {
	// Expected shape: ["", "a", org, ws, app, "security-key", optional "private"]
	parts := splitSegments(path)
	if len(parts) < 5 {
		return "", "", "", false
	}
	if parts[0] != "a" {
		return "", "", "", false
	}
	return parts[1], parts[2], parts[3], true
}

func splitSegments(path string) []string {
	out := make([]string, 0, 8)
	start := 0
	for i := 0; i <= len(path); i++ {
		if i == len(path) || path[i] == '/' {
			if i > start {
				out = append(out, path[start:i])
			}
			start = i + 1
		}
	}
	return out
}

// -- DI resolvers ------------------------------------------------------

type bagGetter interface {
	Get(key string) (interface{}, bool)
}

func resolveLoginPageService(ctx interface{}) *loginpage.Service {
	bag, ok := ctx.(bagGetter)
	if !ok {
		return nil
	}
	raw, ok := bag.Get(loginpage.ContextBagKeyService)
	if !ok {
		return nil
	}
	svc, _ := raw.(*loginpage.Service)
	return svc
}

func resolveCredRegistry(ctx interface{}) *dbregistry.OrgApiCredentialsDBRegistry {
	bag, ok := ctx.(bagGetter)
	if !ok {
		return nil
	}
	raw, ok := bag.Get(dbregistry.ContextBagKey)
	if !ok {
		return nil
	}
	reg, _ := raw.(*dbregistry.OrgApiCredentialsDBRegistry)
	return reg
}

func resolveKeysService(ctx interface{}) *keys.Service {
	bag, ok := ctx.(bagGetter)
	if !ok {
		return nil
	}
	raw, ok := bag.Get(keys.ContextBagKeyService)
	if !ok {
		return nil
	}
	svc, _ := raw.(*keys.Service)
	return svc
}

