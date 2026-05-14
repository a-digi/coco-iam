package adapters

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/a-digi/coco-iam/src/applications/oauthproviders/entity"
)

// MicrosoftResolver speaks the "v2.0" Azure AD endpoints for
// consumer + work accounts through the "common" tenant. Admins
// can override authorize_url / token_url in the provider config
// to point at a specific tenant if required.
type MicrosoftResolver struct {
	Client *http.Client
}

const (
	msftDefaultAuthorizeURL = "https://login.microsoftonline.com/common/oauth2/v2.0/authorize"
	msftDefaultTokenURL     = "https://login.microsoftonline.com/common/oauth2/v2.0/token"
	msftDefaultUserinfoURL  = "https://graph.microsoft.com/oidc/userinfo"
)

func (m *MicrosoftResolver) AuthorizeURL(cfg entity.ProviderConfig, state, codeChallenge, redirectURI string) (string, error) {
	u := firstNonEmpty(cfg.AuthorizeURL, msftDefaultAuthorizeURL)
	scopes := cfg.Scopes
	if len(scopes) == 0 {
		scopes = defaultScopes(entity.ProviderMicrosoft)
	}
	q := url.Values{
		"client_id":             {cfg.ClientID},
		"redirect_uri":          {redirectURI},
		"response_type":         {"code"},
		"response_mode":         {"query"},
		"scope":                 {strings.Join(scopes, " ")},
		"state":                 {state},
		"code_challenge":        {codeChallenge},
		"code_challenge_method": {"S256"},
	}
	return u + "?" + q.Encode(), nil
}

func (m *MicrosoftResolver) ExchangeCode(ctx context.Context, cfg entity.ProviderConfig, code, codeVerifier, redirectURI string) (string, string, error) {
	form := url.Values{
		"client_id":     {cfg.ClientID},
		"client_secret": {cfg.ClientSecret},
		"code":          {code},
		"code_verifier": {codeVerifier},
		"redirect_uri":  {redirectURI},
		"grant_type":    {"authorization_code"},
	}
	var out struct {
		AccessToken string `json:"access_token"`
		IDToken     string `json:"id_token"`
	}
	tokenURL := firstNonEmpty(cfg.TokenURL, msftDefaultTokenURL)
	if err := postForm(ctx, m.Client, tokenURL, form, &out); err != nil {
		return "", "", err
	}
	if out.AccessToken == "" {
		return "", "", fmt.Errorf("%w: no access_token", ErrInvalidResponse)
	}
	return out.AccessToken, out.IDToken, nil
}

func (m *MicrosoftResolver) FetchIdentity(ctx context.Context, cfg entity.ProviderConfig, accessToken, _ string) (entity.Identity, error) {
	u := firstNonEmpty(cfg.UserinfoURL, msftDefaultUserinfoURL)
	var out struct {
		Sub        string `json:"sub"`
		Email      string `json:"email"`
		GivenName  string `json:"givenname"`
		FamilyName string `json:"familyname"`
		Picture    string `json:"picture"`
	}
	if err := getJSON(ctx, m.Client, u, accessToken, &out); err != nil {
		return entity.Identity{}, err
	}
	if out.Sub == "" {
		return entity.Identity{}, fmt.Errorf("%w: missing sub", ErrInvalidResponse)
	}
	return entity.Identity{
		Provider: entity.ProviderMicrosoft,
		Sub:      out.Sub,
		Email:    out.Email,
		// Microsoft's OIDC userinfo endpoint always returns
		// verified email addresses (the account's primary),
		// so we treat presence as proof.
		EmailVerified: out.Email != "",
		FirstName:     out.GivenName,
		LastName:      out.FamilyName,
		PictureURL:    out.Picture,
	}, nil
}
