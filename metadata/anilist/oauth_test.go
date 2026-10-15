package anilist

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestImplicitGrantURL(t *testing.T) {
	u, err := url.Parse(ImplicitGrantURL("id", "st"))
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if u.Scheme+"://"+u.Host+u.Path != OAuthAuthorizeURL || q.Get("response_type") != "token" || q.Get("client_id") != "id" || q.Get("state") != "st" || q.Has("redirect_uri") {
		t.Errorf("got %s", u)
	}
}

func TestImplicitToken(t *testing.T) {
	if token := ImplicitToken("tok", time.Hour); token.AccessToken != "tok" || token.TokenType != "Bearer" || time.Until(token.Expiry) > time.Hour {
		t.Errorf("got %+v", token)
	}
	if token := ImplicitToken("tok", 0); time.Until(token.Expiry) < ImplicitTokenExpiry-time.Minute {
		t.Errorf("no expiry: got %s", token.Expiry)
	}
}

func TestExchangeCode(t *testing.T) {
	var form url.Values
	tokens := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		form = r.PostForm
		if r.PostForm.Get("code") != "the-code" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"exchanged","token_type":"Bearer","expires_in":3600}`))
	}))
	defer tokens.Close()
	endpoint := OAuthEndpoint
	OAuthEndpoint.TokenURL = tokens.URL
	defer func() { OAuthEndpoint = endpoint }()

	config := OAuthConfig("id", "secret", "http://localhost/callback")
	token, err := ExchangeCode(config, "the-code")
	if err != nil || token.AccessToken != "exchanged" {
		t.Fatalf("got %+v, %v", token, err)
	}
	if form.Get("client_id") != "id" || form.Get("client_secret") != "secret" || form.Get("redirect_uri") != "http://localhost/callback" {
		t.Errorf("form %v", form)
	}
	if _, err := ExchangeCode(config, "evil"); err == nil {
		t.Error("no error for a bad code")
	}
}
