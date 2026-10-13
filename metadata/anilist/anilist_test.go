package anilist

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/luevano/libmangal/metadata"
	"github.com/luevano/libmangal/progress"
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

	var reported []string
	ctx := progress.WithReporter(context.Background(), func(s progress.Status) { reported = append(reported, s.String()) })
	start := time.Now()
	if _, err := a.Search(ctx, "berserk"); err != nil {
		t.Fatal(err)
	}
	if want := "AniList rate limited, retrying in 1s"; len(reported) != 1 || reported[0] != want {
		t.Errorf("reported %q, want %q", reported, want)
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
		w.Header().Set("Retry-After", "1")
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.WriteHeader(http.StatusTooManyRequests)
	})

	_, err := a.Search(context.Background(), "berserk")
	var rateLimit *RateLimitError
	if !errors.As(err, &rateLimit) || !errors.Is(err, metadata.ErrRateLimited) {
		t.Fatalf("got %v, want a RateLimitError", err)
	}
	if rateLimit.RetryAfterDuration() != time.Second {
		t.Errorf("retry after %s, want 1s", rateLimit.RetryAfter)
	}
	if want := `search "berserk": anilist: 429 too many requests (retry after 1s)`; err.Error() != want {
		t.Errorf("got %q, want %q", err, want)
	}
}

func TestRetryAfter(t *testing.T) {
	future := strconv.FormatInt(time.Now().Add(30*time.Second).Unix(), 10)
	past := strconv.FormatInt(time.Now().Add(-time.Minute).Unix(), 10)
	for _, c := range []struct {
		retryAfter, reset string
		min, max          time.Duration
	}{
		{"5", "", 5 * time.Second, 5 * time.Second},
		// "retry after 0s" while limited: the reset, else a minute
		{"0", future, 29 * time.Second, 31 * time.Second},
		{"0", "", time.Minute, time.Minute},
		{"0", past, time.Minute, time.Minute},
		{"", "", time.Minute, time.Minute},
		// capped
		{"3600", "", 5 * time.Minute, 5 * time.Minute},
		{"0", strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10), 5 * time.Minute, 5 * time.Minute},
		{time.Now().Add(20 * time.Second).UTC().Format(http.TimeFormat), "", 19 * time.Second, 21 * time.Second},
	} {
		h := http.Header{}
		if c.retryAfter != "" {
			h.Set("Retry-After", c.retryAfter)
		}
		if c.reset != "" {
			h.Set("X-RateLimit-Reset", c.reset)
		}
		if got := retryAfter(h); got < c.min || got > c.max {
			t.Errorf("Retry-After %q, reset %q: %s, want %s-%s", c.retryAfter, c.reset, got, c.min, c.max)
		}
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

func TestLoginInvalidToken(t *testing.T) {
	a := newTestAnilist(t, func(w http.ResponseWriter, r *http.Request) {
		// what AniList answers for a bad token (checked 2026-10-10)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"data":null,"errors":[{"message":"Invalid token","status":400}]}`))
	})

	err := a.Login(context.Background(), "expired")
	if !errors.Is(err, metadata.ErrUnauthorized) || a.Authenticated() {
		t.Errorf("got %v authenticated=%t, want unauthorized", err, a.Authenticated())
	}
}

func TestSetMangaProgress(t *testing.T) {
	tests := []struct {
		name       string
		entry      string // mediaListEntry json
		chapter    int
		wantStatus bool // mutation sets the status
		wantSave   bool
	}{
		{"not in list", `null`, 3, true, true},
		{"higher", `{"progress":10,"status":"COMPLETED"}`, 12, false, true},
		{"lower", `{"progress":10,"status":"COMPLETED"}`, 5, false, false},
		{"same", `{"progress":10,"status":"CURRENT"}`, 10, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mutations []string
			a := newTestAnilist(t, func(w http.ResponseWriter, r *http.Request) {
				var body apiRequestBody
				_ = json.NewDecoder(r.Body).Decode(&body)
				if strings.Contains(body.Query, "mediaListEntry") {
					_, _ = w.Write([]byte(`{"data":{"Media":{"mediaListEntry":` + tt.entry + `}}}`))
					return
				}
				mutations = append(mutations, body.Query)
				_, _ = w.Write([]byte(`{"data":{"SaveMediaListEntry":{"id":1}}}`))
			})
			_ = a.session.Login("token", func() (metadata.User, error) { return nil, nil })

			if err := a.SetMangaProgress(context.Background(), 7, tt.chapter); err != nil {
				t.Fatal(err)
			}
			if saved := len(mutations) == 1; saved != tt.wantSave {
				t.Fatalf("saved %t, want %t", saved, tt.wantSave)
			}
			if tt.wantSave {
				if setsStatus := strings.Contains(mutations[0], "status:"); setsStatus != tt.wantStatus {
					t.Errorf("mutation sets the status %t, want %t", setsStatus, tt.wantStatus)
				}
			}
		})
	}
}

