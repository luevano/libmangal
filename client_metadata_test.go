package libmangal

import (
	"context"
	"testing"

	"github.com/luevano/libmangal/metadata"
)

func newTestClient(t *testing.T, metas ...metadata.Provider) *Client {
	t.Helper()
	c, err := NewClient(context.Background(), fakeLoader{&fakeProvider{}}, DefaultClientOptions())
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range metas {
		if err := c.SetMetadataProvider(withCache(t, m)); err != nil {
			t.Fatal(err)
		}
	}
	return c
}

func TestSearchByMangaWithoutMetadata(t *testing.T) {
	berserk := newAniMeta(7, "Berserk")
	ani := &fakeMetaProvider{source: metadata.IDSourceAnilist, byQuery: map[string][]metadata.Metadata{"Fake Manga": {berserk}}}
	c := newTestClient(t)
	manga := newFakeChapter(nil, 1).volume.manga

	meta, found, err := c.SearchByManga(context.Background(), withCache(t, ani), manga)
	if err != nil || !found || meta != berserk {
		t.Fatalf("got %v found=%t err=%v, want %v", meta, found, err, berserk)
	}
}

func TestSearchByMangaUsesMetadataID(t *testing.T) {
	berserk := newAniMeta(7, "Berserk")
	ani := &fakeMetaProvider{source: metadata.IDSourceAnilist, byID: map[int]metadata.Metadata{7: berserk}}
	c := newTestClient(t)
	// the manga already has metadata from this provider (e.g. bound before)
	manga := newFakeChapter(newAniMeta(7, "Berserk"), 1).volume.manga

	meta, found, err := c.SearchByManga(context.Background(), withCache(t, ani), manga)
	if err != nil || !found || meta != berserk {
		t.Fatalf("got %v found=%t err=%v, want %v", meta, found, err, berserk)
	}
	if len(ani.queries) != 0 {
		t.Errorf("searched by title %q, want an id lookup", ani.queries)
	}
}
