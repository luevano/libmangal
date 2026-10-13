package libmangal

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/luevano/libmangal/mangadata"
	"github.com/luevano/libmangal/metadata"
	"github.com/luevano/libmangal/progress"
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
	berserk := newAniMeta(7, "Fake Manga")
	ani := &fakeMetaProvider{source: metadata.IDSourceAnilist, byQuery: map[string][]metadata.Metadata{"Fake Manga": {berserk}}}
	c := newTestClient(t)
	manga := newFakeChapter(nil, 1).volume.manga

	meta, found, err := c.SearchByMangaContext(context.Background(), withCache(t, ani), manga)
	if err != nil || !found || meta != berserk {
		t.Fatalf("got %v found=%t err=%v, want %v", meta, found, err, berserk)
	}
}

func TestSearchByMangaUsesMetadataID(t *testing.T) {
	berserk := newAniMeta(7, "Fake Manga")
	ani := &fakeMetaProvider{source: metadata.IDSourceAnilist, byID: map[int]metadata.Metadata{7: berserk}}
	c := newTestClient(t)
	// the manga already has metadata from this provider (e.g. bound before)
	manga := newFakeChapter(newAniMeta(7, "Fake Manga"), 1).volume.manga

	meta, found, err := c.SearchByMangaContext(context.Background(), withCache(t, ani), manga)
	if err != nil || !found || meta != berserk {
		t.Fatalf("got %v found=%t err=%v, want %v", meta, found, err, berserk)
	}
	if len(ani.queries) != 0 {
		t.Errorf("searched by title %q, want an id lookup", ani.queries)
	}
}

func TestSearchMetadataOrder(t *testing.T) {
	ani := &fakeMetaProvider{source: metadata.IDSourceAnilist, byQuery: map[string][]metadata.Metadata{"Fake Manga": {newAniMeta(1, "Fake Manga")}}}
	mal := &fakeMetaProvider{source: metadata.IDSourceMyAnimeList, byQuery: map[string][]metadata.Metadata{"Fake Manga": {newAniMeta(2, "Fake Manga")}}}
	kitsu := &fakeMetaProvider{source: metadata.IDSourceKitsu, byQuery: map[string][]metadata.Metadata{"Fake Manga": {newAniMeta(3, "Fake Manga")}}}
	manga := newFakeChapter(nil, 1).volume.manga

	// map iteration would pick a random one now and then
	for range 50 {
		c := newTestClient(t, mal, ani, kitsu)
		meta, err := c.SearchMetadataContext(context.Background(), manga)
		if err != nil {
			t.Fatal(err)
		}
		if meta.ID().Value() != 2 {
			t.Fatalf("got %v, want the first set provider's (MAL, id 2)", meta)
		}
	}
}

func TestSearchMetadataSkipsFailingProvider(t *testing.T) {
	failing := &fakeMetaProvider{source: metadata.IDSourceAnilist, searchErr: errors.New("down")}
	mal := &fakeMetaProvider{source: metadata.IDSourceMyAnimeList, byQuery: map[string][]metadata.Metadata{"Fake Manga": {newAniMeta(2, "Fake Manga")}}}
	manga := newFakeChapter(nil, 1).volume.manga

	c := newTestClient(t, failing, mal)
	var got []string
	ctx := progress.WithReporter(context.Background(), func(s progress.Status) { got = append(got, s.String()) })
	meta, err := c.SearchMetadataContext(ctx, manga)
	if err != nil || meta == nil || meta.ID().Value() != 2 {
		t.Fatalf("got %v, %v, want the MAL result", meta, err)
	}
	want := []string{`searching fake-al for "Fake Manga"`, "fake-al failed, trying fake-mal"}
	if !slices.Equal(got, want) {
		t.Errorf("reported %q, want %q", got, want)
	}

	c = newTestClient(t, failing)
	if _, err := c.SearchMetadataContext(context.Background(), manga); err == nil {
		t.Fatal("got no error, want the failing provider's")
	}
}