func TestSearchMany(t *testing.T) {
	var requests int
	a := newTestAnilist(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		var body apiRequestBody
		_ = json.NewDecoder(r.Body).Decode(&body)
		// answer every aliased search with one manga named after its query
		out := map[string]any{}
		for name, q := range body.Variables {
			out[name] = map[string]any{"media": []any{map[string]any{"id": len(q.(string)), "title": map[string]any{"english": q}}}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": out})
	})

	queries := []string{"a", "bb", "ccc", "dddd", "eeeee", "ffffff", "ggggggg", "hhhhhhhh", "iiiiiiiii", "jjjjjjjjjj"}
	results, err := a.SearchMany(context.Background(), queries)
	if err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Errorf("%d requests, want 2 (8 searches per request)", requests)
	}
	for i, q := range queries {
		if len(results[i]) != 1 || results[i][0].Title() != q {
			t.Errorf("query %q: got %v", q, results[i])
		}
	}
}

func TestNullMedia(t *testing.T) {
	a := newTestAnilist(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"page":{"media":[null,{"id":7,"title":{"english":"Berserk"}}]}}}`))
	})

	metas, err := a.Search(context.Background(), "berserk")
	if err != nil || len(metas) != 1 || metas[0].ID().Value() != 7 {
		t.Errorf("search: got %v, %v", metas, err)
	}
	found, err := a.SearchByIDs(context.Background(), []int{7, 8})
	if err != nil || len(found) != 1 || found[7] == nil {
		t.Errorf("by ids: got %v, %v", found, err)
	}
}

func TestFailedLoginLogsOut(t *testing.T) {
	a := newTestAnilist(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer good" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"data":null,"errors":[{"message":"Invalid token","status":400}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"viewer":{"id":1,"name":"david"}}}`))
	})
	if err := a.Login(context.Background(), "good"); err != nil || a.User() == nil {
		t.Fatalf("login: %v, user %v", err, a.User())
	}
	if err := a.Login(context.Background(), "bad"); err == nil || a.Authenticated() || a.User() != nil {
		t.Errorf("got %v authenticated=%t user %v, want logged out", err, a.Authenticated(), a.User())
	}
}

func TestUserAgent(t *testing.T) {
	var ua string
	a := newTestAnilist(t, func(w http.ResponseWriter, r *http.Request) {
		ua = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte(`{"data":{"page":{"media":[]}}}`))
	})
	if _, err := a.Search(context.Background(), "berserk"); err != nil {
		t.Fatal(err)
	}
	if ua != DefaultUserAgent {
		t.Errorf("user agent %q, want %q", ua, DefaultUserAgent)
	}
}

