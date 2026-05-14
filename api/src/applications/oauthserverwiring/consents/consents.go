// Package consents serves the user-facing "Connected apps"
// endpoints: list the OAuth clients this user has consented
// to, plus revoke a specific (user, client) consent.
//
// Mounted under
//
//	/a/{orgSlug}/{wsSlug}/{appSlug}/profile/me/consents
//
// so the user authenticates with the same RS256 bearer the
// /profile/me endpoint already issues.
package consents

import (
	"context"
	"crypto/rsa"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/a-digi/coco-iam/src/applications/oauthserverwiring"
	"github.com/a-digi/coco-iam/src/applications/userprofile"
	oauth_sqlstore "github.com/a-digi/coco-iam/src/oauthserver/sqlstore"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// Deps bundles the collaborators both handlers need. Built once
// at startup; the routes file constructs one Deps + two
// handlers that share it.
type Deps struct {
	Slugs   userprofile.SlugResolver
	Keys    userprofile.KeyLoader
	Users   userprofile.UserOrgReader
	UsersDB func(orgID string) (*sql.DB, error)
	MainDB  *sql.DB
	Now     func() time.Time
}

// ListedConsent is the wire row for /consents.
type ListedConsent struct {
	ClientRowID   string   `json:"client_row_id"`
	ClientID      string   `json:"client_id"`
	DisplayName   string   `json:"display_name"`
	GrantedScopes []string `json:"granted_scopes"`
	GrantedAt     string   `json:"granted_at"`
}

type listResponse struct {
	Consents []ListedConsent `json:"consents"`
}

// ListHandler serves GET .../consents.
type ListHandler struct{ Deps }

// RevokeHandler serves DELETE .../consents/{clientRowId}.
type RevokeHandler struct{ Deps }

func (h *ListHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	userID, appID, orgID, ok := h.authenticate(reqCtx)
	if !ok {
		return
	}

	usersDB, err := h.UsersDB(orgID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	rows, err := usersDB.QueryContext(r.Context(),
		`SELECT client_row_id, granted_scopes, granted_at
		 FROM oauth_user_consents
		 WHERE user_id = ? AND revoked_at IS NULL
		 ORDER BY granted_at DESC`,
		userID,
	)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	clientRepo := oauth_sqlstore.NewClientRepo(h.MainDB, oauthserverwiring.NewBcryptHasher(0))
	out := []ListedConsent{}
	for rows.Next() {
		var (
			clientRowID, scopesRaw, grantedAt string
		)
		if err := rows.Scan(&clientRowID, &scopesRaw, &grantedAt); err != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		var scopes []string
		if scopesRaw != "" {
			_ = json.Unmarshal([]byte(scopesRaw), &scopes)
		}
		// Look up the client row in the main DB so we can
		// surface a user-friendly name. A missing client row
		// (deleted by the admin since the user consented)
		// renders as "(removed app)" rather than failing the
		// whole list.
		display := "(removed app)"
		clientID := ""
		if c, err := clientRepo.FindByID(r.Context(), appID, clientRowID); err == nil && c != nil {
			display = c.DisplayName
			if display == "" {
				display = c.ClientID
			}
			clientID = c.ClientID
		}
		out = append(out, ListedConsent{
			ClientRowID:   clientRowID,
			ClientID:      clientID,
			DisplayName:   display,
			GrantedScopes: scopes,
			GrantedAt:     grantedAt,
		})
	}
	response.SuccessResponse(w, http.StatusOK, listResponse{Consents: out})
}

func (h *RevokeHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	userID, _, orgID, ok := h.authenticate(reqCtx)
	if !ok {
		return
	}
	clientRowID := pathSegmentAfter(r.URL.Path, "consents")
	if clientRowID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "missing client id")
		return
	}
	usersDB, err := h.UsersDB(orgID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	now := nowFn(h.Now).UTC().Format(time.RFC3339)
	if _, err := usersDB.ExecContext(r.Context(),
		`UPDATE oauth_user_consents SET revoked_at = ?
		 WHERE user_id = ? AND client_row_id = ? AND revoked_at IS NULL`,
		now, userID, clientRowID,
	); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, map[string]any{"ok": true})
}

// authenticate runs the same bearer check the /profile/me
// handler uses (via the now-exported userprofile.AuthenticateUser).
// On failure it writes the 401 and returns ok=false.
func (h *Deps) authenticate(reqCtx request.RequestContext) (userID, appID, orgID string, ok bool) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	if h.Slugs == nil || h.Keys == nil || h.Users == nil ||
		h.UsersDB == nil || h.MainDB == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "consents handler not configured")
		return "", "", "", false
	}
	orgSlug, wsSlug, appSlug, parsed := parseSlugSegments(r.URL.Path)
	if !parsed {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid URL")
		return "", "", "", false
	}
	appID, orgID, err := h.Slugs.ResolveSlugs(orgSlug, wsSlug, appSlug)
	if err != nil {
		response.ErrorResponse(w, http.StatusUnauthorized, "unauthorized")
		return "", "", "", false
	}
	loadKey := userprofile.LoadPublicKeyFunc(func(kid string) (*rsa.PublicKey, error) {
		return h.Keys.LoadPublicKey(appID, kid)
	})
	userOrg := userprofile.UserOrgLookupFunc(h.Users.UserOrg)
	uid, authErr := userprofile.AuthenticateUser(
		r.Header.Get("Authorization"),
		orgID,
		loadKey,
		userOrg,
		nowFn(h.Now),
	)
	if authErr != nil {
		response.ErrorResponse(w, http.StatusUnauthorized, "unauthorized")
		return "", "", "", false
	}
	return uid, appID, orgID, true
}

// parseSlugSegments matches /a/<org>/<ws>/<app>/...
func parseSlugSegments(path string) (org, ws, app string, ok bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 4 || parts[0] != "a" {
		return "", "", "", false
	}
	return parts[1], parts[2], parts[3], true
}

// pathSegmentAfter returns the next path segment after marker.
func pathSegmentAfter(path, marker string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i := 0; i+1 < len(parts); i++ {
		if parts[i] == marker {
			return strings.TrimSpace(parts[i+1])
		}
	}
	return ""
}

func nowFn(f func() time.Time) time.Time {
	if f != nil {
		return f()
	}
	return time.Now()
}

// keep context referenced for future cancellation needs.
var _ = context.Background
