package libmangal

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/luevano/libmangal/mangadata"
)

// newNamedClient names manga directories with name, like a name template.
func newNamedClient(t *testing.T, provider ProviderInfo, name string) *Client {
	t.Helper()
	options := DefaultClientOptions()
	options.MangaName = func(ProviderInfo, mangadata.Manga) string { return name }
	c, err := NewClientContext(context.Background(), fakeLoader{&infoProvider{fakeProvider: &fakeProvider{}, info: provider}}, options)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

type infoProvider struct {
	*fakeProvider
	info ProviderInfo
}

func (p *infoProvider) Info() ProviderInfo { return p.info }

func listDirs(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestReuseMangaDir(t *testing.T) {
	fake := ProviderInfo{ID: "fake", Name: "Fake", Version: "0.1.0"}
	options := DefaultDownloadOptions()
	options.Directory = t.TempDir()
	options.SearchMetadata = false

	first := newFakeChapter(validMeta(), 1)
	if _, err := newNamedClient(t, fake, "Old Title").DownloadChapterContext(context.Background(), first, options); err != nil {
		t.Fatal(err)
	}
	info, err := ReadMangaDirInfo(newTestClient(t).FS(), filepath.Join(options.Directory, "Old Title"))
	if err != nil || info.Provider != "fake" || info.Manga.ID != "fake-manga" {
		t.Fatalf("got %+v, %v", info, err)
	}

	// the title changed (or the template), a new client like a new mangal run
	c := newNamedClient(t, fake, "New Title")
	second := newFakeChapter(validMeta(), 1)
	second.info.Number, second.info.Title = 2, "Chapter 2"
	dir, err := c.ChapterDir(second, options)
	if err != nil || dir != filepath.Join(options.Directory, "Old Title") {
		t.Fatalf("ChapterDir: %q, %v", dir, err)
	}
	down, err := c.DownloadChapterContext(context.Background(), second, options)
	if err != nil {
		t.Fatal(err)
	}
	if down.Directory != dir {
		t.Errorf("downloaded to %q, want %q", down.Directory, dir)
	}

	// the provider was renamed: its old id still finds the directory
	renamed := ProviderInfo{ID: "fake2", Name: "Fake 2", Version: "0.1.0", History: []ProviderAlias{{ID: "fake"}}}
	if dir, _ := newNamedClient(t, renamed, "Other").ChapterDir(second, options); dir != filepath.Join(options.Directory, "Old Title") {
		t.Errorf("renamed provider: %q", dir)
	}

	// without reusing, the template wins
	options.ReuseMangaDir = false
	if _, err := newNamedClient(t, fake, "New Title").DownloadChapterContext(context.Background(), second, options); err != nil {
		t.Fatal(err)
	}
	if got := listDirs(t, options.Directory); len(got) != 2 {
		t.Errorf("directories %q, want Old Title and New Title", got)
	}
}

func TestReuseMangaDirByMetadataID(t *testing.T) {
	options := DefaultDownloadOptions()
	options.Directory = t.TempDir()
	options.SearchMetadata = false

	withAnilist := func() *aniMeta { return newAniMeta(7, "Fake Manga") }
	chapter := newFakeChapter(withAnilist(), 1)
	if _, err := newNamedClient(t, ProviderInfo{ID: "one", Name: "One", Version: "0.1.0"}, "From One").DownloadChapterContext(context.Background(), chapter, options); err != nil {
		t.Fatal(err)
	}

	// another provider, another manga id, same Anilist id
	other := newFakeChapter(withAnilist(), 1)
	other.volume.manga.info.ID = "other-id"
	dir, err := newNamedClient(t, ProviderInfo{ID: "two", Name: "Two", Version: "0.1.0"}, "From Two").ChapterDir(other, options)
	if err != nil || dir != filepath.Join(options.Directory, "From One") {
		t.Errorf("got %q, %v, want the directory with the same Anilist id", dir, err)
	}

	// metadata of the provider itself doesn't count
	plain := newFakeChapter(validMeta(), 1)
	plain.volume.manga.info.ID = "other-id"
	if dir, _ := newNamedClient(t, ProviderInfo{ID: "two", Name: "Two", Version: "0.1.0"}, "From Two").ChapterDir(plain, options); dir != filepath.Join(options.Directory, "From Two") {
		t.Errorf("got %q", dir)
	}
}

func TestReuseOldProviderDir(t *testing.T) {
	options := DefaultDownloadOptions()
	options.Directory = t.TempDir()
	options.SearchMetadata = false
	options.CreateProviderDir = true
	chapter := newFakeChapter(validMeta(), 1)

	old := ProviderInfo{ID: "mango-mangasee", Name: "MangaSee", Version: "0.1.0"}
	if _, err := newNamedClient(t, old, "Manga").DownloadChapterContext(context.Background(), chapter, options); err != nil {
		t.Fatal(err)
	}

	renamed := ProviderInfo{ID: "mango-mangasee", Name: "Weeb Central", Version: "0.1.0", History: []ProviderAlias{{ID: "mango-mangasee", Name: "MangaSee"}}}
	dir, err := newNamedClient(t, renamed, "Manga").ChapterDir(chapter, options)
	if err != nil || dir != filepath.Join(options.Directory, "MangaSee", "Manga") {
		t.Errorf("got %q, %v, want the old provider directory", dir, err)
	}

	options.ReuseMangaDir = false
	if dir, _ := newNamedClient(t, renamed, "Manga").ChapterDir(chapter, options); dir != filepath.Join(options.Directory, "Weeb Central", "Manga") {
		t.Errorf("got %q without reusing", dir)
	}
}
