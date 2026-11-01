package metadata

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/philippgille/gokv"
)

// fakeBatcher is a fakeProvider that also batches, counting the batched calls.
type fakeBatcher struct {
	*fakeProvider
	manyCalls, idsCalls int
}

func (p *fakeBatcher) SearchMany(ctx context.Context, queries []string) ([][]Metadata, error) {
	p.manyCalls++
	results := make([][]Metadata, len(queries))
	for i, q := range queries {
		results[i] = p.byQuery[q]
	}
	return results, nil
}

func (p *fakeBatcher) SearchByIDs(ctx context.Context, ids []int) (map[int]Metadata, error) {
	p.idsCalls++
	found := map[int]Metadata{}
	for _, id := range ids {
		if m, ok := p.byID[id]; ok {
			found[id] = m
		}
	}
	return found, nil
}

func TestFindClosestManyBatches(t *testing.T) {
	berserk := &fakeMeta{id: 1, title: "Berserk"}
	vagabond := &fakeMeta{id: 2, title: "Vagabond"}
	kaiju := &titledMeta{id: 3, title: "Kaiju No. 8"}
	bound := &fakeMeta{id: 4, title: "Bound One"}
	fake := &fakeBatcher{fakeProvider: &fakeProvider{
		byID: map[int]Metadata{4: bound},
		byQuery: map[string][]Metadata{
			"Berserk":            {berserk},
			"Vagabond":           {vagabond},
			"Kaiju Number Eight": {},
			"Kaiju No. 8":        {kaiju},
		},
	}}
	p := newTestProvider(t, fake.fakeProvider)
	p.provider = fake
	if err := p.BindTitleWithID("bound title", 4); err != nil {
		t.Fatal(err)
	}

	queries := []Query{
		{Titles: []string{"Berserk"}},
		{Titles: []string{"Vagabond"}},
		{Titles: []string{"Kaiju Number Eight", "Kaiju No. 8"}},
		{Titles: []string{"bound title"}},
		{Titles: []string{"Nothing"}},
	}
	matches, found, err := p.FindClosestManyContext(context.Background(), queries)
	if err != nil {
		t.Fatal(err)
	}
	want := []Metadata{berserk, vagabond, kaiju, bound, nil}
	for i := range queries {
		if found[i] != (want[i] != nil) || (found[i] && matches[i].Metadata != want[i]) {
			t.Errorf("query %d: got %v found=%t, want %v", i, matches[i].Metadata, found[i], want[i])
		}
	}
	if !matches[3].Bound {
		t.Error("binding not marked as bound")
	}
	// round 1 (4 first titles) and round 2 (kaiju's second title)
	if fake.manyCalls != 2 || fake.idsCalls != 1 {
		t.Errorf("%d batched searches and %d id lookups, want 2 and 1", fake.manyCalls, fake.idsCalls)
	}
	if len(fake.queries) != 0 {
		t.Errorf("searched one by one: %q", fake.queries)
	}

	// again, all from the cache (bindings and query results)
	if _, _, err := p.FindClosestManyContext(context.Background(), queries); err != nil {
		t.Fatal(err)
	}
	if fake.manyCalls != 2 || fake.idsCalls != 1 {
		t.Errorf("not cached: %d batched searches, %d id lookups", fake.manyCalls, fake.idsCalls)
	}
}

