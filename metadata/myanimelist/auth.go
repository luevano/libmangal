package myanimelist

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/luevano/libmangal/metadata"
)

const (
	OAuthBaseURL      = "https://myanimelist.net/v1/oauth2/"
	OAuthTokenURL     = OAuthBaseURL + "token"
	OAuthAuthorizeURL = OAuthBaseURL + "authorize"
)

// Authenticated returns true if the Provider is
// currently authenticated (user logged in).
func (p *MyAnimeList) Authenticated() bool {
	return p.token != ""
}

// Login authorizes an user with the given access token.
func (p *MyAnimeList) Login(ctx context.Context, token string) error {
	p.token = token

	user, err := p.getAuthenticatedUser(ctx)
	if err != nil {
		// remove token as it's possible it's not valid
		p.token = ""
		return fmt.Errorf("login: %w", err)
	}
	p.user = user

	return nil
}

// Logout de-authorizes the currently authorized user.
func (p *MyAnimeList) Logout() error {
	if !p.Authenticated() {
		return fmt.Errorf("logout: mal: %w", metadata.ErrUnauthorized)
	}
	p.user = nil
	p.token = ""
	return nil
}

// getAuthenticatedUser will query for the user data to the MyAnimeList API.
func (p *MyAnimeList) getAuthenticatedUser(ctx context.Context) (metadata.User, error) {
	params := url.Values{}
	params.Set("fields", userFields)

	var user *User
	err := p.request(ctx, http.MethodGet, "users/@me", params, p.commonMangaReqHeaders(), nil, &user)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if user == nil {
		return nil, errors.New("get user: mal: no user in the response")
	}

	return user, nil
}
