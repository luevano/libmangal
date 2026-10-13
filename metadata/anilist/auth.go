package anilist

import (
	"context"
	"errors"
	"fmt"

	"github.com/luevano/libmangal/metadata"
)

const (
	OAuthBaseURL      = "https://anilist.co/api/v2/oauth/"
	OAuthTokenURL     = OAuthBaseURL + "token"
	OAuthAuthorizeURL = OAuthBaseURL + "authorize"
)

// Authenticated returns true if the Provider is
// currently authenticated (user logged in).
func (p *Anilist) Authenticated() bool {
	return p.token != ""
}

// Login authorizes an user with the given access token.
// When it fails the provider is logged out.
func (p *Anilist) Login(ctx context.Context, token string) error {
	p.token = token

	user, err := p.getAuthenticatedUser(ctx)
	if err != nil {
		// a failed login logs out, the old user doesn't go with no token
		p.token, p.user = "", nil
		return fmt.Errorf("login: %w", err)
	}
	p.user = user

	return nil
}

// Logout de-authorizes the currently authorized user.
func (p *Anilist) Logout() error {
	if !p.Authenticated() {
		return fmt.Errorf("logout: anilist: %w", metadata.ErrUnauthorized)
	}
	p.user = nil
	p.token = ""
	return nil
}

// getAuthenticatedUser will query for the user data to the Anilist API.
func (p *Anilist) getAuthenticatedUser(ctx context.Context) (metadata.User, error) {
	body := apiRequestBody{
		Query: queryViewer,
	}
	data, err := sendRequest[userData](ctx, p, body)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if data.Viewer == nil {
		return nil, errors.New("get user: anilist: no user in the response")
	}
	return data.Viewer, nil
}
