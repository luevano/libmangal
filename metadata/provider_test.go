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

	// title bindings never expire, cached metadata does: the other buckets keep nothing
	options := ProviderWithCacheOptions{
		Provider: fake,
		CacheStore: func(_, bucketName string) (gokv.Store, error) {
			if bucketName == CacheBucketNameTitleToID {
				return newMemStore(), nil
			}
			return expiredStore{newMemStore()}, nil
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

// expiredStore forgets what's set, like values that expired right away.
type expiredStore struct{ memStore }

func (expiredStore) Set(string, any) error { return nil }

// failingStore fails every operation.
type failingStore struct {
	memStore
	err error
}

func (s failingStore) Get(string, any) (bool, error) { return false, s.err }

// closeCounter counts its closes.
type closeCounter struct {
	memStore
	closed *int
}

func (s closeCounter) Close() error { *s.closed++; return nil }

func TestStoreOpenedOnce(t *testing.T) {
	first := &fakeMeta{id: 7, title: "Berserk"}
	fake := &fakeProvider{byID: map[int]Metadata{7: first}, byQuery: map[string][]Metadata{"berserk": {first}}}
	opens, closes := 0, 0
	p, err := NewProviderWithCache(ProviderWithCacheOptions{
		Provider: fake,
		CacheStore: func(string, string) (gokv.Store, error) {
			opens++
			return closeCounter{newMemStore(), &closes}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if _, _, err := p.FindClosestContext(context.Background(), Query{Titles: []string{"berserk"}}); err != nil {
			t.Fatal(err)
		}
	}
	if opens != 3 || closes != 0 {
		t.Errorf("%d opens, %d closes after 3 searches, want 3 (one per bucket) and 0", opens, closes)
	}
	if err := p.Close(); err != nil || closes != 3 {
		t.Errorf("close: %v, %d closes, want 3", err, closes)
	}
}

func TestCacheErrorsWrap(t *testing.T) {
	errStore := errors.New("store down")
	_, err := NewProviderWithCache(ProviderWithCacheOptions{
		Provider:   &fakeProvider{},
		CacheStore: func(string, string) (gokv.Store, error) { return nil, errStore },
	})
	if !errors.Is(err, errStore) {
		t.Fatalf("open: got %v, want it to wrap %v", err, errStore)
	}

	p, err := NewProviderWithCache(ProviderWithCacheOptions{
		Provider:   &fakeProvider{},
		CacheStore: func(string, string) (gokv.Store, error) { return failingStore{newMemStore(), errStore}, nil },
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
