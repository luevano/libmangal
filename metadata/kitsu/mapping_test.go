package kitsu

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/luevano/libmangal/metadata"
)

// mappingsHandler answers the mappings of anilist/manga ids (as Kitsu does, checked
// 2026-10-12), pages of limit; ids maps an AniList id to its Kitsu mangas.
func mappingsHandler(t *testing.T, ids map[string][]string, limit int, requests *atomic.Int32) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		q := r.URL.Query()
		if r.URL.Path != "/api/edge/mappings" || q.Get("filter[externalSite]") != "anilist/manga" || q.Get("include") != "item" {
			t.Errorf("unexpected request %s", r.URL)
		}
		var data, included []string
		for ext := range strings.SplitSeq(q.Get("filter[externalId]"), ",") {
			for _, kt := range ids[ext] {
				data = append(data, fmt.Sprintf(`{"id": "m%s-%s", "type": "mappings",
					"attributes": {"externalSite": "anilist/manga", "externalId": %q},
					"relationships": {"item": {"links": {}, "data": {"type": "manga", "id": %q}}}}`, ext, kt, ext, kt))
				included = append(included, fmt.Sprintf(`{"id": %q, "type": "manga", "attributes": {"subtype": "manga"}}`, kt))
			}
		}
		count := len(data)
		offset, _ := strconv.Atoi(q.Get("page[offset]"))
		data = data[min(offset, count):min(offset+limit, count)]
		fmt.Fprintf(w, `{"data": [%s], "included": [%s], "meta": {"count": %d}}`, strings.Join(data, ","), strings.Join(included, ","), count)
	}
}

func TestIDsFrom(t *testing.T) {
	var requests atomic.Int32
	ids := map[string][]string{"30002": {"8"}, "31133": {"2435"}, "74347": {"24147", "900"}}
	for i := 1; i <= 25; i++ {
		ids[strconv.Itoa(1000+i)] = []string{strconv.Itoa(i)}
	}
	k := newTestKitsu(t, mappingsHandler(t, ids, 2, &requests))

	got, err := k.IDsFrom(context.Background(), metadata.IDCodeAnilist, []int{30002, 31133, 74347, 999})
	if err != nil {
		t.Fatal(err)
	}
	// 4 mappings, 2 a page; 74347 has two, the lowest wins
	if want := map[int]int{30002: 8, 31133: 2435, 74347: 900}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if requests.Load() != 2 {
		t.Errorf("%d requests, want 2 (pages of 2)", requests.Load())
	}

	requests.Store(0)
	k = newTestKitsu(t, mappingsHandler(t, ids, pageLimit, &requests))
	many := make([]int, 25)
	for i := range many {
		many[i] = 1001 + i
	}
	if got, err := k.IDsFrom(context.Background(), metadata.IDCodeAnilist, many); err != nil || len(got) != 25 || got[1025] != 25 {
		t.Errorf("got %v, %v", got, err)
	}
	if requests.Load() != 2 {
		t.Errorf("%d requests, want 2 (20 ids per request)", requests.Load())
	}
}

func TestIDsFromIgnores(t *testing.T) {
	k := newTestKitsu(t, func(w http.ResponseWriter, r *http.Request) {
		// another id, another site and an anime aren't taken
		_, _ = w.Write([]byte(`{"data": [
			{"id": "1", "type": "mappings", "attributes": {"externalSite": "anilist/manga", "externalId": "5"}, "relationships": {"item": {"data": {"type": "manga", "id": "50"}}}},
			{"id": "2", "type": "mappings", "attributes": {"externalSite": "anilist/anime", "externalId": "1"}, "relationships": {"item": {"data": {"type": "anime", "id": "60"}}}},
			{"id": "3", "type": "mappings", "attributes": {"externalSite": "anilist/manga", "externalId": "1"}, "relationships": {"item": {"data": {"type": "anime", "id": "70"}}}},
			{"id": "4", "type": "mappings", "attributes": {"externalSite": "anilist/manga", "externalId": "1"}, "relationships": {"item": {"data": {"type": "manga", "id": "80"}}}}
		], "meta": {"count": 4}}`))
	})
	got, err := k.IDsFrom(context.Background(), metadata.IDCodeAnilist, []int{1})
	if err != nil || !reflect.DeepEqual(got, map[int]int{1: 80}) {
		t.Errorf("got %v, %v", got, err)
	}
	if _, err := k.IDsFrom(context.Background(), metadata.IDCodeMangaUpdates, []int{1}); !errors.Is(err, errors.ErrUnsupported) {
		t.Errorf("mangaupdates: %v, want ErrUnsupported", err)
	}
}

func TestIDsFromRateLimit(t *testing.T) {
	var requests atomic.Int32
	inner := mappingsHandler(t, map[string][]string{"30002": {"8"}}, pageLimit, &requests)
	var calls atomic.Int32
	k := newTestKitsu(t, func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		inner(w, r)
	})
	got, err := k.IDsFrom(context.Background(), metadata.IDCodeAnilist, []int{30002})
	if err != nil || got[30002] != 8 || calls.Load() != 2 {
		t.Errorf("got %v, %v after %d calls", got, err, calls.Load())
	}

	k = newTestKitsu(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
	})
	_, err = k.IDsFrom(context.Background(), metadata.IDCodeAnilist, []int{30002})
	if !errors.Is(err, metadata.ErrRateLimited) || !strings.HasPrefix(err.Error(), "map al ids [30002]: kitsu: GET /api/edge/mappings: 429") {
		t.Errorf("got %v, want a rate limit", err)
	}
}
