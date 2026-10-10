package libmangal

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/luevano/libmangal/logger"
	"github.com/luevano/libmangal/mangadata"
	"github.com/luevano/libmangal/metadata"
)

type fakeManga struct {
	info mangadata.MangaInfo
	meta metadata.Metadata
}

func (m *fakeManga) String() string                     { return m.info.Title }
func (m *fakeManga) Info() mangadata.MangaInfo          { return m.info }
func (m *fakeManga) Metadata() metadata.Metadata        { return m.meta }
func (m *fakeManga) SetMetadata(meta metadata.Metadata) { m.meta = meta }

type fakeVolume struct {
	manga *fakeManga
}

func (v *fakeVolume) String() string             { return "volume 1" }
func (v *fakeVolume) Info() mangadata.VolumeInfo { return mangadata.VolumeInfo{Number: 1} }
func (v *fakeVolume) Manga() mangadata.Manga     { return v.manga }

type fakeChapter struct {
	info   mangadata.ChapterInfo
	volume *fakeVolume
	pages  int
}

func (c *fakeChapter) String() string              { return c.info.Title }
func (c *fakeChapter) Info() mangadata.ChapterInfo { return c.info }
func (c *fakeChapter) Volume() mangadata.Volume    { return c.volume }

type fakePage struct {
	chapter *fakeChapter
	n       int
}

func (p *fakePage) String() string             { return fmt.Sprintf("page %d", p.n) }
func (p *fakePage) Extension() string          { return ".jpg" }
func (p *fakePage) Chapter() mangadata.Chapter { return p.chapter }

// fakeProvider serves chapters made with newFakeChapter, images are a few bytes
// with the page number.
type fakeProvider struct {
	imageErr error
	// inFlight/maxInFlight track concurrent GetPageImage calls
	inFlight, maxInFlight atomic.Int32
	mu                    sync.Mutex
}

func (p *fakeProvider) String() string { return "fake" }
func (p *fakeProvider) Close() error   { return nil }
func (p *fakeProvider) Info() ProviderInfo {
	return ProviderInfo{ID: "fake", Name: "Fake", Version: "0.1.0"}
}
func (p *fakeProvider) SetLogger(*logger.Logger) {}

func (p *fakeProvider) SearchMangas(context.Context, string) ([]mangadata.Manga, error) {
	return nil, errors.New("not implemented")
}

func (p *fakeProvider) MangaVolumes(context.Context, mangadata.Manga) ([]mangadata.Volume, error) {
	return nil, errors.New("not implemented")
}

func (p *fakeProvider) VolumeChapters(context.Context, mangadata.Volume) ([]mangadata.Chapter, error) {
	return nil, errors.New("not implemented")
}

func (p *fakeProvider) ChapterPages(_ context.Context, chapter mangadata.Chapter) ([]mangadata.Page, error) {
	c := chapter.(*fakeChapter)
	pages := make([]mangadata.Page, c.pages)
	for i := range pages {
		pages[i] = &fakePage{chapter: c, n: i + 1}
	}
	return pages, nil
}

func (p *fakeProvider) GetPageImage(ctx context.Context, page mangadata.Page) ([]byte, error) {
	n := p.inFlight.Add(1)
	defer p.inFlight.Add(-1)
	p.mu.Lock()
	if n > p.maxInFlight.Load() {
		p.maxInFlight.Store(n)
	}
	p.mu.Unlock()

	if p.imageErr != nil {
		return nil, p.imageErr
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return []byte(page.String()), nil
}

type fakeLoader struct {
	provider Provider
}

func (l fakeLoader) String() string                         { return "fake" }
func (l fakeLoader) Info() ProviderInfo                     { return l.provider.Info() }
func (l fakeLoader) Load(context.Context) (Provider, error) { return l.provider, nil }

func newFakeChapter(meta metadata.Metadata, pages int) *fakeChapter {
	manga := &fakeManga{
		info: mangadata.MangaInfo{Title: "Fake Manga", ID: "fake-manga", URL: "https://example.com/manga"},
		meta: meta,
	}
	return &fakeChapter{
		info:   mangadata.ChapterInfo{Title: "Chapter 1", Number: 1},
		volume: &fakeVolume{manga: manga},
		pages:  pages,
	}
}

// validMeta is metadata that passes metadata.Validate.
func validMeta() *mangadata.Metadata {
	return &mangadata.Metadata{
		EnglishTitle:      "Fake Manga",
		AuthorList:        []string{"Someone"},
		DateStart:         metadata.Date{Year: 2020, Month: 1, Day: 2},
		PublicationStatus: metadata.StatusFinished,
		ProviderID:        "fake-manga",
		ProviderIDCode:    "fk",
	}
}

// errWriter fails every write.
type errWriter struct{}

var errWrite = errors.New("write failed")

func (errWriter) Write([]byte) (int, error) { return 0, errWrite }

func newTestLogger() *logger.Logger { return logger.NewLogger() }
