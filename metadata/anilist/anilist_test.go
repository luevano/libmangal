package anilist

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
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

func TestRateLimitRetry(t *testing.T) {
	var requests atomic.Int32
	a := newTestAnilist(t, func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) == 1 {
			// the remaining requests are 0 when rate limited, not a wait time
			w.Header().Set("Retry-After", "1")
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(`{"data":{"page":{"media":[]}}}`))
	})

	start := time.Now()
	if _, err := a.Search(context.Background(), "berserk"); err != nil {
		t.Fatal(err)
	}
	if waited := time.Since(start); waited < time.Second {
		t.Errorf("retried after %s, want the 1s from Retry-After", waited)
	}
	if requests.Load() != 2 {
		t.Errorf("%d requests, want 2", requests.Load())
	}
}

func TestRateLimitRetryCancel(t *testing.T) {
	a := newTestAnilist(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	if _, err := a.Search(ctx, "berserk"); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("got %v, want the context error", err)
	}
}

func TestRateLimitError(t *testing.T) {
	a := newTestAnilist(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "0")
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.WriteHeader(http.StatusTooManyRequests)
	})

	_, err := a.Search(context.Background(), "berserk")
	var rateLimit *RateLimitError
	if !errors.As(err, &rateLimit) || !errors.Is(err, metadata.ErrRateLimited) {
		t.Fatalf("got %v, want a RateLimitError", err)
	}
	if rateLimit.RetryAfterDuration() != 0 {
		t.Errorf("retry after %s, want 0s", rateLimit.RetryAfter)
	}
	if want := `search "berserk": anilist: 429 too many requests (retry after 0s)`; err.Error() != want {
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

func TestSearchByIDNotFound(t *testing.T) {
	a := newTestAnilist(t, func(w http.ResponseWriter, r *http.Request) {
		// what AniList answers for an unknown id (checked 2026-10-09)
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errors":[{"message":"Not Found.","status":404,"locations":[{"line":1,"column":16}]}],"data":{"Media":null}}`))
	})

	meta, found, err := a.SearchByID(context.Background(), 999999999)
	if err != nil || found || meta != nil {
		t.Errorf("got %v found=%t err=%v, want not found", meta, found, err)
	}
}