func TestReadChapterSyncsEnabledProviders(t *testing.T) {
	openFile = func(string) error { return nil }
	t.Cleanup(func() { openFile = open.Run })

	newProviders := func() (*fakeMetaProvider, *fakeMetaProvider, *fakeMetaProvider) {
		ani := &fakeMetaProvider{source: metadata.IDSourceAnilist, byQuery: map[string][]metadata.Metadata{"Fake Manga": {newAniMeta(7, "Fake Manga")}}}
		mal := &fakeMetaProvider{source: metadata.IDSourceMyAnimeList, byQuery: map[string][]metadata.Metadata{"Fake Manga": {newAniMeta(8, "Fake Manga")}}}
		kitsu := &fakeMetaProvider{source: metadata.IDSourceKitsu, byQuery: map[string][]metadata.Metadata{"Fake Manga": {newAniMeta(9, "Fake Manga")}}}
		return ani, mal, kitsu
	}
	chapter := newFakeChapter(nil, 1)
	chapter.info.Number = 12.5

	tests := []struct {
		name                        string
		options                     ReadOptions
		wantAni, wantMAL, wantKitsu bool
	}{
		{"anilist only", ReadOptions{SaveAnilist: true}, true, false, false},
		{"mal only", ReadOptions{SaveMyAnimeList: true}, false, true, false},
		{"kitsu only", ReadOptions{SaveKitsu: true}, false, false, true},
		{"all", ReadOptions{SaveAnilist: true, SaveMyAnimeList: true, SaveKitsu: true}, true, true, true},
		{"none", ReadOptions{}, false, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ani, mal, kitsu := newProviders()
			c := newTestClient(t, ani, mal, kitsu)
			if err := c.ReadChapterContext(context.Background(), "x.cbz", chapter, tt.options); err != nil {
				t.Fatal(err)
			}
			if got := ani.progress[7] == 12; got != tt.wantAni {
				t.Errorf("anilist progress %v, want set=%t", ani.progress, tt.wantAni)
			}
			if got := mal.progress[8] == 12; got != tt.wantMAL {
				t.Errorf("mal progress %v, want set=%t", mal.progress, tt.wantMAL)
			}
			if got := kitsu.progress[9] == 12; got != tt.wantKitsu {
				t.Errorf("kitsu progress %v, want set=%t", kitsu.progress, tt.wantKitsu)
			}
		})
	}
}

func TestSearchByMangaSkipsBadMatches(t *testing.T) {
	spinoff := newAniMeta(9, "Fake Manga: Side Story")
	ani := &fakeMetaProvider{source: metadata.IDSourceAnilist, byQuery: map[string][]metadata.Metadata{"Fake Manga": {spinoff}}}
	c := newTestClient(t)
	cached := withCache(t, ani)
	manga := newFakeChapter(nil, 1).volume.manga

	match, found, err := c.MatchMangaContext(context.Background(), cached, manga)
	if err != nil || !found || match.Metadata != spinoff || !match.Bad() {
		t.Fatalf("got %+v found=%t err=%v, want the spin-off as a bad match", match, found, err)
	}
	if meta, found, err := c.SearchByMangaContext(context.Background(), cached, manga); err != nil || found || meta != nil {
		t.Errorf("got %v found=%t err=%v, want the bad match not used", meta, found, err)
	}
}

func TestMatchMangas(t *testing.T) {
	byID := newAniMeta(7, "Fake Manga")
	byTitle := newAniMeta(8, "Other Manga")
	ani := &fakeMetaProvider{
		source:  metadata.IDSourceAnilist,
		byID:    map[int]metadata.Metadata{7: byID},
		byQuery: map[string][]metadata.Metadata{"Other Manga": {byTitle}},
	}
	c := newTestClient(t)
	withID := newFakeChapter(newAniMeta(7, "Fake Manga"), 1).volume.manga
	other := newFakeChapter(nil, 1).volume.manga
	other.info.Title = "Other Manga"
	untitled := newFakeChapter(nil, 1).volume.manga
	untitled.info.Title = ""

	matches, found, err := c.MatchMangasContext(context.Background(), withCache(t, ani), []mangadata.Manga{withID, other, untitled})
	if err != nil {
		t.Fatal(err)
	}
	if !found[0] || matches[0].Metadata != byID || !matches[0].Bound {
		t.Errorf("by id: %+v", matches[0])
	}
	if !found[1] || matches[1].Metadata != byTitle || matches[1].Bad() {
		t.Errorf("by title: %+v", matches[1])
	}
	if found[2] {
		t.Errorf("untitled found: %+v", matches[2])
	}
}
