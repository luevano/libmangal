package kitsu

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/luevano/libmangal/metadata"
)

// OAuthTokenURL is the token endpoint, Kitsu only has the password grant
// (and refreshing), no browser authorization.
const OAuthTokenURL = "https://kitsu.app/api/oauth/token"

// Token is an OAuth token of the Kitsu API.
type Token struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
	// ExpiresIn is in seconds from CreatedAt.
	ExpiresIn int `json:"expires_in"`
	// CreatedAt is a unix timestamp.
	CreatedAt int64 `json:"created_at"`
}

// Expiry is when the access token expires, zero if unknown.
func (t Token) Expiry() time.Time {
	if t.CreatedAt == 0 || t.ExpiresIn == 0 {
		return time.Time{}
	}
	return time.Unix(t.CreatedAt+int64(t.ExpiresIn), 0)
}

// PasswordToken gets a token with the user's email (or name) and password.
// A wrong password matches metadata.ErrUnauthorized.
//
// Like the metadata.Provider methods (Login, Search, ...) it takes a context and has no
// plain variant, same for RefreshToken and LoginPassword.
func (p *Kitsu) PasswordToken(ctx context.Context, username, password string) (Token, error) {
	form := url.Values{}
	form.Set("grant_type", "password")
	form.Set("username", username)
	form.Set("password", password)
	token, err := p.requestToken(ctx, form)
	if err != nil {
		return Token{}, fmt.Errorf("get token: %w", err)
	}
	return token, nil
}

// RefreshToken gets a new token with the refresh token of a previous one.
// It takes a context like the metadata.Provider methods, no plain variant.
func (p *Kitsu) RefreshToken(ctx context.Context, refreshToken string) (Token, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	token, err := p.requestToken(ctx, form)
	if err != nil {
		return Token{}, fmt.Errorf("refresh token: %w", err)
	}
	return token, nil
}

// LoginPassword gets a token with the user's email (or name) and password and
// logs in with it. The token is returned to be saved (Login, RefreshToken).
// It takes a context like the metadata.Provider methods, no plain variant.
func (p *Kitsu) LoginPassword(ctx context.Context, username, password string) (Token, error) {
	token, err := p.PasswordToken(ctx, username, password)
	if err != nil {
		return Token{}, fmt.Errorf("login: %w", err)
	}
	if err := p.Login(ctx, token.AccessToken); err != nil {
		return Token{}, err
	}
	return token, nil
}

func (p *Kitsu) requestToken(ctx context.Context, form url.Values) (Token, error) {
	if p.options.ClientID != "" {
		form.Set("client_id", p.options.ClientID)
		form.Set("client_secret", p.options.ClientSecret)
	}
	encoded := form.Encode()

	var token Token
	err := p.send(ctx, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.tokenURL, strings.NewReader(encoded))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")
		return req, nil
	}, &token)
	if err != nil {
		return Token{}, err
	}
	if token.AccessToken == "" {
		return Token{}, errors.New("kitsu: no access token in the response")
	}
	return token, nil
}

// Authenticated returns true if the Provider is
// currently authenticated (user logged in).
func (p *Kitsu) Authenticated() bool {
	return p.token != ""
}

// Login authorizes an user with the given access token.
// When it fails the provider is logged out.
func (p *Kitsu) Login(ctx context.Context, token string) error {
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
