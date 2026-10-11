package kitsu

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/luevano/libmangal/metadata"
)

// OAuthTokenURL is the token endpoint, Kitsu only has the password grant
// (and refreshing), no browser authorization.
const OAuthTokenURL = "https://kitsu.app/api/oauth/token"

// Authenticated returns true if the Provider is
// currently authenticated (user logged in).
func (p *Kitsu) Authenticated() bool {
	return p.token != ""
}

// Login authorizes an user with the given access token.
func (p *Kitsu) Login(ctx context.Context, token string) error {
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
func (p *Kitsu) Logout() error {
	if !p.Authenticated() {
		return fmt.Errorf("logout: kitsu: %w", metadata.ErrUnauthorized)
	}
	p.user = nil
	p.token = ""
	return nil
}

// getAuthenticatedUser will query for the user data to the Kitsu API.
func (p *Kitsu) getAuthenticatedUser(ctx context.Context) (*User, error) {
	params := url.Values{}
	params.Set("filter[self]", "true")
	params.Set("fields[users]", "name,slug,about,avatar")

	var doc document
	if err := p.request(ctx, http.MethodGet, "users", params, nil, &doc); err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	users, err := doc.resources()
	if err != nil {
		return nil, fmt.Errorf("get user: kitsu: decode response: %w", err)
	}
	// an invalid token isn't refused, it just has no user
	if len(users) == 0 {
		return nil, fmt.Errorf("get user: kitsu: %w", metadata.ErrUnauthorized)
	}
	user, err := newUser(users[0])
	if err != nil {
		return nil, fmt.Errorf("get user: kitsu: %w", err)
	}
	return user, nil
}
