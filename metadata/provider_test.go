package metadata

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"testing"

	"github.com/philippgille/gokv"
)

func newTestProvider(t *testing.T, p *fakeProvider) *ProviderWithCache {
	t.Helper()
	options := DefaultProviderWithCacheOptions()
	options.Provider = p
	pwc, err := NewProviderWithCache(options)
	if err != nil {
		t.Fatal(err)
	}
	return pwc
}

func TestProviderWithCacheConcurrent(t *testing.T) {
	berserk := &fakeMeta{id: 7, title: "Berserk"}
	p := newTestProvider(t, &fakeProvider{
		byID:    map[int]Metadata{7: berserk},
		byQuery: map[string][]Metadata{"berserk": {berserk}},
	})

	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			if _, _, err := p.SearchByID(context.Background(), 7); err != nil {
				t.Error(err)
			}
			if _, err := p.Search(context.Background(), "berserk"); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
}

func TestDefaultCacheStoreCaches(t *testing.T) {
	berserk := &fakeMeta{id: 7, title: "Berserk"}
	fake := &fakeProvider{byQuery: map[string][]Metadata{"berserk": {berserk}}}
	p := newTestProvider(t, fake)

	for range 2 {
		metas, err := p.Search(context.Background(), "berserk")
		if err != nil {
			t.Fatal(err)
		}
		if len(metas) != 1 || metas[0] != berserk {
			t.Fatalf("got %v, want [%v]", metas, berserk)
		}
	}
	if len(fake.queries) != 1 {
		t.Errorf("provider searched %d times, want 1 (second search from the cache)", len(fake.queries))
	}
	if len(fake.idCalls) != 0 {
		t.Errorf("provider got %d id lookups, want 0", len(fake.idCalls))
	}
}

func TestFindClosestKeepsBindingWhenMetadataExpires(t *testing.T) {
	bound := &fakeMeta{id: 42, title: "Berserk of Gluttony"}
	first := &fakeMeta{id: 7, title: "Berserk"}
	fake := &fakeProvider{
		byID:    map[int]Metadata{42: bound, 7: first},
		byQuery: map[string][]Metadata{"berserk": {first, bound}},
	}

	// title bindings never expire, cached metadata does: a store with no metadata
	titles := newMemStore()
	options := ProviderWithCacheOptions{
		Provider: fake,
		CacheStore: func(_, bucketName string) (gokv.Store, error) {
			if bucketName == CacheBucketNameTitleToID {
				return titles, nil
			}
			return newMemStore(), nil
		},
	}
	p, err := NewProviderWithCache(options)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.BindTitleWithID("berserk", 42); err != nil {
		t.Fatal(err)
	}

	for range 2 {
		meta, found, err := p.FindClosest(context.Background(), "berserk", 3, 3)
		if err != nil || !found {
			t.Fatalf("got found=%t err=%v", found, err)
		}
		if meta != bound {
			t.Fatalf("got %v, want the bound %v", meta, bound)
		}
	}
	if len(fake.queries) != 0 {
		t.Errorf("searched by title %v, want only id lookups", fake.queries)
	}
}

func TestFindClosestTrimsRunes(t *testing.T) {
	fake := &fakeProvider{}
	p := newTestProvider(t, fake)

	if _, found, err := p.FindClosest(context.Background(), "ベルセルク", 3, 3); err != nil || found {
		t.Fatalf("got found=%t err=%v", found, err)
	}
	want := []string{"ベルセルク", "ベル", "ベ"}
	if len(fake.queries) != len(want) {
		t.Fatalf("queries %q, want %q", fake.queries, want)
	}
	for i, q := range fake.queries {
		if q != want[i] {
			t.Errorf("query %d: got %q, want %q", i, q, want[i])
		}
	}
}

func TestFindClosestLogsID(t *testing.T) {
	berserk := &fakeMeta{id: 7, title: "Berserk"}
	p := newTestProvider(t, &fakeProvider{byQuery: map[string][]Metadata{"berserk": {berserk}}})
	var logs []string
	p.Logger().SetOnLog(func(format string, a ...any) { logs = append(logs, fmt.Sprintf(format, a...)) })

	if _, _, err := p.FindClosest(context.Background(), "berserk", 1, 1); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(logs, `found closest: "Berserk" with id 7`) {
		t.Errorf("logs %q, want the closest id", logs)
	}
}
