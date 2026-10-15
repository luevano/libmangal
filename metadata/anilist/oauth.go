package anilist

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

// AniList's OAuth2, for apps logging in a user in the browser (the callback server and
// the browser are the app's):
// https://anilist.gitbook.io/anilist-apiv2-docs/overview/oauth/getting-started

// OAuthEndpoint is AniList's OAuth2 endpoint. It's a var so tests can point it elsewhere.
var OAuthEndpoint = oauth2.Endpoint{
	AuthURL:   OAuthAuthorizeURL,
	TokenURL:  OAuthTokenURL,
	AuthStyle: oauth2.AuthStyleInParams,
}

// ImplicitTokenExpiry is how long AniList's implicit grant tokens last.
const ImplicitTokenExpiry = 365 * 24 * time.Hour

// OAuthConfig is the code grant config of an AniList API client, redirectURL must be the
// client's redirect URL.
func OAuthConfig(clientID, clientSecret, redirectURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     OAuthEndpoint,
		RedirectURL:  redirectURL,
	}
}

// ImplicitGrantURL is the authorize url of an implicit grant (a client used without its
// secret): response_type token and no redirect uri, AniList uses the client's. The
// token comes back in the redirect's fragment (#access_token=...&expires_in=...).
func ImplicitGrantURL(clientID, state string) string {
	v := url.Values{
		"response_type": {"token"},
		"client_id":     {clientID},
		"state":         {state},
	}
	sep := "?"
	if strings.Contains(OAuthEndpoint.AuthURL, "?") {
		sep = "&"
	}
	return OAuthEndpoint.AuthURL + sep + v.Encode()
}

// ImplicitToken is the token of an implicit grant's redirect, expiresIn 0 (not said) is
// ImplicitTokenExpiry.
func ImplicitToken(accessToken string, expiresIn time.Duration) *oauth2.Token {
	if expiresIn <= 0 {
		expiresIn = ImplicitTokenExpiry
	}
	return &oauth2.Token{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		Expiry:      time.Now().Add(expiresIn),
	}
}

// ExchangeCode calls ExchangeCodeContext with context.Background().
func ExchangeCode(config *oauth2.Config, code string) (*oauth2.Token, error) {
	return ExchangeCodeContext(context.Background(), config, code)
}

// ExchangeCodeContext gets the token for a code grant's code (config from OAuthConfig).
// The http client is ctx's oauth2.HTTPClient, if any.
func ExchangeCodeContext(ctx context.Context, config *oauth2.Config, code string) (*oauth2.Token, error) {
	token, err := config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchange the access code: %w", err)
	}
	return token, nil
}
