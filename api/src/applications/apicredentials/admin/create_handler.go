package admin

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/a-digi/coco-iam/src/applications/apicredentials/entity"
	"github.com/a-digi/coco-iam/src/applications/apicredentials/purpose"
	"github.com/a-digi/coco-iam/src/auth/crypto/bcrypt"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// MaxLifetime is the ceiling on credential expiry. If the admin asks
// for more, the server clamps to this value and reports the effective
// expiry in the response. Matches the plan decision of 1 year.
const MaxLifetime = 365 * 24 * time.Hour

// CreateHandler serves POST /api/v1/applications/{id}/api-credentials.
// Issues a fresh api_id + api_secret, stores the bcrypt hash, and
// returns the plaintext api_secret exactly once in the response body.
//
// The caller MUST capture the plaintext secret from this response —
// it is not retrievable later, by design.
type CreateHandler struct{}

type createPayload struct {
	Label     string    `json:"label"`
	Purposes  []string  `json:"purposes"`
	ExpiresAt time.Time `json:"expires_at"`
}

type createResponse struct {
	// Credential metadata — same shape as a list entry.
	Credential listEntry `json:"credential"`
	// APISecret is the plaintext secret, shown ONCE. The admin UI
	// surfaces it in a copy-to-clipboard modal that warns the user.
	APISecret string `json:"api_secret"`
	// Clamped is true when the requested expiry exceeded MaxLifetime
	// and was reduced on the server side. The UI can surface this
	// so the admin notices the difference between what they asked
	// for and what was granted.
	Clamped bool `json:"clamped,omitempty"`
}

// apiIDEntropy / secretEntropy control the generated credential's
// length. 24 bytes → 32 base64url chars, which is plenty for the api
// id (public) and more than enough for the secret (private).
const (
	apiIDEntropy  = 24
	secretEntropy = 32
)

func (h *CreateHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	appID := appIDFromPath(reqCtx)
	if appID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "missing application id")
		return
	}

	var body createPayload
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	defer r.Body.Close()

	purposes, err := validatePurposes(body.Purposes)
	if err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	expires, clamped, err := clampExpiry(body.ExpiresAt, time.Now())
	if err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	apiID, err := randomToken(apiIDEntropy)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to generate api id")
		return
	}
	secret, err := randomToken(secretEntropy)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to generate api secret")
		return
	}
	hash, err := bcrypt.HashPassword(secret, bcrypt.DefaultCost)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to hash secret")
		return
	}

	repo, _, ok := openRepoForApp(reqCtx, appID)
	if !ok {
		return
	}

	cred := entity.Credential{
		ID:            newUUID(),
		ApplicationID: appID,
		APIID:         apiID,
		SecretHash:    hash,
		Label:         body.Label,
		ExpiresAt:     expires,
		IsActive:      true,
	}
	if err := repo.Insert(cred, purposes); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	// CreatedAt is stamped by SQLite; re-read so the wire has the
	// truth rather than a best-guess client time.
	stored, storedPurposes, err := repo.FindByAPIID(apiID)
	if err == nil && stored != nil {
		cred = *stored
		purposes = storedPurposes
	}

	response.SuccessResponse(w, http.StatusCreated, createResponse{
		Credential: toListEntry(cred, purposes),
		APISecret:  secret,
		Clamped:    clamped,
	})
}

// validatePurposes rejects an empty list and unknown purpose strings.
// A credential with no purposes would be useless; a typo-silently-
// accepted purpose would be a security hole because the admin thinks
// they granted something they didn't.
func validatePurposes(in []string) ([]string, error) {
	if len(in) == 0 {
		return nil, errors.New("purposes must include at least one value")
	}
	known := map[string]bool{
		purpose.SecurityKeyRead.String(): true,
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, p := range in {
		if !known[p] {
			return nil, errors.New("unknown purpose: " + p)
		}
		if seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out, nil
}

// clampExpiry enforces (a) an expiry is provided and (b) it's at most
// MaxLifetime from now. An over-long request is silently reduced; a
// missing or past expiry is rejected outright. Returns (effective,
// clamped, err).
func clampExpiry(requested time.Time, now time.Time) (time.Time, bool, error) {
	if requested.IsZero() {
		return time.Time{}, false, errors.New("expires_at is required")
	}
	if !requested.After(now) {
		return time.Time{}, false, errors.New("expires_at must be in the future")
	}
	max := now.Add(MaxLifetime)
	if requested.After(max) {
		return max, true, nil
	}
	return requested, false, nil
}

// randomToken returns `n` cryptographically-random bytes encoded as a
// url-safe base64 string (no padding). Suitable for both api ids
// (public) and secrets (private) — the function is shared so both
// halves of a credential share the same generator and cost profile.
func randomToken(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// newUUID returns a v4-shaped UUID string. Inline to avoid pulling a
// new module dependency for a single use.
func newUUID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		// Falls back to a zero UUID on an impossible crypto/rand error;
		// the unique index on api_id will still block collisions.
		return "00000000-0000-0000-0000-000000000000"
	}
	buf[6] = (buf[6] & 0x0f) | 0x40 // v4
	buf[8] = (buf[8] & 0x3f) | 0x80 // variant 1
	const hex = "0123456789abcdef"
	out := make([]byte, 36)
	j := 0
	for i, b := range buf {
		switch i {
		case 4, 6, 8, 10:
			out[j] = '-'
			j++
		}
		out[j] = hex[b>>4]
		out[j+1] = hex[b&0x0f]
		j += 2
	}
	return string(out)
}