func TestRefreshByIDs(t *testing.T) {
	old := &fakeMeta{id: 1, title: "Old"}
	fake := &fakeBatcher{fakeProvider: &fakeProvider{byID: map[int]Metadata{1: old}}}
	p := newTestProvider(t, fake.fakeProvider)
	p.provider = fake
	ctx := context.Background()
	if _, err := p.SearchByIDsContext(ctx, []int{1}); err != nil {
		t.Fatal(err)
	}
	renamed := &fakeMeta{id: 1, title: "Renamed"}
	fake.byID[1] = renamed

	// the cache has the old one, a refresh asks again
	if got, err := p.SearchByIDsContext(ctx, []int{1}); err != nil || got[1].String() != "Old" || fake.idsCalls != 1 {
		t.Fatalf("cached: %v, %v, %d calls", got, err, fake.idsCalls)
	}
	got, err := p.RefreshByIDsContext(ctx, []int{1, 1, 2})
	if err != nil || len(got) != 1 || got[1].String() != "Renamed" || fake.idsCalls != 2 {
		t.Fatalf("refreshed: %v, %v, %d calls", got, err, fake.idsCalls)
	}
	if got, _ := p.SearchByIDsContext(ctx, []int{1}); got[1].String() != "Renamed" || fake.idsCalls != 2 {
		t.Errorf("not cached: %v, %d calls", got, fake.idsCalls)
	}
}

func TestCachedByIDs(t *testing.T) {
	fake := &fakeBatcher{fakeProvider: &fakeProvider{byID: map[int]Metadata{1: &fakeMeta{id: 1, title: "One"}, 2: &fakeMeta{id: 2, title: "Two"}}}}
	p := newTestProvider(t, fake.fakeProvider)
	p.provider = fake
	if _, err := p.SearchByIDsContext(context.Background(), []int{1}); err != nil {
		t.Fatal(err)
	}
	got, err := p.CachedByIDs([]int{1, 2, 1})
	if err != nil || len(got) != 1 || got[1].String() != "One" || fake.idsCalls != 1 {
		t.Errorf("cached: %v, %v, %d calls", got, err, fake.idsCalls)
	}
}

// fakeMapper is a fakeProvider that maps AniList ids, counting the calls.
type fakeMapper struct {
	*fakeProvider
	ids   map[int]int
	calls [][]int
}

func (p *fakeMapper) IDsFrom(ctx context.Context, code IDCode, ids []int) (map[int]int, error) {
	if code != IDCodeAnilist {
		return nil, errors.ErrUnsupported
	}
	p.calls = append(p.calls, ids)
	found := map[int]int{}
	for _, id := range ids {
		if mapped, ok := p.ids[id]; ok {
			found[id] = mapped
		}
	}
	return found, nil
}

func TestIDsFrom(t *testing.T) {
	fake := &fakeMapper{fakeProvider: &fakeProvider{}, ids: map[int]int{30002: 8, 31133: 2435}}
	p := newTestProvider(t, fake.fakeProvider)
	ctx := context.Background()
	if _, err := p.IDsFromContext(ctx, IDCodeAnilist, []int{1}); !errors.Is(err, errors.ErrUnsupported) {
		t.Errorf("not a mapper: %v, want ErrUnsupported", err)
	}
	p.provider = fake

	got, err := p.IDsFromContext(ctx, IDCodeAnilist, []int{30002, 1, 30002})
	if err != nil || !reflect.DeepEqual(got, map[int]int{30002: 8}) {
		t.Fatalf("got %v, %v", got, err)
	}
	// the found one is cached, the unknown one asked again with the new one
	got, err = p.IDsFromContext(ctx, IDCodeAnilist, []int{30002, 1, 31133})
	if err != nil || !reflect.DeepEqual(got, map[int]int{30002: 8, 31133: 2435}) {
		t.Fatalf("got %v, %v", got, err)
	}
	if want := [][]int{{30002, 1}, {1, 31133}}; !reflect.DeepEqual(fake.calls, want) {
		t.Errorf("calls %v, want %v", fake.calls, want)
	}
	if _, err := p.IDsFromContext(ctx, IDCodeMyAnimeList, []int{2}); !errors.Is(err, errors.ErrUnsupported) {
		t.Errorf("mal: %v, want ErrUnsupported", err)
	}
}

