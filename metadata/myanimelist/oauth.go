package myanimelist

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/oauth2"
)

// MyAnimeList's OAuth2 is a code grant with PKCE, for apps logging in a user in the
// browser (the callback server and the browser are the app's):
// https://myanimelist.net/apiconfig/references/authorization
// https://myanimelist.net/blog.php?eid=835707

// OAuthEndpoint is MyAnimeList's OAuth2 endpoint. It's a var so tests can point it
// elsewhere.
var OAuthEndpoint = oauth2.Endpoint{
	AuthURL:   OAuthAuthorizeURL,
	TokenURL:  OAuthTokenURL,
	AuthStyle: oauth2.AuthStyleInParams,
}

// ErrRefreshRejected is a refresh token MyAnimeList rejected: the login has to be done again.
var ErrRefreshRejected = errors.New("oauth: refresh token rejected")

// OAuthConfig is the config of a MyAnimeList API client (clientSecret is empty for the App
// Type "other"), redirectURL must be the client's App Redirect URL.
func OAuthConfig(clientID, clientSecret, redirectURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     OAuthEndpoint,
		RedirectURL:  redirectURL,
	}
}

// AuthCodeURL is the authorize url, with verifier (oauth2.GenerateVerifier) as the PKCE
// code challenge: MyAnimeList only supports the plain one (the challenge is the verifier).
func AuthCodeURL(config *oauth2.Config, state, verifier string) string {
	return config.AuthCodeURL(state,
		oauth2.SetAuthURLParam("code_challenge", verifier),
		oauth2.SetAuthURLParam("code_challenge_method", "plain"),
	)
}

// ExchangeCode calls ExchangeCodeContext with context.Background().
func ExchangeCode(config *oauth2.Config, code, verifier string) (*oauth2.Token, error) {
	return ExchangeCodeContext(context.Background(), config, code, verifier)
}

// ExchangeCodeContext gets the token for the code, with the verifier given to AuthCodeURL.
// The http client is ctx's oauth2.HTTPClient, if any.
func ExchangeCodeContext(ctx context.Context, config *oauth2.Config, code, verifier string) (*oauth2.Token, error) {
	token, err := config.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return nil, fmt.Errorf("exchange the access code: %w", err)
	}
	return token, nil
}

// RefreshToken calls RefreshTokenContext with context.Background().
func RefreshToken(clientID, clientSecret, refreshToken string) (*oauth2.Token, error) {
	return RefreshTokenContext(context.Background(), clientID, clientSecret, refreshToken)
}

// RefreshTokenContext gets a new token with the refresh token. The http client is ctx's
// oauth2.HTTPClient, if any. A rejected refresh token matches ErrRefreshRejected.
func RefreshTokenContext(ctx context.Context, clientID, clientSecret, refreshToken string) (*oauth2.Token, error) {
	if clientID == "" {
		return nil, errors.New("refresh the token: empty client id")
	}
	expired := &oauth2.Token{RefreshToken: refreshToken, Expiry: time.Now().Add(-time.Hour)}
	token, err := OAuthConfig(clientID, clientSecret, "").TokenSource(ctx, expired).Token()
	if err != nil {
		var re *oauth2.RetrieveError
		if errors.As(err, &re) && re.Response != nil && (re.Response.StatusCode == http.StatusBadRequest || re.Response.StatusCode == http.StatusUnauthorized) {
			return nil, fmt.Errorf("%w: %w", ErrRefreshRejected, err)
		}
		return nil, fmt.Errorf("refresh the token: %w", err)
	}
	return token, nil
}
