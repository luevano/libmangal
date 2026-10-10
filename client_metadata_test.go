package libmangal

import (
	"context"
	"errors"
	"testing"

	"github.com/luevano/libmangal/metadata"
	"github.com/skratchdot/open-golang/open"
)

func newTestClient(t *testing.T, metas ...metadata.Provider) *Client {
	t.Helper()
	c, err := NewClientContext(context.Background(), fakeLoader{&fakeProvider{}}, DefaultClientOptions())
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

	meta, found, err := c.SearchByMangaContext(context.Background(), withCache(t, ani), manga)
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

	meta, found, err := c.SearchByMangaContext(context.Background(), withCache(t, ani), manga)
	if err != nil || !found || meta != berserk {
		t.Fatalf("got %v found=%t err=%v, want %v", meta, found, err, berserk)
	}
	if len(ani.queries) != 0 {
		t.Errorf("searched by title %q, want an id lookup", ani.queries)
	}
}

func TestSearchMetadataOrder(t *testing.T) {
	ani := &fakeMetaProvider{source: metadata.IDSourceAnilist, byQuery: map[string][]metadata.Metadata{"Fake Manga": {newAniMeta(1, "From AniList")}}}
	mal := &fakeMetaProvider{source: metadata.IDSourceMyAnimeList, byQuery: map[string][]metadata.Metadata{"Fake Manga": {newAniMeta(2, "From MAL")}}}
	kitsu := &fakeMetaProvider{source: metadata.IDSourceKitsu, byQuery: map[string][]metadata.Metadata{"Fake Manga": {newAniMeta(3, "From Kitsu")}}}
	manga := newFakeChapter(nil, 1).volume.manga

	// map iteration would pick a random one now and then
	for range 50 {
		c := newTestClient(t, mal, ani, kitsu)
		meta, err := c.SearchMetadataContext(context.Background(), manga)
		if err != nil {
			t.Fatal(err)
		}
		if meta.Title() != "From MAL" {
			t.Fatalf("got %q, want the first set provider (MAL)", meta.Title())
		}
	}
}

func TestSearchMetadataSkipsFailingProvider(t *testing.T) {
	failing := &fakeMetaProvider{source: metadata.IDSourceAnilist, searchErr: errors.New("down")}
	mal := &fakeMetaProvider{source: metadata.IDSourceMyAnimeList, byQuery: map[string][]metadata.Metadata{"Fake Manga": {newAniMeta(2, "From MAL")}}}
	manga := newFakeChapter(nil, 1).volume.manga

	c := newTestClient(t, failing, mal)
	meta, err := c.SearchMetadataContext(context.Background(), manga)
	if err != nil || meta == nil || meta.Title() != "From MAL" {
		t.Fatalf("got %v, %v, want the MAL result", meta, err)
	}

	c = newTestClient(t, failing)
	if _, err := c.SearchMetadataContext(context.Background(), manga); err == nil {
		t.Fatal("got no error, want the failing provider's")
	}
}

func TestReadChapterSyncsEnabledProviders(t *testing.T) {
	openFile = func(string) error { return nil }
	t.Cleanup(func() { openFile = open.Run })

	newProviders := func() (*fakeMetaProvider, *fakeMetaProvider) {
		ani := &fakeMetaProvider{source: metadata.IDSourceAnilist, byQuery: map[string][]metadata.Metadata{"Fake Manga": {newAniMeta(7, "Berserk")}}}
		mal := &fakeMetaProvider{source: metadata.IDSourceMyAnimeList, byQuery: map[string][]metadata.Metadata{"Fake Manga": {newAniMeta(8, "Berserk")}}}
		return ani, mal
	}
	chapter := newFakeChapter(nil, 1)
	chapter.info.Number = 12.5

	tests := []struct {
		name             string
		options          ReadOptions
		wantAni, wantMAL bool
	}{
		{"anilist only", ReadOptions{SaveAnilist: true}, true, false},
		{"mal only", ReadOptions{SaveMyAnimeList: true}, false, true},
		{"both", ReadOptions{SaveAnilist: true, SaveMyAnimeList: true}, true, true},
		{"none", ReadOptions{SaveHistory: true}, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ani, mal := newProviders()
			c := newTestClient(t, ani, mal)
			if err := c.ReadChapterContext(context.Background(), "x.cbz", chapter, tt.options); err != nil {
				t.Fatal(err)
			}
			if got := ani.progress[7] == 12; got != tt.wantAni {
				t.Errorf("anilist progress %v, want set=%t", ani.progress, tt.wantAni)
			}
			if got := mal.progress[8] == 12; got != tt.wantMAL {
				t.Errorf("mal progress %v, want set=%t", mal.progress, tt.wantMAL)
			}
		})
	}
}
