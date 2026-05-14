package adapters

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/a-digi/coco-iam/src/applications/oauthproviders/entity"
)

// GitHubResolver speaks GitHub's OAuth 2.0 flow. GitHub is not
// OIDC-compliant — no id_token, userinfo is the separate
// /user + /user/emails REST pair — so the userinfo branch
// reaches two endpoints and merges the result.
type GitHubResolver struct {
	Client *http.Client
}

const (
	githubDefaultAuthorizeURL = "https://github.com/login/oauth/authorize"
	githubDefaultTokenURL     = "https://github.com/login/oauth/access_token"
	githubDefaultUserinfoURL  = "https://api.github.com/user"
	githubEmailsURL           = "https://api.github.com/user/emails"
)

func (g *GitHubResolver) AuthorizeURL(cfg entity.ProviderConfig, state, codeChallenge, redirectURI string) (string, error) {
	u := firstNonEmpty(cfg.AuthorizeURL, githubDefaultAuthorizeURL)
	scopes := cfg.Scopes
	if len(scopes) == 0 {
		scopes = defaultScopes(entity.ProviderGitHub)
	}
	q := url.Values{
		"client_id":     {cfg.ClientID},
		"redirect_uri":  {redirectURI},
		"scope":         {strings.Join(scopes, " ")},
		"state":         {state},
		"code_challenge":        {codeChallenge},
		"code_challenge_method": {"S256"},
		"allow_signup":  {"true"},
	}
	return u + "?" + q.Encode(), nil
}

func (g *GitHubResolver) ExchangeCode(ctx context.Context, cfg entity.ProviderConfig, code, codeVerifier, redirectURI string) (string, string, error) {
	form := url.Values{
		"client_id":     {cfg.ClientID},
		"client_secret": {cfg.ClientSecret},
		"code":          {code},
		"code_verifier": {codeVerifier},
		"redirect_uri":  {redirectURI},
	}
	var out struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	tokenURL := firstNonEmpty(cfg.TokenURL, githubDefaultTokenURL)
	if err := postForm(ctx, g.Client, tokenURL, form, &out); err != nil {
		return "", "", err
	}
	if out.Error != "" || out.AccessToken == "" {
		return "", "", fmt.Errorf("%w: github: %s %s", ErrTokenExchange, out.Error, out.ErrorDesc)
	}
	return out.AccessToken, "", nil
}

// FetchIdentity pulls the primary profile from /user and, if the
// profile's email field is empty (because the user hid their
// public email), falls back to /user/emails to find a verified
// primary address.
func (g *GitHubResolver) FetchIdentity(ctx context.Context, cfg entity.ProviderConfig, accessToken, _ string) (entity.Identity, error) {
	userURL := firstNonEmpty(cfg.UserinfoURL, githubDefaultUserinfoURL)
	var user struct {
		ID    int64  `json:"id"`
		Login string `json:"login"`
		Name  string `json:"name"`
		Email string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := getJSON(ctx, g.Client, userURL, accessToken, &user); err != nil {
		return entity.Identity{}, err
	}
	if user.ID == 0 {
		return entity.Identity{}, fmt.Errorf("%w: missing user id", ErrInvalidResponse)
	}

	email := user.Email
	verified := false
	if email == "" {
		// Fetch /user/emails. Errors here are non-fatal; we just
		// end up with an unverified-email identity and the login
		// decision layer handles it.
		var emails []struct {
			Email    string `json:"email"`
			Primary  bool   `json:"primary"`
			Verified bool   `json:"verified"`
		}
		if err := getJSON(ctx, g.Client, githubEmailsURL, accessToken, &emails); err == nil {
			for _, e := range emails {
				if e.Primary {
					email = e.Email
					verified = e.Verified
					break
				}
			}
		}
	} else {
		// GitHub's /user doesn't tell us verified-ness for the
		// profile email — treat a surfaced email as verified
		// (GitHub would have refused to show an unverified one
		// at that endpoint). Mirror the same fallback to
		// /user/emails if the user wants to be explicit.
		verified = true
	}

	first, last := splitName(user.Name)
	return entity.Identity{
		Provider:      entity.ProviderGitHub,
		Sub:           fmt.Sprintf("%d", user.ID),
		Email:         email,
		EmailVerified: verified,
		FirstName:     first,
		LastName:      last,
		PictureURL:    user.AvatarURL,
	}, nil
}

// splitName splits "First Last" on the first space; GitHub's
// profile "name" is a single free-text field.
func splitName(full string) (string, string) {
	full = strings.TrimSpace(full)
	if full == "" {
		return "", ""
	}
	idx := strings.Index(full, " ")
	if idx < 0 {
		return full, ""
	}
	return strings.TrimSpace(full[:idx]), strings.TrimSpace(full[idx+1:])
}
