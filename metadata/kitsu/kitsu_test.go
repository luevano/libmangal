package kitsu

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/luevano/libmangal/metadata"
)

func newTestKitsu(t *testing.T, handler http.HandlerFunc) *Kitsu {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	k, err := NewKitsu(DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	k.apiURL = server.URL + "/api/edge"
	k.tokenURL = server.URL + "/api/oauth/token"
	return k
}

// soloLeveling is a trimmed manga resource as Kitsu answers it (checked 2026-10-11),
// with the relationships of mangaParams.
const soloLeveling = `{
	"id": "54114", "type": "manga",
	"attributes": {
		"slug": "solo-levelling",
		"synopsis": "In a world where awakened beings called Hunters...",
		"titles": {"en_jp": "Boku dake Level Up na Ken", "en_kr": "Na Honjaman Level Up", "en_us": "Solo Leveling", "ko_kr": "나 혼자만 레벨업"},
		"canonicalTitle": "Solo Leveling",
		"abbreviatedTitles": ["Only I level up", "Na Honjaman Level Up"],
		"averageRating": "84.5",
		"startDate": "2018-03-04", "endDate": "2023-05-31",
		"ageRating": "PG", "subtype": "manhwa", "status": "finished",
		"posterImage": {"large": "https://media.kitsu.app/manga/poster_images/54114/large.jpg", "original": "https://media.kitsu.app/manga/poster_images/54114/original.jpg"},
		"coverImage": null,
		"chapterCount": 201, "volumeCount": null, "serialization": "KakaoPage"
	},
	"relationships": {
		"categories": {"data": [{"type": "categories", "id": "150"}]},
		"staff": {"data": [{"type": "mediaStaff", "id": "1"}, {"type": "mediaStaff", "id": "2"}]},
		"characters": {"data": [{"type": "mediaCharacters", "id": "10"}, {"type": "mediaCharacters", "id": "11"}]},
		"mappings": {"data": [{"type": "mappings", "id": "100"}, {"type": "mappings", "id": "101"}, {"type": "mappings", "id": "102"}]}
	}
}`

const soloLevelingIncluded = `[
	{"id": "150", "type": "categories", "attributes": {"title": "Action"}},
	{"id": "1", "type": "mediaStaff", "attributes": {"role": "Story"}, "relationships": {"person": {"data": {"type": "people", "id": "7"}}}},
	{"id": "2", "type": "mediaStaff", "attributes": {"role": "Art"}, "relationships": {"person": {"data": {"type": "people", "id": "8"}}}},
	{"id": "7", "type": "people", "attributes": {"name": "Chugong"}},
	{"id": "8", "type": "people", "attributes": {"name": "DUBU"}},
	{"id": "10", "type": "mediaCharacters", "attributes": {"role": "main"}, "relationships": {"character": {"data": {"type": "characters", "id": "20"}}}},
	{"id": "11", "type": "mediaCharacters", "attributes": {"role": "supporting"}, "relationships": {"character": {"data": {"type": "characters", "id": "21"}}}},
	{"id": "20", "type": "characters", "attributes": {"canonicalName": "Jinwoo Sung"}},
	{"id": "21", "type": "characters", "attributes": {"canonicalName": "Jinah Sung"}},
	{"id": "100", "type": "mappings", "attributes": {"externalSite": "anilist/manga", "externalId": "105398"}},
	{"id": "101", "type": "mappings", "attributes": {"externalSite": "mangaupdates", "externalId": "6z1uqw7"}},
	{"id": "102", "type": "mappings", "attributes": {"externalSite": "myanimelist/manga", "externalId": "121496"}}
]`

const novel = `{"id": "56076", "type": "manga", "attributes": {"canonicalTitle": "Only I Level Up", "subtype": "novel", "status": "finished"}}`

func TestSearch(t *testing.T) {
	var query, userAgent string
	k := newTestKitsu(t, func(w http.ResponseWriter, r *http.Request) {
		query, userAgent = r.URL.Query().Get("filter[text]"), r.UserAgent()
		if r.URL.Path != "/api/edge/manga" || r.URL.Query().Get("include") == "" {
			t.Errorf("got %s, want the manga list with includes", r.URL)
		}
		_, _ = w.Write([]byte(`{"data": [` + soloLeveling + `, ` + novel + `], "included": ` + soloLevelingIncluded + `}`))
	})

	metas, err := k.Search(context.Background(), "solo leveling")
	if err != nil {
		t.Fatal(err)
	}
	if query != "solo leveling" || userAgent != DefaultUserAgent {
		t.Errorf("query %q user agent %q", query, userAgent)
	}
	if len(metas) != 1 {
		t.Fatalf("got %v, want the novel left out", metas)
	}
	m := metas[0]
	if err := metadata.Validate(m); err != nil {
		t.Error(err)
	}
	checks := []struct {
		name      string
		got, want any
	}{
		{"string", m.String(), "Solo Leveling (2018) [ktid-54114]"},
		{"alternate titles", m.AlternateTitles(), []string{"Boku dake Level Up na Ken", "Na Honjaman Level Up", "나 혼자만 레벨업", "Only I level up"}},
		{"score", m.Score(), float32(84.5 / 20)},
		{"cover", m.Cover(), "https://media.kitsu.app/manga/poster_images/54114/original.jpg"},
		{"banner", m.Banner(), ""},
		{"genres", m.Genres(), []string{"Action"}},
		{"authors", m.Authors(), []string{"Chugong"}},
		{"artists", m.Artists(), []string{"DUBU"}},
		{"characters", m.Characters(), []string{"Jinwoo Sung"}},
		{"start", m.StartDate(), metadata.Date{Year: 2018, Month: 3, Day: 4}},
		{"end", m.EndDate(), metadata.Date{Year: 2023, Month: 5, Day: 31}},
		{"status", m.Status(), metadata.StatusFinished},
		{"country", m.Country(), "KR"},
		{"chapters", m.Chapters(), 201},
		{"url", m.URL(), "https://kitsu.app/manga/solo-levelling"},
		{"extra ids", m.ExtraIDs(), []metadata.ID{
			{Raw: "105398", Source: metadata.IDSourceAnilist, Code: metadata.IDCodeAnilist},
			{Raw: "121496", Source: metadata.IDSourceMyAnimeList, Code: metadata.IDCodeMyAnimeList},
		}},
	}
	for _, c := range checks {
		if !reflect.DeepEqual(c.got, c.want) {
			t.Errorf("%s: got %#v, want %#v", c.name, c.got, c.want)
		}
	}
}

func TestSearchByID(t *testing.T) {
	k := newTestKitsu(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/edge/manga/54114" {
			t.Errorf("got %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data": ` + soloLeveling + `, "included": ` + soloLevelingIncluded + `}`))
	})

	meta, found, err := k.SearchByID(context.Background(), 54114)
	if err != nil || !found {
		t.Fatalf("found=%t err=%v", found, err)
	}
	if meta.Title() != "Solo Leveling" || meta.ID().Value() != 54114 {
		t.Errorf("got %v", meta)
	}
}

func TestSearchByIDNotFound(t *testing.T) {
	k := newTestKitsu(t, func(w http.ResponseWriter, r *http.Request) {
		// what Kitsu answers for an unknown id (checked 2026-10-11)
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errors":[{"title":"Record not found","detail":"The record identified by 999999999 could not be found.","code":"404","status":"404"}]}`))
	})

	meta, found, err := k.SearchByID(context.Background(), 999999999)
	if err != nil || found || meta != nil {
		t.Errorf("got %v found=%t err=%v, want not found", meta, found, err)
	}
}