func TestSearchFetchesExpiredIDsInOneBatch(t *testing.T) {
	one, two := &fakeMeta{id: 1, title: "One"}, &fakeMeta{id: 2, title: "Two"}
	fake := &fakeBatcher{fakeProvider: &fakeProvider{
		byID:    map[int]Metadata{1: one, 2: two},
		byQuery: map[string][]Metadata{"q": {one, two}},
	}}
	// query results are kept, metadata expires right away
	p, err := NewProviderWithCache(ProviderWithCacheOptions{
		Provider: fake,
		CacheStore: func(_, bucketName string) (gokv.Store, error) {
			if bucketName == CacheBucketNameIDToManga {
				return expiredStore{newMemStore()}, nil
			}
			return newMemStore(), nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	p.provider = fake

	for range 2 {
		metas, err := p.SearchContext(context.Background(), "q")
		if err != nil || !reflect.DeepEqual(metas, []Metadata{one, two}) {
			t.Fatalf("got %v, %v", metas, err)
		}
	}
	if fake.manyCalls != 1 || fake.idsCalls != 1 || len(fake.idCalls) != 0 || len(fake.queries) != 0 {
		t.Errorf("%d batched searches, %d batched id lookups, %d single id lookups, %d single searches; want 1, 1, 0, 0",
			fake.manyCalls, fake.idsCalls, len(fake.idCalls), len(fake.queries))
	}
}

// partialMeta is a search result until fetched by id.
type partialMeta struct {
	fakeMeta
	partial bool
}

func (m *partialMeta) Partial() bool { return m.partial }

func TestPartialResults(t *testing.T) {
	whole := &partialMeta{fakeMeta: fakeMeta{id: 1, title: "Berserk"}}
	record := &partialMeta{fakeMeta: fakeMeta{id: 1, title: "Berserk"}, partial: true}
	other := &partialMeta{fakeMeta: fakeMeta{id: 2, title: "Zzzzzzz"}, partial: true}
	fake := &fakeProvider{
		byID:    map[int]Metadata{1: whole, 2: &partialMeta{fakeMeta: fakeMeta{id: 2, title: "Zzzzzzz"}}},
		byQuery: map[string][]Metadata{"berserk": {record, other}, "again": {record}, "nomatch": {other}},
	}
	p := newTestProvider(t, fake)
	ctx := context.Background()

	if _, err := p.SearchContext(ctx, "berserk"); err != nil {
		t.Fatal(err)
	}
	if results, err := p.SearchContext(ctx, "berserk"); err != nil || len(results) != 2 || results[0] != Metadata(record) {
		t.Errorf("cached search: %v, err %v", results, err)
	}
	if len(fake.idCalls) != 0 {
		t.Errorf("searching got ids %v", fake.idCalls)
	}
	if cached, err := p.CachedByIDs([]int{1}); err != nil || len(cached) != 0 {
		t.Errorf("CachedByIDs: %v, err %v", cached, err)
	}

	// a bad match isn't fetched, the good one is
	bad, found, err := p.FindClosestContext(ctx, Query{Titles: []string{"nomatch"}})
	if err != nil || !found || !bad.Bad() || !IsPartial(bad.Metadata) || len(fake.idCalls) != 0 {
		t.Errorf("bad match %+v, found %v, err %v, ids %v", bad, found, err, fake.idCalls)
	}
	good, found, err := p.FindClosestContext(ctx, Query{Titles: []string{"berserk"}})
	if err != nil || !found || good.Metadata != Metadata(whole) || len(fake.idCalls) != 1 {
		t.Errorf("good match %+v, found %v, err %v, ids %v", good, found, err, fake.idCalls)
	}

	// the whole one stays, a new search doesn't replace it
	if results, err := p.SearchContext(ctx, "again"); err != nil || results[0] != Metadata(whole) {
		t.Errorf("new search: %v, err %v", results, err)
	}
	if meta, ok, err := p.SearchByIDContext(ctx, 1); err != nil || !ok || meta != Metadata(whole) || len(fake.idCalls) != 1 {
		t.Errorf("by id: %v, %v, err %v, ids %v", meta, ok, err, fake.idCalls)
	}

	// a cached search result isn't the manga
	if meta, ok, err := p.SearchByIDContext(ctx, 2); err != nil || !ok || IsPartial(meta) {
		t.Errorf("by id: %v, %v, err %v", meta, ok, err)
	}
}
