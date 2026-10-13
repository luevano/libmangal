package myanimelist

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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
			_ = m.session.Login("token", func() (metadata.User, error) { return nil, nil })

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

func TestCapQuery(t *testing.T) {
	long := strings.Repeat("あ", 70)
	if got := []rune(capQuery(long)); len(got) != maxQueryRunes {
		t.Errorf("got %d runes", len(got))
	}
	if got := capQuery("One Piece"); got != "One Piece" {
		t.Errorf("got %q", got)
	}
}

func TestInvalidClientID(t *testing.T) {
	err := fmt.Errorf("search: %w", &HTTPError{StatusCode: http.StatusBadRequest, Message: "invalid_request: Invalid client id"})
	if !errors.Is(err, ErrInvalidClientID) || errors.Is(err, metadata.ErrUnauthorized) {
		t.Errorf("got %v", err)
	}
	if errors.Is(&HTTPError{StatusCode: http.StatusBadRequest, Message: "bad q"}, ErrInvalidClientID) {
		t.Error("any 400 isn't a bad client id")
	}
}

func TestFailedLoginLogsOut(t *testing.T) {
	m := newTestMAL(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer good" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"invalid_token"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":1,"name":"david"}`))
	})
	if err := m.Login(context.Background(), "good"); err != nil || m.User() == nil {
		t.Fatalf("login: %v, user %v", err, m.User())
	}
	if err := m.Login(context.Background(), "bad"); err == nil || m.Authenticated() || m.User() != nil {
		t.Errorf("got %v authenticated=%t user %v, want logged out", err, m.Authenticated(), m.User())
	}
}

func TestUserAgent(t *testing.T) {
	var ua string
	m := newTestMAL(t, func(w http.ResponseWriter, r *http.Request) {
		ua = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	if _, err := m.Search(context.Background(), "berserk"); err != nil {
		t.Fatal(err)
	}
	if ua != DefaultUserAgent {
		t.Errorf("user agent %q, want %q", ua, DefaultUserAgent)
	}
}

func TestRateLimitRetry(t *testing.T) {
	var bodies []string
	m := newTestMAL(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))
		if len(bodies) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(`{"data":[]}`))
	})

	headers := http.Header{}
	err := m.request(context.Background(), http.MethodPatch, "manga/1/my_list_status", nil, headers, strings.NewReader("num_chapters_read=3"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(bodies) != 2 || bodies[1] != "num_chapters_read=3" {
		t.Errorf("requests %q, want the body sent twice", bodies)
	}
}

func TestHugeErrorBody(t *testing.T) {
	m := newTestMAL(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"` + strings.Repeat("a", 1<<20) + `"}`))
	})
	_, err := m.Search(context.Background(), "berserk")
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) || len(httpErr.Message) > maxErrorBody {
		t.Errorf("got a %d bytes error", len(err.Error()))
	}
}

func TestLoginUser(t *testing.T) {
	m := newTestMAL(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/users/@me" || r.Header.Get("Authorization") != "Bearer token" {
			t.Errorf("got %s with %q", r.URL.Path, r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`{"id":1,"name":"david","picture":"avatar.png"}`))
	})
	if err := m.Logout(); !errors.Is(err, metadata.ErrUnauthorized) {
		t.Errorf("logout before login: %v", err)
	}
	if err := m.Login(context.Background(), "token"); err != nil || !m.Authenticated() {
		t.Fatalf("login: %v", err)
	}
	u := m.User()
	if u.String() != "david (1)" || u.About() != "" || u.Avatar() != "avatar.png" || u.URL() != "https://myanimelist.net/profile/david" || u.Source() != metadata.IDSourceMyAnimeList {
		t.Errorf("user %q %q %q %q", u.String(), u.Avatar(), u.URL(), u.Source())
	}
	if err := m.Logout(); err != nil || m.Authenticated() || m.User() != nil {
		t.Errorf("logout: %v, authenticated=%t", err, m.Authenticated())
	}
}

func TestSearchByID(t *testing.T) {
	m := newTestMAL(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/manga/2" {
			t.Errorf("path %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"id":2,"title":"Berserk",
			"main_picture":{"medium":"medium.jpg"},
			"alternative_titles":{"synonyms":["Berserk: The Prototype"],"en":"Berserk","ja":"ベルセルク"},
			"start_date":"1989-08-25","synopsis":"Guts","mean":9.47,"nsfw":"gray",
			"genres":[{"id":1,"name":"Action"}],"media_type":"manga","status":"on_hiatus","num_chapters":0,
			"authors":[{"node":{"first_name":"Kentarou","last_name":"Miura"},"role":"Story & Art"},{"node":{"first_name":"Kouji","last_name":"Mori"},"role":"Story"}]}`))
	})

	meta, found, err := m.SearchByID(context.Background(), 2)
	if err != nil || !found {
		t.Fatalf("got found=%t, %v", found, err)
	}
	if err := metadata.Validate(meta); err != nil {
		t.Fatal(err)
	}
	if meta.String() != "Berserk (1989) [malid-2]" || len(meta.AlternateTitles()) != 3 || meta.Cover() != "medium.jpg" || meta.Banner() != "" {
		t.Errorf("got %q, titles %q, cover %q", meta.String(), meta.AlternateTitles(), meta.Cover())
	}
	if fmt.Sprint(meta.Authors()) != "[Kentarou Miura Kouji Mori]" || fmt.Sprint(meta.Artists()) != "[Kentarou Miura]" || meta.Translators() != nil || meta.Letterers() != nil {
		t.Errorf("authors %q, artists %q", meta.Authors(), meta.Artists())
	}
	if fmt.Sprint(meta.Genres()) != "[Action]" || len(meta.Tags()) != 0 || len(meta.Characters()) != 0 || meta.Description() != "Guts" || meta.Score() < 4.7 || meta.Score() > 4.8 {
		t.Errorf("genres %q, description %q, score %v", meta.Genres(), meta.Description(), meta.Score())
	}
	if meta.Status() != metadata.StatusHiatus || meta.Country() != "JP" || meta.URL() != "https://myanimelist.net/manga/2" || meta.EndDate() != (metadata.Date{}) || len(meta.ExtraIDs()) != 0 {
		t.Errorf("status %q, country %q, url %q, end %v", meta.Status(), meta.Country(), meta.URL(), meta.EndDate())
	}
}
