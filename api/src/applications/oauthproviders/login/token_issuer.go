package login

import (
	"context"

	"github.com/a-digi/coco-iam/src/applications/keys"
	"github.com/a-digi/coco-iam/src/auth/oauth"
	oauth_lib "github.com/a-digi/coco-oauth/oauth"
)

// AppTokenIssuer wraps oauth.IssueAppLoginTokens as a TokenIssuer
// for the login handshake. Production wiring supplies the real
// keys.Service + an oauth_lib.AuthConfig; tests substitute the
// interface directly.
type AppTokenIssuer struct {
	Keys *keys.Service
	Cfg  oauth_lib.AuthConfig
}

// IssueLoginTokens delegates to the existing helper that the
// password flow uses, so bearers issued via OAuth follow the
// same contract external apps already consume.
func (a *AppTokenIssuer) IssueLoginTokens(_ context.Context, appID, userID string, scopes []string, resourceIDs map[string][]string) (string, string, error) {
	resp, err := oauth.IssueAppLoginTokens(a.Keys, appID, a.Cfg, userID, scopes, resourceIDs)
	if err != nil {
		return "", "", err
	}
	return resp.AccessToken, resp.RefreshToken, nil
}
