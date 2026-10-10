package anilist

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/luevano/libmangal/metadata"
)

func newTestAnilist(t *testing.T, handler http.HandlerFunc) *Anilist {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	a, err := NewAnilist(DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	a.apiURL = server.URL
	return a
}

func TestRateLimitError(t *testing.T) {
	a := newTestAnilist(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.WriteHeader(http.StatusTooManyRequests)
	})

	_, err := a.Search(context.Background(), "berserk")
	var rateLimit *RateLimitError
	if !errors.As(err, &rateLimit) || !errors.Is(err, metadata.ErrRateLimited) {
		t.Fatalf("got %v, want a RateLimitError", err)
	}
	if rateLimit.RetryAfterDuration() != 30*time.Second {
		t.Errorf("retry after %s, want 30s", rateLimit.RetryAfter)
	}
	if want := `search "berserk": anilist: 429 too many requests (retry after 30s)`; err.Error() != want {
		t.Errorf("got %q, want %q", err, want)
	}
}

func TestHTTPError(t *testing.T) {
	a := newTestAnilist(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"errors":[{"message":"Invalid token","status":400}],"data":null}`))
	})

	_, err := a.Search(context.Background(), "berserk")
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("got %v, want a 400 HTTPError", err)
	}
	if want := `search "berserk": anilist: 400 bad request: Invalid token`; err.Error() != want {
		t.Errorf("got %q, want %q", err, want)
	}
}

func TestSearch(t *testing.T) {
	a := newTestAnilist(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"page":{"media":[{"id":7,"title":{"english":"Berserk"}}]}}}`))
	})

	metas, err := a.Search(context.Background(), "berserk")
	if err != nil {
		t.Fatal(err)
	}
	if len(metas) != 1 || metas[0].Title() != "Berserk" || metas[0].ID().Value() != 7 {
		t.Errorf("got %v", metas)
	}
}
