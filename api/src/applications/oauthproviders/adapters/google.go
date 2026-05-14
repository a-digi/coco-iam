package adapters

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/a-digi/coco-iam/src/applications/oauthproviders/entity"
)

// GoogleResolver speaks Google's OIDC flow. Uses the endpoints
// baked in by default but respects admin overrides on
// entity.ProviderConfig so a test / enterprise tenant can point
// at a non-default issuer.
type GoogleResolver struct {
	Client *http.Client
}

const (
	googleDefaultAuthorizeURL = "https://accounts.google.com/o/oauth2/v2/auth"
	googleDefaultTokenURL     = "https://oauth2.googleapis.com/token"
	googleDefaultUserinfoURL  = "https://openidconnect.googleapis.com/v1/userinfo"
)

func (g *GoogleResolver) AuthorizeURL(cfg entity.ProviderConfig, state, codeChallenge, redirectURI string) (string, error) {
	u := firstNonEmpty(cfg.AuthorizeURL, googleDefaultAuthorizeURL)
	scopes := cfg.Scopes
	if len(scopes) == 0 {
		scopes = defaultScopes(entity.ProviderGoogle)
	}
	q := url.Values{
		"client_id":             {cfg.ClientID},
		"redirect_uri":          {redirectURI},
		"response_type":         {"code"},
		"scope":                 {strings.Join(scopes, " ")},
		"state":                 {state},
		"code_challenge":        {codeChallenge},
		"code_challenge_method": {"S256"},
		"access_type":           {"online"},
		"prompt":                {"select_account"},
	}
	return u + "?" + q.Encode(), nil
}

func (g *GoogleResolver) ExchangeCode(ctx context.Context, cfg entity.ProviderConfig, code, codeVerifier, redirectURI string) (string, string, error) {
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
	tokenURL := firstNonEmpty(cfg.TokenURL, googleDefaultTokenURL)
	if err := postForm(ctx, g.Client, tokenURL, form, &out); err != nil {
		return "", "", err
	}
	if out.AccessToken == "" {
		return "", "", fmt.Errorf("%w: no access_token", ErrInvalidResponse)
	}
	return out.AccessToken, out.IDToken, nil
}

func (g *GoogleResolver) FetchIdentity(ctx context.Context, cfg entity.ProviderConfig, accessToken, _ string) (entity.Identity, error) {
	u := firstNonEmpty(cfg.UserinfoURL, googleDefaultUserinfoURL)
	var out struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		GivenName     string `json:"given_name"`
		FamilyName    string `json:"family_name"`
		Picture       string `json:"picture"`
	}
	if err := getJSON(ctx, g.Client, u, accessToken, &out); err != nil {
		return entity.Identity{}, err
	}
	if out.Sub == "" {
		return entity.Identity{}, fmt.Errorf("%w: missing sub", ErrInvalidResponse)
	}
	return entity.Identity{
		Provider:      entity.ProviderGoogle,
		Sub:           out.Sub,
		Email:         out.Email,
		EmailVerified: out.EmailVerified,
		FirstName:     out.GivenName,
		LastName:      out.FamilyName,
		PictureURL:    out.Picture,
	}, nil
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}
