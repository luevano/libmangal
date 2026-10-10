package metadata

import (
	"context"
	"strconv"
	"sync"

	"github.com/luevano/libmangal/logger"
)

// fakeMeta is a minimal Metadata with an id.
type fakeMeta struct {
	id    int
	title string
}

func (m *fakeMeta) String() string            { return m.title }
func (m *fakeMeta) Title() string             { return m.title }
func (m *fakeMeta) AlternateTitles() []string { return nil }
func (m *fakeMeta) Score() float32            { return 0 }
func (m *fakeMeta) Description() string       { return "" }
func (m *fakeMeta) Cover() string             { return "" }
func (m *fakeMeta) Banner() string            { return "" }
func (m *fakeMeta) Tags() []string            { return nil }
func (m *fakeMeta) Genres() []string          { return nil }
func (m *fakeMeta) Characters() []string      { return nil }
func (m *fakeMeta) Authors() []string         { return nil }
func (m *fakeMeta) Artists() []string         { return nil }
func (m *fakeMeta) Translators() []string     { return nil }
func (m *fakeMeta) Letterers() []string       { return nil }
func (m *fakeMeta) StartDate() Date           { return Date{} }
func (m *fakeMeta) EndDate() Date             { return Date{} }
func (m *fakeMeta) Publisher() string         { return "" }
func (m *fakeMeta) Status() Status            { return StatusFinished }
func (m *fakeMeta) Format() string            { return "" }
func (m *fakeMeta) Country() string           { return "" }
func (m *fakeMeta) Chapters() int             { return 0 }
func (m *fakeMeta) Notes() string             { return "" }
func (m *fakeMeta) URL() string               { return "" }
func (m *fakeMeta) ID() ID {
	return ID{Raw: strconv.Itoa(m.id), Source: IDSourceAnilist, Code: IDCodeAnilist}
}
func (m *fakeMeta) ExtraIDs() []ID { return nil }

// fakeProvider answers from fixed maps and counts the calls.
type fakeProvider struct {
	byID    map[int]Metadata
	byQuery map[string][]Metadata

	mu       sync.Mutex
	queries  []string
	idCalls  []int
	logger   *logger.Logger
	searchFn func(query string) ([]Metadata, error)
}

func (p *fakeProvider) String() string { return "fake" }
func (p *fakeProvider) Info() ProviderInfo {
	return ProviderInfo{ID: "fake", Code: IDCodeAnilist, Source: IDSourceAnilist, Name: "Fake", Version: "0.1.0"}
}
func (p *fakeProvider) SetLogger(l *logger.Logger) { p.logger = l }
func (p *fakeProvider) Logger() *logger.Logger     { return p.logger }

func (p *fakeProvider) SearchByID(_ context.Context, id int) (Metadata, bool, error) {
	p.mu.Lock()
	p.idCalls = append(p.idCalls, id)
	p.mu.Unlock()
	m, ok := p.byID[id]
	return m, ok, nil
}

func (p *fakeProvider) Search(_ context.Context, query string) ([]Metadata, error) {
	p.mu.Lock()
	p.queries = append(p.queries, query)
	p.mu.Unlock()
	if p.searchFn != nil {
		return p.searchFn(query)
	}
	return p.byQuery[query], nil
}

func (p *fakeProvider) SetMangaProgress(context.Context, int, int) error { return nil }
func (p *fakeProvider) Authenticated() bool                              { return false }
func (p *fakeProvider) User() User                                       { return nil }
func (p *fakeProvider) Login(context.Context, string) error              { return nil }
func (p *fakeProvider) Logout() error                                    { return nil }
