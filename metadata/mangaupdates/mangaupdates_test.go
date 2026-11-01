package mangaupdates

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"slices"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/luevano/libmangal/metadata"
	"golang.org/x/time/rate"
)

// testdata: trimmed answers of the live API (2026-10-28), plus a novel and a hit on another
// title in the search.
func newTestMangaUpdates(t *testing.T, handler http.HandlerFunc) *MangaUpdates {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	p, err := NewMangaUpdates(DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	p.apiURL = server.URL + "/v1"
	p.limiter = rate.NewLimiter(rate.Inf, 1)
	return p
}

func fixtureHandler(t *testing.T, requests *atomic.Int32) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Header.Get("User-Agent") != DefaultUserAgent {
			t.Errorf("user agent %q", r.Header.Get("User-Agent"))
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/series/search":
			var body struct {
				Search      string   `json:"search"`
				FilterTypes []string `json:"filter_types"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Search != "one piece" || !slices.Contains(body.FilterTypes, "Doujinshi") || r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("search body %+v (%v), content type %q", body, err, r.Header.Get("Content-Type"))
			}
			http.ServeFile(w, r, "testdata/search_one_piece.json")
		case r.Method == http.MethodGet && r.URL.Path == "/v1/series/55099564912":
			http.ServeFile(w, r, "testdata/series_55099564912.json")
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func TestSearchByID(t *testing.T) {
	if strconv.IntSize < 64 {
		t.Skip("series ids don't fit a 32-bit int")
	}
	var requests atomic.Int32
	p := newTestMangaUpdates(t, fixtureHandler(t, &requests))

	onePiece := int64(55099564912)
	meta, found, err := p.SearchByID(context.Background(), int(onePiece))
	if err != nil || !found {
		t.Fatalf("found %v, err %v", found, err)
	}
	if err := metadata.Validate(meta); err != nil {
		t.Fatal(err)
	}
	m := meta.(*Manga)
	for name, c := range map[string][2]any{
		"String":    {m.String(), "One Piece (1997) [muid-55099564912]"},
		"Authors":   {m.Authors(), []string{"ODA Eiichiro"}},
		"Artists":   {m.Artists(), []string{"ODA Eiichiro"}},
		"Publisher": {m.Publisher(), "Shueisha"},
		"Status":    {m.Status(), metadata.StatusReleasing},
		"Country":   {m.Country(), "JP"},
		"Score":     {m.Score(), float32(8.89) / 2},
		"Cover":     {m.Cover(), "https://cdn.mangaupdates.com/image/i531056.jpg"},
		"Genres":    {len(m.Genres()), 6},
		"Tags":      {m.Tags(), []string{"Adapted to Anime", "Absent Parent/s", "Abusive Authority Figure/s", "Abuse of Power"}},
		"ID":        {m.ID(), metadata.ID{Raw: "55099564912", Source: metadata.IDSourceMangaUpdates, Code: metadata.IDCodeMangaUpdates}},
		"Alt":       {m.AlternateTitles()[0], "Đảo Hải Tặc"},
		"Chapters":  {m.Chapters(), 1196},
	} {
		if !reflect.DeepEqual(c[0], c[1]) {
			t.Errorf("%s = %v, want %v", name, c[0], c[1])
		}
	}

	if _, found, err := p.SearchByID(context.Background(), 404); found || err != nil {
		t.Errorf("unknown id: found %v, err %v", found, err)
	}
}

func TestSearch(t *testing.T) {
	if strconv.IntSize < 64 {
		t.Skip("series ids don't fit a 32-bit int")
	}
	var requests atomic.Int32
	p := newTestMangaUpdates(t, fixtureHandler(t, &requests))
	metas, err := p.Search(context.Background(), "one piece")
	if err != nil {
		t.Fatal(err)
	}
	if got := requests.Load(); got != 1 {
		t.Errorf("%d requests, want 1", got)
	}
	// the doujinshi and novel are left out
	if len(metas) != 4 {
		t.Fatalf("got %v", metas)
	}
	m := metas[0].(*Manga)
	if !metadata.IsPartial(m) || m.Status() != "" || len(m.Authors()) != 0 {
		t.Errorf("not a partial result: %+v", m)
	}
	if err := metadata.Validate(m); err == nil {
		t.Error("a search result is valid")
	}
	for name, c := range map[string][2]any{
		"String": {m.String(), "One Piece (1997) [muid-55099564912]"},
		"Score":  {m.Score(), float32(8.89) / 2},
		"Cover":  {m.Cover(), "https://cdn.mangaupdates.com/image/i531056.jpg"},
		"Genres": {len(m.Genres()), 6},
		"Hit":    {metas[1].AlternateTitles(), []string{"Sasakure One Piece"}},
	} {
		if !reflect.DeepEqual(c[0], c[1]) {
			t.Errorf("%s = %v, want %v", name, c[0], c[1])
		}
	}
}

// TestSearchCached: searching costs one request however often, matching one more for the
// chosen series only, and a whole series isn't replaced by its search result.
func TestSearchCached(t *testing.T) {
	if strconv.IntSize < 64 {
		t.Skip("series ids don't fit a 32-bit int")
	}
	var requests atomic.Int32
	options := metadata.DefaultProviderWithCacheOptions()
	options.Provider = newTestMangaUpdates(t, fixtureHandler(t, &requests))
	p, err := metadata.NewProviderWithCache(options)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	ctx := context.Background()

	for range 2 {
		metas, err := p.SearchContext(ctx, "one piece")
		if err != nil || len(metas) != 4 {
			t.Fatalf("got %v, err %v", metas, err)
		}
	}
	if got := requests.Load(); got != 1 {
		t.Errorf("%d requests after two searches, want 1", got)
	}

	match, found, err := p.FindClosestContext(ctx, metadata.Query{Titles: []string{"one piece"}})
	if err != nil || !found {
		t.Fatalf("found %v, err %v", found, err)
	}
	if err := metadata.Validate(match.Metadata); err != nil {
		t.Errorf("the chosen series: %v", err)
	}
	if got := requests.Load(); got != 2 {
		t.Errorf("%d requests after matching, want 2", got)
	}

	metas, err := p.SearchContext(ctx, "one piece")
	if err != nil {
		t.Fatal(err)
	}
	if metadata.IsPartial(metas[0]) || !metadata.IsPartial(metas[1]) {
		t.Errorf("after matching: partial %v, %v", metadata.IsPartial(metas[0]), metadata.IsPartial(metas[1]))
	}
	if got := requests.Load(); got != 2 {
		t.Errorf("%d requests after searching again, want 2", got)
	}
}

func TestErrors(t *testing.T) {
	var calls atomic.Int32
	p := newTestMangaUpdates(t, func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"status":"exception","reason":"database down"}`))
	})
	_, err := p.Search(context.Background(), "x")
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) || httpErr.StatusCode != 500 || httpErr.Message != "database down" || calls.Load() != 2 {
		t.Errorf("err %v after %d calls", err, calls.Load())
	}

	p = newTestMangaUpdates(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
	})
	if _, _, err := p.SearchByID(context.Background(), 1); !errors.Is(err, metadata.ErrRateLimited) {
		t.Errorf("err %v, want rate limited", err)
	}
	if err := p.SetMangaProgress(context.Background(), 1, 1); !errors.Is(err, errors.ErrUnsupported) {
		t.Errorf("progress: %v", err)
	}
}

