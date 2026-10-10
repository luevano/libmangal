package myanimelist

import (
	"context"
	"errors"
	"io"
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

func TestSetMangaProgress(t *testing.T) {
	tests := []struct {
		name       string
		listStatus string
		chapter    int
		wantForm   string // "" when no update is expected
	}{
		{"not in list", ``, 3, "num_chapters_read=3&status=reading"},
		{"higher", `,"my_list_status":{"status":"completed","num_chapters_read":10}`, 12, "num_chapters_read=12"},
		{"lower", `,"my_list_status":{"status":"completed","num_chapters_read":10}`, 5, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var form string
			m := newTestMAL(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPatch {
					b, _ := io.ReadAll(r.Body)
					form = string(b)
					_, _ = w.Write([]byte(`{}`))
					return
				}
				_, _ = w.Write([]byte(`{"id":7` + tt.listStatus + `}`))
			})
			m.token = "token"

			if err := m.SetMangaProgress(context.Background(), 7, tt.chapter); err != nil {
				t.Fatal(err)
			}
			if form != tt.wantForm {
				t.Errorf("update %q, want %q", form, tt.wantForm)
			}
		})
	}
}

func TestSearchSkipsNovels(t *testing.T) {
	m := newTestMAL(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"node":{"id":1,"title":"Solo Leveling","media_type":"light_novel"}},{"node":{"id":2,"title":"Solo Leveling","media_type":"manhwa"}}]}`))
	})
	metas, err := m.Search(context.Background(), "solo leveling")
	if err != nil {
		t.Fatal(err)
	}
	if len(metas) != 1 || metas[0].ID().Value() != 2 {
		t.Errorf("got %v, want only the manhwa", metas)
	}
}
