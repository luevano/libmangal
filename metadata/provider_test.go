package metadata

import (
	"context"
	"sync"
	"testing"
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