func TestHugeErrorBody(t *testing.T) {
	a := newTestAnilist(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"errors":[{"message":"` + strings.Repeat("a", 1<<20) + `"}]}`))
	})
	_, err := a.Search(context.Background(), "berserk")
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) || len(httpErr.Message) > maxErrorBody {
		t.Errorf("got a %d bytes error", len(err.Error()))
	}
}

func TestMangaStaffRoles(t *testing.T) {
	var m Manga
	if err := json.Unmarshal([]byte(`{"staff": {"edges": [
		{"role": "Story & Art", "node": {"name": {"full": "Kentarou Miura"}}},
		{"role": "Story", "node": {"name": {"full": "Kouji Mori"}}},
		{"role": "Translator (English)", "node": {"name": {"full": "Jason DeAngelis"}}},
		{"role": "Lettering (English)", "node": {"name": {"full": "Dan Nakrosis"}}}
	]}}`), &m); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		got, want []string
	}{
		{m.Authors(), []string{"Kentarou Miura", "Kouji Mori"}},
		{m.Artists(), []string{"Kentarou Miura"}},
		{m.Translators(), []string{"Jason DeAngelis"}},
		{m.Letterers(), []string{"Dan Nakrosis"}},
	} {
		if !slices.Equal(tt.got, tt.want) {
			t.Errorf("got %q, want %q", tt.got, tt.want)
		}
	}
}

func TestSearchByIDsChunks(t *testing.T) {
	var requests atomic.Int32
	a := newTestAnilist(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		var body apiRequestBody
		_ = json.NewDecoder(r.Body).Decode(&body)
		// every id is known except 13
		var media []any
		for _, id := range body.Variables["ids"].([]any) {
			if id.(float64) != 13 {
				media = append(media, map[string]any{"id": id, "title": map[string]any{"english": "x"}})
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"page": map[string]any{"media": media}}})
	})

	ids := make([]int, idsPerRequest+5)
	for i := range ids {
		ids[i] = i + 1
	}
	var reports []progress.Status
	ctx := progress.WithReporter(context.Background(), func(s progress.Status) { reports = append(reports, s) })
	found, err := a.SearchByIDs(ctx, ids)
	if err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 2 || len(reports) != 2 || reports[1].Done != 2 || reports[1].Total != 2 {
		t.Errorf("%d requests, reports %v; want 2 requests reported", requests.Load(), reports)
	}
	if len(found) != len(ids)-1 || found[13] != nil || found[len(ids)].ID().Value() != len(ids) {
		t.Errorf("found %d: %v", len(found), found)
	}
}

func TestLoginUser(t *testing.T) {
	a := newTestAnilist(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"viewer":{"id":1,"name":"david","about":"hi","avatar":{"medium":"avatar.png"},"siteUrl":"https://anilist.co/user/david"}}}`))
	})
	if err := a.Logout(); !errors.Is(err, metadata.ErrUnauthorized) {
		t.Errorf("logout before login: %v", err)
	}
	if err := a.Login(context.Background(), "token"); err != nil || !a.Authenticated() {
		t.Fatalf("login: %v", err)
	}
	u := a.User()
	if u.String() != "david (1)" || u.About() != "hi" || u.Avatar() != "avatar.png" || u.URL() != "https://anilist.co/user/david" || u.Source() != metadata.IDSourceAnilist {
		t.Errorf("user %q %q %q %q %q", u, u.About(), u.Avatar(), u.URL(), u.Source())
	}
	if err := a.Logout(); err != nil || a.Authenticated() || a.User() != nil {
		t.Errorf("logout: %v, authenticated=%t", err, a.Authenticated())
	}
}

func TestMangaFields(t *testing.T) {
	var m Manga
	if err := json.Unmarshal([]byte(`{
		"id": 30002, "idMal": 2,
		"title": {"romaji": "Berserk", "native": "ベルセルク"},
		"synonyms": ["Berserk", " Kenpuu Denki "],
		"averageScore": 94, "description": "Guts", "status": "HIATUS", "chapters": 380,
		"coverImage": {"large": "large.jpg", "medium": "medium.jpg"}, "bannerImage": "banner.jpg",
		"tags": [{"name": "Gore", "rank": 90}, {"name": "Cats", "rank": 20}],
		"staff": {"edges": [{"role": "Story & Art", "node": {"name": {"full": "Kentarou Miura"}}}]},
		"genres": ["Action"], "characters": {"nodes": [{"name": {"full": "Guts"}}]},
		"startDate": {"year": 1989, "month": 8, "day": 25},
		"countryOfOrigin": "JP", "isAdult": true, "siteUrl": "https://anilist.co/manga/30002"
	}`), &m); err != nil {
		t.Fatal(err)
	}
	if err := metadata.Validate(&m); err != nil {
		t.Fatal(err)
	}
	if m.String() != "Berserk (1989) [alid-30002]" || m.Title() != "Berserk" || !slices.Equal(m.AlternateTitles(), []string{"ベルセルク", "Kenpuu Denki"}) {
		t.Errorf("titles %q %q", m.String(), m.AlternateTitles())
	}
	if m.Score() != 4.7 || m.Description() != "Guts" || m.Status() != metadata.StatusHiatus || m.Chapters() != 380 {
		t.Errorf("score %v, description %q, status %q, chapters %d", m.Score(), m.Description(), m.Status(), m.Chapters())
	}
	if m.Cover() != "large.jpg" || m.Banner() != "banner.jpg" || !slices.Equal(m.Tags(), []string{"Gore"}) || !slices.Equal(m.Genres(), []string{"Action"}) || !slices.Equal(m.Characters(), []string{"Guts"}) {
		t.Errorf("cover %q, banner %q, tags %q, genres %q, characters %q", m.Cover(), m.Banner(), m.Tags(), m.Genres(), m.Characters())
	}
	if m.Country() != "JP" || m.AgeRating() != metadata.AgeRatingAdultsOnly || m.URL() != "https://anilist.co/manga/30002" || m.StartDate().Year != 1989 || m.EndDate() != (metadata.Date{}) {
		t.Errorf("country %q, age %q, url %q, dates %v %v", m.Country(), m.AgeRating(), m.URL(), m.StartDate(), m.EndDate())
	}
	if ids := m.ExtraIDs(); len(ids) != 1 || ids[0].Code != metadata.IDCodeMyAnimeList || ids[0].Value() != 2 {
		t.Errorf("extra ids %v", ids)
	}
}