func TestRequestsAreSpaced(t *testing.T) {
	p, err := NewMangaUpdates(DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if p.limiter.Limit() != rate.Every(requestInterval) || p.limiter.Burst() != 1 {
		t.Errorf("limit %v burst %d", p.limiter.Limit(), p.limiter.Burst())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := p.SearchByID(ctx, 1); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled: %v", err)
	}
}

func TestStatus(t *testing.T) {
	for text, want := range map[string]metadata.Status{
		"115 Volumes (Ongoing)":    metadata.StatusReleasing,
		"10 Volumes (Complete)":    metadata.StatusFinished,
		"3 Volumes (Hiatus)":       metadata.StatusHiatus,
		"2 Volumes (Discontinued)": metadata.StatusCancelled,
		"":                         metadata.StatusReleasing,
	} {
		if got := (&Manga{StatusText: text}).Status(); got != want {
			t.Errorf("%q: %s, want %s", text, got, want)
		}
	}
	if got := (&Manga{Completed: true}).Status(); got != metadata.StatusFinished {
		t.Errorf("completed: %s", got)
	}
}

func TestGobRegistered(t *testing.T) {
	gob.RegisterName("mangaupdates-manga", &Manga{})
	data, err := os.ReadFile("testdata/series_55099564912.json")
	if err != nil {
		t.Fatal(err)
	}
	m := &Manga{}
	if err := json.Unmarshal(data, m); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	var in metadata.Metadata = m
	if err := gob.NewEncoder(&buf).Encode(&in); err != nil {
		t.Fatal(err)
	}
	var out metadata.Metadata
	if err := gob.NewDecoder(&buf).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(out, in) {
		t.Errorf("decoded %+v", out)
	}
}