func TestHTTPError(t *testing.T) {
	k := newTestKitsu(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"errors":[{"title":"Invalid page value","detail":"Limit exceeds maximum page size of 20.","code":"118","status":"400"}]}`))
	})

	_, err := k.Search(context.Background(), "berserk")
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusBadRequest || httpErr.Code != "118" {
		t.Fatalf("got %v, want a 400 HTTPError", err)
	}
	if want := `search "berserk": kitsu: GET /api/edge/manga: 400 bad request: Limit exceeds maximum page size of 20.`; err.Error() != want {
		t.Errorf("got %q, want %q", err, want)
	}
	if errors.Is(err, metadata.ErrUnauthorized) || errors.Is(err, metadata.ErrNotFound) {
		t.Error("a bad request isn't unauthorized or not found")
	}
}

func TestRateLimitRetry(t *testing.T) {
	var requests atomic.Int32
	k := newTestKitsu(t, func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(`{"data": []}`))
	})

	start := time.Now()
	if _, err := k.Search(context.Background(), "berserk"); err != nil {
		t.Fatal(err)
	}
	if waited := time.Since(start); waited < time.Second {
		t.Errorf("retried after %s, want the 1s from Retry-After", waited)
	}
	if requests.Load() != 2 {
		t.Errorf("%d requests, want 2", requests.Load())
	}
}

func TestRateLimitError(t *testing.T) {
	k := newTestKitsu(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
	})

	_, err := k.Search(context.Background(), "berserk")
	var rateLimit *RateLimitError
	if !errors.As(err, &rateLimit) || !errors.Is(err, metadata.ErrRateLimited) {
		t.Fatalf("got %v, want a RateLimitError", err)
	}
	if want := `search "berserk": kitsu: GET /api/edge/manga: 429 too many requests (retry after 1s)`; err.Error() != want {
		t.Errorf("got %q, want %q", err, want)
	}
	if got := retryAfter(http.Header{"Retry-After": {"0"}}); got != time.Minute {
		t.Errorf("Retry-After 0: %s, want a minute", got)
	}
}

func TestRateLimitRetryCancel(t *testing.T) {
	k := newTestKitsu(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	if _, err := k.Search(ctx, "berserk"); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("got %v, want the context error", err)
	}
}

func TestSearchByIDs(t *testing.T) {
	var requests int
	k := newTestKitsu(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		ids := strings.Split(r.URL.Query().Get("filter[id]"), ",")
		var data []string
		for _, id := range ids {
			if id == "3" {
				continue // unknown
			}
			data = append(data, `{"id": "`+id+`", "type": "manga", "attributes": {"canonicalTitle": "Manga `+id+`"}}`)
		}
		_, _ = w.Write([]byte(`{"data": [` + strings.Join(data, ",") + `]}`))
	})

	ids := make([]int, 25)
	for i := range ids {
		ids[i] = i + 1
	}
	found, err := k.SearchByIDs(context.Background(), ids)
	if err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Errorf("%d requests, want 2 (20 ids per request)", requests)
	}
	if len(found) != 24 || found[3] != nil || found[25].Title() != "Manga 25" {
		t.Errorf("got %d mangas: %v", len(found), found)
	}
}

const selfUser = `{"data": [{"id": "42", "type": "users", "attributes": {"name": "David", "slug": "david", "about": "hi", "avatar": {"original": "https://media.kitsu.app/users/avatars/42/original.jpg"}}}]}`

func TestLogin(t *testing.T) {
	var auth string
	k := newTestKitsu(t, func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		if r.URL.Path != "/api/edge/users" || r.URL.Query().Get("filter[self]") != "true" {
			t.Errorf("got %s, want the self user", r.URL)
		}
		_, _ = w.Write([]byte(selfUser))
	})

	if err := k.Login(context.Background(), "token"); err != nil {
		t.Fatal(err)
	}
	if auth != "Bearer token" || !k.Authenticated() {
		t.Errorf("authorization %q authenticated=%t", auth, k.Authenticated())
	}
	user := k.User()
	if user.String() != "David (42)" || user.URL() != "https://kitsu.app/users/david" || user.Source() != metadata.IDSourceKitsu {
		t.Errorf("got %v %s", user, user.URL())
	}
	if account := metadata.AccountOf(k); account.Status != "logged in" || account.User != "David" {
		t.Errorf("got %+v", account)
	}

	if err := k.Logout(); err != nil || k.Authenticated() || k.User() != nil {
		t.Errorf("logout: %v, authenticated=%t", err, k.Authenticated())
	}
}

func TestLoginInvalidToken(t *testing.T) {
	k := newTestKitsu(t, func(w http.ResponseWriter, r *http.Request) {
		// Kitsu ignores a bad token, there's just no self user (checked 2026-10-11)
		_, _ = w.Write([]byte(`{"data": [], "meta": {"count": 0}}`))
	})

	err := k.Login(context.Background(), "expired")
	if !errors.Is(err, metadata.ErrUnauthorized) || k.Authenticated() || k.User() != nil {
		t.Errorf("got %v authenticated=%t, want unauthorized", err, k.Authenticated())
	}
}

func TestSetMangaProgressUnauthorized(t *testing.T) {
	k := newTestKitsu(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("no request expected")
	})
	if err := k.SetMangaProgress(context.Background(), 7, 3); !errors.Is(err, metadata.ErrUnauthorized) {
		t.Errorf("got %v, want unauthorized", err)
	}
}

func TestSetMangaProgress(t *testing.T) {
	tests := []struct {
		name       string
		entries    string // library entries data
		chapter    int
		wantMethod string // of the write, empty if none
		wantBody   string
	}{
		{
			"not in library", `[]`, 3, http.MethodPost,
			`{"data":{"type":"libraryEntries","attributes":{"status":"current","progress":3},"relationships":{"media":{"data":{"id":"7","type":"manga"}},"user":{"data":{"id":"42","type":"users"}}}}}`,
		},
		{
			"higher", `[{"id": "900", "type": "libraryEntries", "attributes": {"status": "completed", "progress": 10}}]`, 12, http.MethodPatch,
			`{"data":{"id":"900","type":"libraryEntries","attributes":{"progress":12}}}`,
		},
		{"lower", `[{"id": "900", "type": "libraryEntries", "attributes": {"status": "completed", "progress": 10}}]`, 5, "", ""},
		{"same", `[{"id": "900", "type": "libraryEntries", "attributes": {"status": "current", "progress": 10}}]`, 10, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var writes []string
			k := newTestKitsu(t, func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodGet && r.URL.Path == "/api/edge/users":
					_, _ = w.Write([]byte(selfUser))
				case r.Method == http.MethodGet && r.URL.Path == "/api/edge/library-entries":
					q := r.URL.Query()
					if q.Get("filter[user_id]") != "42" || q.Get("filter[manga_id]") != "7" {
						t.Errorf("got %s", r.URL)
					}
					_, _ = w.Write([]byte(`{"data": ` + tt.entries + `}`))
				default:
					if r.Header.Get("Content-Type") != contentType {
						t.Errorf("content type %q", r.Header.Get("Content-Type"))
					}
					body, _ := io.ReadAll(r.Body)
					writes = append(writes, r.Method+" "+r.URL.Path+" "+string(body))
					w.WriteHeader(http.StatusCreated)
					_, _ = w.Write([]byte(`{"data": {"id": "900", "type": "libraryEntries"}}`))
				}
			})
			if err := k.Login(context.Background(), "token"); err != nil {
				t.Fatal(err)
			}

			if err := k.SetMangaProgress(context.Background(), 7, tt.chapter); err != nil {
				t.Fatal(err)
			}
			if tt.wantMethod == "" {
				if len(writes) != 0 {
					t.Errorf("got %v, want no write", writes)
				}
				return
			}
			if len(writes) != 1 {
				t.Fatalf("got %v, want one write", writes)
			}
			method, rest, _ := strings.Cut(writes[0], " ")
			_, body, _ := strings.Cut(rest, " ")
			if method != tt.wantMethod || !jsonEqual(t, body, tt.wantBody) {
				t.Errorf("got %s %s, want %s %s", method, body, tt.wantMethod, tt.wantBody)
			}
		})
	}
}

func jsonEqual(t *testing.T, a, b string) bool {
	t.Helper()
	var va, vb any
	if err := json.Unmarshal([]byte(a), &va); err != nil {
		t.Fatalf("%s: %v", a, err)
	}
	if err := json.Unmarshal([]byte(b), &vb); err != nil {
		t.Fatalf("%s: %v", b, err)
	}
	return reflect.DeepEqual(va, vb)
}

// mangal caches metadata with gob, as a metadata.Metadata
func TestMangaGob(t *testing.T) {
	var doc document
	if err := json.Unmarshal([]byte(`{"data": `+soloLeveling+`, "included": `+soloLevelingIncluded+`}`), &doc); err != nil {
		t.Fatal(err)
	}
	mangas, err := doc.mangas()
	if err != nil {
		t.Fatal(err)
	}
	gob.RegisterName("kitsu-manga-test", &Manga{})

	var buf bytes.Buffer
	in := metadata.Metadata(mangas[0])
	if err := gob.NewEncoder(&buf).Encode(&in); err != nil {
		t.Fatal(err)
	}
	var out metadata.Metadata
	if err := gob.NewDecoder(&buf).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Errorf("got %#v, want %#v", out, in)
	}
}

func TestLoginPassword(t *testing.T) {
	var form url.Values
	k := newTestKitsu(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/oauth/token":
			if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
				t.Errorf("got %s %q", r.Method, r.Header.Get("Content-Type"))
			}
			_ = r.ParseForm()
			form = r.PostForm
			_, _ = w.Write([]byte(`{"access_token":"access","token_type":"Bearer","expires_in":2592000,"refresh_token":"refresh","scope":"public","created_at":1791700000}`))
		case "/api/edge/users":
			if r.Header.Get("Authorization") != "Bearer access" {
				t.Errorf("authorization %q", r.Header.Get("Authorization"))
			}
			_, _ = w.Write([]byte(selfUser))
		default:
			t.Errorf("unexpected %s", r.URL)
		}
	})
	k.options.ClientID, k.options.ClientSecret = "id", "secret"

	token, err := k.LoginPassword(context.Background(), "david@example.com", "hunter2")
	if err != nil {
		t.Fatal(err)
	}
	want := url.Values{"grant_type": {"password"}, "username": {"david@example.com"}, "password": {"hunter2"}, "client_id": {"id"}, "client_secret": {"secret"}}
	if !reflect.DeepEqual(form, want) {
		t.Errorf("form %v, want %v", form, want)
	}
	if token.RefreshToken != "refresh" || !token.Expiry().Equal(time.Unix(1791700000+2592000, 0)) {
		t.Errorf("got %+v", token)
	}
	if !k.Authenticated() || k.User().Name() != "David" {
		t.Errorf("authenticated=%t user=%v", k.Authenticated(), k.User())
	}
}

func TestLoginPasswordWrong(t *testing.T) {
	k := newTestKitsu(t, func(w http.ResponseWriter, r *http.Request) {
		// what Kitsu answers for a wrong password (checked 2026-10-11)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"The provided authorization grant is invalid, expired, revoked, does not match the redirection URI used in the authorization request, or was issued to another client."}`))
	})

	_, err := k.LoginPassword(context.Background(), "david@example.com", "wrong")
	var httpErr *HTTPError
	if !errors.Is(err, metadata.ErrUnauthorized) || !errors.As(err, &httpErr) || httpErr.Code != "invalid_grant" || k.Authenticated() {
		t.Errorf("got %v authenticated=%t, want unauthorized", err, k.Authenticated())
	}
	if !strings.HasPrefix(err.Error(), "login: get token: kitsu: POST /api/oauth/token: 400 bad request: invalid_grant: ") {
		t.Errorf("got %q", err)
	}
}

