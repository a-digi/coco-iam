package admin

import (
	"net/http"
	"time"

	"github.com/a-digi/coco-iam/src/applications/apicredentials/entity"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// ListHandler serves GET /api/v1/applications/{id}/api-credentials.
// Returns every credential ever issued for this application,
// newest-first. The plaintext secret is never returned — only the
// bcrypt hash lives in the DB and even that is redacted from the
// response (never leaves the handler).
type ListHandler struct{}

// listEntry is the wire shape of one row on the list endpoint. We
// deliberately don't serialise Credential directly: that would
// surface `secret_hash` via the `db:"secret_hash"` → JSON path and
// leak bcrypt material to every admin with the read scope.
type listEntry struct {
	ID         string     `json:"id"`
	APIID      string     `json:"api_id"`
	Label      string     `json:"label"`
	Purposes   []string   `json:"purposes"`
	ExpiresAt  time.Time  `json:"expires_at"`
	IsActive   bool       `json:"is_active"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
}

type listResponse struct {
	Credentials []listEntry `json:"credentials"`
}

func (h *ListHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	appID := appIDFromPath(reqCtx)
	if appID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "missing application id")
		return
	}
	repo, _, ok := openRepoForApp(reqCtx, appID)
	if !ok {
		return
	}
	creds, purposes, err := repo.ListForApplication(appID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]listEntry, 0, len(creds))
	for i, c := range creds {
		out = append(out, toListEntry(c, purposes[i]))
	}
	response.SuccessResponse(w, http.StatusOK, listResponse{Credentials: out})
}

func toListEntry(c entity.Credential, purposes []string) listEntry {
	if purposes == nil {
		purposes = []string{}
	}
	return listEntry{
		ID:         c.ID,
		APIID:      c.APIID,
		Label:      c.Label,
		Purposes:   purposes,
		ExpiresAt:  c.ExpiresAt,
		IsActive:   c.IsActive,
		LastUsedAt: c.LastUsedAt,
		CreatedAt:  c.CreatedAt,
		RevokedAt:  c.RevokedAt,
	}
}
