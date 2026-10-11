package libmangal

import (
	"context"
	"errors"
	"testing"

	"github.com/luevano/libmangal/mangadata"
)

type byIDProvider struct {
	*fakeProvider
	mangas map[string]*fakeManga
	err    error
}

func (p byIDProvider) MangaByID(_ context.Context, id string) (mangadata.Manga, bool, error) {
	if p.err != nil {
		return nil, false, p.err
	}
	m, ok := p.mangas[id]
	if !ok {
		return nil, false, nil
	}
	return m, true, nil
}

func TestMangaByID(t *testing.T) {
	want := &fakeManga{info: mangadata.MangaInfo{ID: "42", Title: "Answer"}}
	provider := byIDProvider{fakeProvider: &fakeProvider{}, mangas: map[string]*fakeManga{"42": want}}
	c, err := NewClientContext(context.Background(), fakeLoader{provider}, DefaultClientOptions())
	if err != nil {
		t.Fatal(err)
	}

	got, found, err := c.MangaByID("42")
	if err != nil || !found || got != want {
		t.Fatalf("MangaByID(42) = %v, %v, %v; want %v, true, nil", got, found, err, want)
	}

	got, found, err = c.MangaByID("missing")
	if err != nil || found || got != nil {
		t.Fatalf("MangaByID(missing) = %v, %v, %v; want nil, false, nil", got, found, err)
	}
}

func TestMangaByIDError(t *testing.T) {
	errSite := errors.New("site down")
	provider := byIDProvider{fakeProvider: &fakeProvider{}, err: errSite}
	c, err := NewClientContext(context.Background(), fakeLoader{provider}, DefaultClientOptions())
	if err != nil {
		t.Fatal(err)
	}
	if _, found, err := c.MangaByID("42"); !errors.Is(err, errSite) || found {
		t.Fatalf("MangaByID = %v, %v; want %v", found, err, errSite)
	}
}

func TestMangaByIDUnsupported(t *testing.T) {
	c, err := NewClientContext(context.Background(), fakeLoader{&fakeProvider{}}, DefaultClientOptions())
	if err != nil {
		t.Fatal(err)
	}
	if _, found, err := c.MangaByID("42"); !errors.Is(err, ErrMangaByIDUnsupported) || found {
		t.Fatalf("MangaByID = %v, %v; want ErrMangaByIDUnsupported", found, err)
	}
}