func TestRefreshToken(t *testing.T) {
	var form url.Values
	k := newTestKitsu(t, func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		form = r.PostForm
		_, _ = w.Write([]byte(`{"access_token":"new","refresh_token":"refresh2"}`))
	})

	token, err := k.RefreshToken(context.Background(), "refresh")
	if err != nil {
		t.Fatal(err)
	}
	if form.Get("grant_type") != "refresh_token" || form.Get("refresh_token") != "refresh" || form.Has("client_id") {
		t.Errorf("form %v", form)
	}
	if token.AccessToken != "new" || !token.Expiry().IsZero() {
		t.Errorf("got %+v", token)
	}
}

func TestFailedLoginLogsOut(t *testing.T) {
	k := newTestKitsu(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer good" {
			_, _ = w.Write([]byte(`{"data": [], "meta": {"count": 0}}`))
			return
		}
		_, _ = w.Write([]byte(selfUser))
	})
	if err := k.Login(context.Background(), "good"); err != nil || k.User() == nil {
		t.Fatalf("login: %v, user %v", err, k.User())
	}
	if err := k.Login(context.Background(), "bad"); err == nil || k.Authenticated() || k.User() != nil {
		t.Errorf("got %v authenticated=%t user %v, want logged out", err, k.Authenticated(), k.User())
	}
}

func TestOAuthErrorWithoutDescription(t *testing.T) {
	k := newTestKitsu(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
	})
	_, err := k.PasswordToken(context.Background(), "david", "wrong")
	if err == nil || strings.HasSuffix(err.Error(), ": ") || !strings.HasSuffix(err.Error(), "invalid_grant") {
		t.Errorf("got %v", err)
	}
}
