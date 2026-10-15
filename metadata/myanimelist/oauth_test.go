package myanimelist

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"golang.org/x/oauth2"
)

// fakeTokens is a MyAnimeList token endpoint that wants the code and its PKCE verifier.
func fakeTokens(t *testing.T, verifier string, form *url.Values) {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		*form = r.PostForm
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.PostForm.Get("grant_type") == "refresh_token" && r.PostForm.Get("refresh_token") == "good-refresh":
			_, _ = w.Write([]byte(`{"access_token":"refreshed","refresh_token":"new-refresh","token_type":"Bearer","expires_in":2678400}`))
		case r.PostForm.Get("code") == "the-code" && r.PostForm.Get("code_verifier") == verifier:
			_, _ = w.Write([]byte(`{"access_token":"exchanged","refresh_token":"refresh","token_type":"Bearer","expires_in":2678400}`))
		default:
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
		}
	}))
	endpoint := OAuthEndpoint
	OAuthEndpoint.TokenURL = s.URL
	t.Cleanup(func() {
		OAuthEndpoint = endpoint
		s.Close()
	})
}

func TestCodeGrantPKCE(t *testing.T) {
	verifier := oauth2.GenerateVerifier()
	var form url.Values
	fakeTokens(t, verifier, &form)
	config := OAuthConfig("id", "", "http://localhost/callback")

	u, err := url.Parse(AuthCodeURL(config, "st", verifier))
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if q.Get("code_challenge") != verifier || q.Get("code_challenge_method") != "plain" || q.Get("response_type") != "code" || q.Get("client_id") != "id" || q.Get("state") != "st" {
		t.Errorf("authorize url %s", u)
	}
	if len(verifier) < 43 || len(verifier) > 128 {
		t.Errorf("verifier %q, MAL wants 43 to 128 chars", verifier)
	}

	token, err := ExchangeCode(config, "the-code", verifier)
	if err != nil || token.AccessToken != "exchanged" || token.RefreshToken != "refresh" {
		t.Fatalf("got %+v, %v", token, err)
	}
	if form.Get("redirect_uri") != "http://localhost/callback" {
		t.Errorf("exchange form %v", form)
	}
	if _, ok := form["client_secret"]; ok {
		t.Error("client_secret sent without one")
	}
	if _, err := ExchangeCode(config, "the-code", "other"); err == nil {
		t.Error("no error with another verifier")
	}
}

func TestRefreshToken(t *testing.T) {
	var form url.Values
	fakeTokens(t, "", &form)
	token, err := RefreshTokenContext(context.Background(), "id", "", "good-refresh")
	if err != nil {
		t.Fatal(err)
	}
	if token.AccessToken != "refreshed" || token.RefreshToken != "new-refresh" || form.Get("client_id") != "id" {
		t.Errorf("token %+v, form %v", token, form)
	}
	if _, err := RefreshToken("id", "", "revoked"); !errors.Is(err, ErrRefreshRejected) {
		t.Errorf("got %v, want ErrRefreshRejected", err)
	}
	if _, err := RefreshToken("", "", "good-refresh"); err == nil {
		t.Error("no error without a client id")
	}
}
