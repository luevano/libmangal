package anilist

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/luevano/libmangal/metadata"
)

func TestIDsFromMyAnimeList(t *testing.T) {
	var requests atomic.Int32
	a := newTestAnilist(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		var body apiRequestBody
		_ = json.NewDecoder(r.Body).Decode(&body)
		page := int(body.Variables["page"].(float64))
		var media []any
		for _, v := range body.Variables["ids"].([]any) {
			mal := int(v.(float64))
			switch {
			case mal == 999:
				// unknown
			case mal == 2 && page == 1:
				media = append(media, map[string]any{"id": 300, "idMal": 2})
			case mal == 2 && page == 2:
				// a second manga with the same idMal, on the next page: the lowest wins
				media = append(media, map[string]any{"id": 30, "idMal": 2})
			case page == 1:
				media = append(media, map[string]any{"id": mal + 1000, "idMal": mal})
			}
		}
		// a manga with an idMal not asked for is ignored, so is a null one
		if page == 1 {
			media = append(media, map[string]any{"id": 5, "idMal": 12345}, nil)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"page": map[string]any{
			"pageInfo": map[string]any{"hasNextPage": page == 1}, "media": media}}})
	})
	got, err := a.IDsFrom(context.Background(), metadata.IDCodeMyAnimeList, []int{1, 2, 999})
	if err != nil {
		t.Fatal(err)
	}
	if want := map[int]int{1: 1001, 2: 30}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if requests.Load() != 2 {
		t.Errorf("%d requests, want 2 (two pages)", requests.Load())
	}

	requests.Store(0)
	many := make([]int, malIDsPerRequest+1)
	for i := range many {
		many[i] = i + 10
	}
	got, err = a.IDsFrom(context.Background(), metadata.IDCodeMyAnimeList, many)
	if err != nil || len(got) != len(many) || got[10] != 1010 {
		t.Errorf("got %d ids (%v), %v", len(got), got[10], err)
	}
	if requests.Load() != 4 {
		t.Errorf("%d requests, want 4 (two chunks of two pages)", requests.Load())
	}

	if _, err := a.IDsFrom(context.Background(), metadata.IDCodeKitsu, []int{1}); !errors.Is(err, errors.ErrUnsupported) {
		t.Errorf("kitsu: %v, want ErrUnsupported", err)
	}
}
