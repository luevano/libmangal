package libmangal

import (
	"context"
	"errors"
	"testing"

	"github.com/luevano/libmangal/mangadata"
)

// listProvider returns the lists it's given.
type listProvider struct {
	*fakeProvider
	mangas   []mangadata.Manga
	volumes  []mangadata.Volume
	chapters []mangadata.Chapter
	pages    []mangadata.Page
}

func (p listProvider) SearchMangas(context.Context, string) ([]mangadata.Manga, error) {
	return p.mangas, nil
}

func (p listProvider) MangaVolumes(context.Context, mangadata.Manga) ([]mangadata.Volume, error) {
	return p.volumes, nil
}

func (p listProvider) VolumeChapters(context.Context, mangadata.Volume) ([]mangadata.Chapter, error) {
	return p.chapters, nil
}

func (p listProvider) ChapterPages(context.Context, mangadata.Chapter) ([]mangadata.Page, error) {
	return p.pages, nil
}

func newListClient(t *testing.T, p listProvider) *Client {
	t.Helper()
	p.fakeProvider = &fakeProvider{}
	c, err := NewClientContext(t.Context(), fakeLoader{p}, DefaultClientOptions())
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestMangaByIDInvalid(t *testing.T) {
	provider := byIDProvider{fakeProvider: &fakeProvider{}, mangas: map[string]*fakeManga{
		"empty":    {},
		"untitled": {info: mangadata.MangaInfo{ID: "untitled"}},
		"blank":    {info: mangadata.MangaInfo{ID: " ", Title: "Blank"}},
		"nil":      nil,
	}}
	c, err := NewClientContext(t.Context(), fakeLoader{provider}, DefaultClientOptions())
	if err != nil {
		t.Fatal(err)
	}
	for id := range provider.mangas {
		if manga, found, err := c.MangaByID(id); !errors.Is(err, ErrInvalidResult) || found || manga != nil {
			t.Errorf("MangaByID(%s) = %v, %v, %v; want ErrInvalidResult", id, manga, found, err)
		}
	}
}

func TestSearchDropsInvalid(t *testing.T) {
	valid := &fakeManga{info: mangadata.MangaInfo{ID: "1", Title: "One"}}
	c := newListClient(t, listProvider{mangas: []mangadata.Manga{
		&fakeManga{}, valid, (*fakeManga)(nil), nil, &fakeManga{info: mangadata.MangaInfo{ID: "2"}},
	}})
	got, err := c.SearchMangas("q")
	if err != nil || len(got) != 1 || got[0] != valid {
		t.Fatalf("SearchMangas = %v, %v; want only the valid manga", got, err)
	}

	c = newListClient(t, listProvider{mangas: []mangadata.Manga{&fakeManga{}, &fakeManga{}}})
	if got, err := c.SearchMangas("q"); !errors.Is(err, ErrInvalidResult) {
		t.Errorf("all empty: %v, %v; want ErrInvalidResult", got, err)
	}

	c = newListClient(t, listProvider{})
	if got, err := c.SearchMangas("q"); err != nil || len(got) != 0 {
		t.Errorf("no results: %v, %v; want none", got, err)
	}
}

func TestVolumesChaptersPagesDropInvalid(t *testing.T) {
	chapter := newFakeChapter(validMeta(), 1)
	volume := chapter.volume
	page := &fakePage{chapter: chapter, n: 1}
	c := newListClient(t, listProvider{
		volumes:  []mangadata.Volume{nil, volume, (*fakeVolume)(nil)},
		chapters: []mangadata.Chapter{(*fakeChapter)(nil), chapter},
		pages:    []mangadata.Page{page, nil},
	})

	if got, err := c.MangaVolumes(volume.manga); err != nil || len(got) != 1 || got[0] != volume {
		t.Errorf("MangaVolumes = %v, %v; want only the valid volume", got, err)
	}
	if got, err := c.VolumeChapters(volume); err != nil || len(got) != 1 || got[0] != chapter {
		t.Errorf("VolumeChapters = %v, %v; want only the valid chapter", got, err)
	}
	if got, err := c.ChapterPages(chapter); !errors.Is(err, ErrInvalidResult) {
		t.Errorf("ChapterPages = %v, %v; want ErrInvalidResult", got, err)
	}

	c = newListClient(t, listProvider{chapters: []mangadata.Chapter{nil}})
	if got, err := c.VolumeChapters(volume); !errors.Is(err, ErrInvalidResult) {
		t.Errorf("only a nil chapter: %v, %v; want ErrInvalidResult", got, err)
	}
}
