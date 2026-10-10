package metadata

import (
	"context"
	"errors"
	"strings"
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
			if _, _, err := p.SearchByIDContext(context.Background(), 7); err != nil {
				t.Error(err)
			}
			if _, err := p.SearchContext(context.Background(), "berserk"); err != nil {
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
		metas, err := p.SearchContext(context.Background(), "berserk")
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
		match, found, err := p.FindClosestContext(context.Background(), Query{Titles: []string{"berserk"}})
		if err != nil || !found {
			t.Fatalf("got found=%t err=%v", found, err)
		}
		if match.Metadata != bound || !match.Bound || match.Bad() {
			t.Fatalf("got %+v, want the bound %v", match, bound)
		}
	}
	if len(fake.queries) != 0 {
		t.Errorf("searched by title %v, want only id lookups", fake.queries)
	}
}

func TestCacheErrorsWrap(t *testing.T) {
	errStore := errors.New("store down")
	p, err := NewProviderWithCache(ProviderWithCacheOptions{
		Provider:   &fakeProvider{},
		CacheStore: func(string, string) (gokv.Store, error) { return nil, errStore },
	})
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = p.SearchByIDContext(context.Background(), 7)
	if !errors.Is(err, errStore) {
		t.Fatalf("got %v, want it to wrap %v", err, errStore)
	}
	if want := `fake: cache: get id-to-manga "7": store down`; err.Error() != want {
		t.Errorf("got %q, want %q", err, want)
	}
}

func TestValidateWraps(t *testing.T) {
	err := Validate(&fakeMeta{id: 0, title: "x"})
	if err == nil || !strings.HasPrefix(err.Error(), "invalid metadata: ") {
		t.Errorf("got %v", err)
	}
}
