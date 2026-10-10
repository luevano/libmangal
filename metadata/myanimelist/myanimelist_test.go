package myanimelist

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/luevano/libmangal/metadata"
)

func newTestMAL(t *testing.T, handler http.HandlerFunc) *MyAnimeList {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	options := DefaultOptions()
	options.ClientID = "test"
	m, err := NewMAL(options)
	if err != nil {
		t.Fatal(err)
	}
	m.apiURL = server.URL + "/v2"
	return m
}

func TestSearchByIDNotFound(t *testing.T) {
	m := newTestMAL(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"","error":"not_found"}`))
	})

	meta, found, err := m.SearchByID(context.Background(), 1)
	if err != nil || found || meta != nil {
		t.Errorf("got %v found=%t err=%v, want not found", meta, found, err)
	}
}

func TestHTTPError(t *testing.T) {
	m := newTestMAL(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid_token","message":"token is expired"}`))
	})

	_, err := m.Search(context.Background(), "berserk")
	if !errors.Is(err, metadata.ErrUnauthorized) {
		t.Fatalf("got %v, want it to match ErrUnauthorized", err)
	}
	if want := `search "berserk": mal: GET /v2/manga: 401 unauthorized: invalid_token: token is expired`; err.Error() != want {
		t.Errorf("got %q, want %q", err, want)
	}
}

func TestSearch(t *testing.T) {
	m := newTestMAL(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-MAL-CLIENT-ID") != "test" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"node":{"id":2,"title":"Berserk","start_date":"1989-08"}}]}`))
	})

	metas, err := m.Search(context.Background(), "berserk")
	if err != nil {
		t.Fatal(err)
	}
	if len(metas) != 1 || metas[0].Title() != "Berserk" || metas[0].StartDate() != (metadata.Date{Year: 1989, Month: 8}) {
		t.Errorf("got %v", metas)
	}
}
