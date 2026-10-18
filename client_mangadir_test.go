package libmangal

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/luevano/libmangal/mangadata"
	"github.com/spf13/afero"
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

func TestReuseMangaDirDeleted(t *testing.T) {
	fake := ProviderInfo{ID: "fake", Name: "Fake", Version: "0.1.0"}
	options := DefaultDownloadOptions()
	options.Directory = t.TempDir()
	options.SearchMetadata = false

	clientOptions := DefaultClientOptions()
	clientOptions.MangaName = func(ProviderInfo, mangadata.Manga) string { return "Title" }
	clientOptions.MangaDirIndex = &memIndex{dirs: map[string]MangaDirInfo{}}
	c, err := NewClientContext(context.Background(), fakeLoader{&infoProvider{fakeProvider: &fakeProvider{}, info: fake}}, clientOptions)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.DownloadChapterContext(context.Background(), newFakeChapter(validMeta(), 1), options); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(options.Directory, "Title")); err != nil {
		t.Fatal(err)
	}
	// the same client: its scan cache still has the deleted directory
	down, err := c.DownloadChapterContext(context.Background(), newFakeChapter(validMeta(), 1), options)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(options.Directory, "Title"); down.Directory != want {
		t.Errorf("downloaded to %q, want %q", down.Directory, want)
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

type memIndex struct {
	dirs map[string]MangaDirInfo // parent/name -> info
}

func (m *memIndex) FindMangaDir(parent string, provider ProviderInfo, info MangaDirInfo) (string, bool, error) {
	best, name := MangaDirNoMatch, ""
	for path, have := range m.dirs {
		if filepath.Dir(path) != parent {
			continue
		}
		if match := MatchMangaDir(provider, info, have); match > best {
			best, name = match, filepath.Base(path)
		}
	}
	return name, best != MangaDirNoMatch, nil
}

func (m *memIndex) RememberMangaDir(parent, name string, info MangaDirInfo) error {
	m.dirs[filepath.Join(parent, name)] = info
	return nil
}

func TestMangaDirIndex(t *testing.T) {
	fake := ProviderInfo{ID: "fake", Name: "Fake", Version: "0.1.0"}
	index := &memIndex{dirs: map[string]MangaDirInfo{}}
	newClient := func(name string) *Client {
		options := DefaultClientOptions()
		options.MangaName = func(ProviderInfo, mangadata.Manga) string { return name }
		options.MangaDirIndex = index
		c, err := NewClientContext(context.Background(), fakeLoader{&infoProvider{fakeProvider: &fakeProvider{}, info: fake}}, options)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	options := DefaultDownloadOptions()
	options.Directory = t.TempDir()
	options.SearchMetadata = false
	chapter := newFakeChapter(validMeta(), 1)

	// a download is remembered
	if _, err := newClient("Old").DownloadChapterContext(context.Background(), chapter, options); err != nil {
		t.Fatal(err)
	}
	if _, ok := index.dirs[filepath.Join(options.Directory, "Old")]; !ok {
		t.Fatalf("not remembered: %v", index.dirs)
	}

	// the index is used without the marker
	if err := os.Remove(filepath.Join(options.Directory, "Old", FilenameMangaDirInfo)); err != nil {
		t.Fatal(err)
	}
	if dir, _ := newClient("New").ChapterDir(chapter, options); dir != filepath.Join(options.Directory, "Old") {
		t.Errorf("got %q, want the indexed directory", dir)
	}

	// a stale entry falls back to the markers, which seed the index
	index.dirs = map[string]MangaDirInfo{filepath.Join(options.Directory, "Gone"): newMangaDirInfo(fake, chapter.volume.manga)}
	if _, err := newClient("Marked").DownloadChapterContext(context.Background(), chapter, options); err != nil {
		t.Fatal(err)
	}
	index.dirs = map[string]MangaDirInfo{filepath.Join(options.Directory, "Gone"): newMangaDirInfo(fake, chapter.volume.manga)}
	if dir, _ := newClient("Other").ChapterDir(chapter, options); dir != filepath.Join(options.Directory, "Marked") {
		t.Errorf("got %q, want the marked directory", dir)
	}
	if _, ok := index.dirs[filepath.Join(options.Directory, "Marked")]; !ok {
		t.Errorf("marker match not added to the index: %v", index.dirs)
	}
}

func TestWriteMangaDirInfo(t *testing.T) {
	fsys := afero.NewMemMapFs()
	if err := fsys.MkdirAll("/m", 0o755); err != nil {
		t.Fatal(err)
	}
	info := MangaDirInfo{Provider: "p", Manga: mangadata.MangaInfo{ID: "x", Title: "X"}}
	if err := WriteMangaDirInfo(fsys, "/m", info, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ReadMangaDirInfo(fsys, "/m")
	if err != nil || got.Provider != "p" || got.Manga.ID != "x" {
		t.Fatalf("got %+v, %v", got, err)
	}
	// only the file itself is left, no temp files
	entries, _ := afero.ReadDir(fsys, "/m")
	if len(entries) != 1 || entries[0].Name() != FilenameMangaDirInfo {
		t.Errorf("entries %v", entries)
	}
}

type ctxKey struct{}

// ctxIndex records the context values its Context methods got.
type ctxIndex struct {
	memIndex
	got []any
}

func (m *ctxIndex) FindMangaDirContext(ctx context.Context, parent string, provider ProviderInfo, info MangaDirInfo) (string, bool, error) {
	m.got = append(m.got, ctx.Value(ctxKey{}))
	return m.FindMangaDir(parent, provider, info)
}

func (m *ctxIndex) RememberMangaDirContext(ctx context.Context, parent, name string, info MangaDirInfo) error {
	m.got = append(m.got, ctx.Value(ctxKey{}))
	return m.RememberMangaDir(parent, name, info)
}

func (m *ctxIndex) FindChapter(string, mangadata.Chapter, Format) (string, string, bool, error) {
	return "", "", false, nil
}

func (m *ctxIndex) FindChapterContext(ctx context.Context, mangaDir string, chapter mangadata.Chapter, format Format) (string, string, bool, error) {
	m.got = append(m.got, ctx.Value(ctxKey{}))
	return m.FindChapter(mangaDir, chapter, format)
}

func TestIndexContext(t *testing.T) {
	index := &ctxIndex{dirs: map[string]MangaDirInfo{}}
	options := DefaultClientOptions()
	options.MangaDirIndex = index
	options.ChapterIndex = index
	c, err := NewClientContext(context.Background(), fakeLoader{&fakeProvider{}}, options)
	if err != nil {
		t.Fatal(err)
	}
	down := DefaultDownloadOptions()
	down.Directory = t.TempDir()
	down.SearchMetadata = false
	down.ReuseMangaDir = true
	ctx := context.WithValue(context.Background(), ctxKey{}, "v")
	if _, err := c.DownloadChapterContext(ctx, newFakeChapter(validMeta(), 1), down); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ChapterDirContext(ctx, newFakeChapter(validMeta(), 1), down); err != nil {
		t.Fatal(err)
	}
	// find + chapter + remember on download, find on ChapterDirContext
	if len(index.got) != 4 {
		t.Fatalf("got %d context calls, want 4: %v", len(index.got), index.got)
	}
	for _, v := range index.got {
		if v != "v" {
			t.Errorf("got context value %v, want the caller's", v)
		}
	}
}
