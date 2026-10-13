package libmangal

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/luevano/libmangal/mangadata"
	"github.com/luevano/libmangal/metadata"
	"github.com/luevano/libmangal/progress"
	"github.com/skratchdot/open-golang/open"
)

// openFile opens a file with the os default app, replaced in tests.
var openFile = open.Run

// SetMetadataProvider will add or update the given metadata Provider.
//
// The provider keeps its own logger (ProviderWithCache.SetLogger), it may be shared
// by several clients.
func (c *Client) SetMetadataProvider(provider *metadata.ProviderWithCache) error {
	if provider == nil {
		return errors.New("nil metadata provider")
	}

	id := provider.Info().Source
	if id < 1 {
		return fmt.Errorf("metadata provider %q: invalid source %d", provider.Info().ID, id)
	}

	if _, ok := c.meta[id]; !ok {
		c.metaOrder = append(c.metaOrder, id)
	}
	c.meta[id] = provider
	return nil
}

// GetMetadataProvider returns the requested metadata Provider for the given id.
func (c *Client) GetMetadataProvider(id metadata.IDSource) (*metadata.ProviderWithCache, error) {
	p, ok := c.meta[id]
	if !ok {
		return nil, fmt.Errorf("no metadata provider with source %d", id)
	}
	return p, nil
}

// SearchMetadata will search for metadata on the available metadata providers,
// in the order they were set, returning the first found.
//
// Tries to search manga metadata in the following order:
//
// 1. If the manga contains non-nil metadata, by its metadata ID if available.
//
// 2. If the manga Title field is binded to a metadata ID.
//
// 3. Find closest manga metadata (FindClosest) by using the manga Title field.
func (c *Client) SearchMetadata(manga mangadata.Manga) (metadata.Metadata, error) {
	return c.SearchMetadataContext(context.Background(), manga)
}

// SearchMetadataContext is SearchMetadata with a context.
func (c *Client) SearchMetadataContext(
	ctx context.Context,
	manga mangadata.Manga,
) (metadata.Metadata, error) {
	c.logger.Log("searching metadata for manga %q on all available providers", manga)

	if len(c.meta) == 0 {
		return nil, errors.New("no metadata providers")
	}

	// a failing provider shouldn't stop the others from being searched
	var errs []error
	// why the previous provider didn't do, for the status
	var previous string
	for _, id := range c.metaOrder {
		p := c.meta[id]
		name := p.Info().Name
		if previous == "" {
			progress.Report(ctx, progress.Status{Source: "metadata", Message: fmt.Sprintf("searching %s for %q", name, manga), Minor: true})
		} else {
			progress.Report(ctx, progress.Status{Source: "metadata", Message: fmt.Sprintf("%s, trying %s", previous, name)})
		}
		meta, found, err := c.SearchByMangaContext(ctx, p, manga)
		if err != nil {
			c.logger.Log("searching metadata for manga %q on %q: %s", manga, p, err)
			errs = append(errs, err)
			previous = name + " failed"
			continue
		}
		if !found {
			c.logger.Log("no metadata found for manga %q on %q", manga, p)
			previous = "no " + name + " match"
			continue
		}

		c.logger.Log("found metadata for manga %q on %q", manga, p)
		return meta, nil
	}

	return nil, errors.Join(errs...)
}

// SearchByManga is a convenience method to search given a Manga.
// It's meant to be used by the SearchMetadata method.
//
// It's MatchManga, but only good matches (see metadata.Match.Bad) count as found.
func (c *Client) SearchByManga(provider *metadata.ProviderWithCache, manga mangadata.Manga) (metadata.Metadata, bool, error) {
	return c.SearchByMangaContext(context.Background(), provider, manga)
}

// SearchByMangaContext is SearchByManga with a context.
func (c *Client) SearchByMangaContext(
	ctx context.Context,
	provider *metadata.ProviderWithCache,
	manga mangadata.Manga,
) (metadata.Metadata, bool, error) {
	match, found, err := c.MatchMangaContext(ctx, provider, manga)
	if err != nil || !found {
		return nil, false, err
	}
	if match.Bad() {
		c.logger.Log("closest metadata for manga %q is a bad match (%q, score %.2f), not using it", manga, match.Metadata, match.Score)
		return nil, false, nil
	}
	return match.Metadata, true, nil
}

// MatchManga finds the metadata that best matches the manga, even a bad match
// (metadata.Match.Bad) so it can be shown to the user, never used automatically.
//
// Tries to search manga metadata in the following order:
//
// 1. If the manga contains non-nil metadata, by its metadata IDs if available (a bound match).
//
// 2. If the manga title is bound to a metadata ID (a bound match).
//
// 3. The closest search result (metadata.ProviderWithCache.FindClosest) for the manga
// title, then the alternate titles of its metadata.
func (c *Client) MatchManga(provider *metadata.ProviderWithCache, manga mangadata.Manga) (metadata.Match, bool, error) {
	return c.MatchMangaContext(context.Background(), provider, manga)
}

// MatchMangaContext is MatchManga with a context.
func (c *Client) MatchMangaContext(
	ctx context.Context,
	provider *metadata.ProviderWithCache,
	manga mangadata.Manga,
) (metadata.Match, bool, error) {
	matches, found, err := c.MatchMangasContext(ctx, provider, []mangadata.Manga{manga})
	if err != nil {
		return metadata.Match{}, false, err
	}
	return matches[0], found[0], nil
}

// MatchMangas is MatchManga for several mangas, with as few requests as possible
// (see metadata.ProviderWithCache.FindClosestMany).
func (c *Client) MatchMangas(provider *metadata.ProviderWithCache, mangas []mangadata.Manga) ([]metadata.Match, []bool, error) {
	return c.MatchMangasContext(context.Background(), provider, mangas)
}

// MatchMangasContext is MatchMangas with a context.
func (c *Client) MatchMangasContext(
	ctx context.Context,
	provider *metadata.ProviderWithCache,
	mangas []mangadata.Manga,
) ([]metadata.Match, []bool, error) {
	c.logger.Log("searching metadata for %d mangas on %q", len(mangas), provider)
	matches := make([]metadata.Match, len(mangas))
	found := make([]bool, len(mangas))

	source := provider.Info().Source
	queries := make([]metadata.Query, len(mangas))
	idsOf := map[int][]int{}
	var allIDs []int
	for i, manga := range mangas {
		queries[i] = metadata.Query{Titles: []string{manga.Info().Title}}
		meta := manga.Metadata()
		if meta == nil {
			continue
		}
		for _, id := range append([]metadata.ID{meta.ID()}, meta.ExtraIDs()...) {
			if id.Source == source && id.Value() != 0 {
				idsOf[i] = append(idsOf[i], id.Value())
				allIDs = append(allIDs, id.Value())
			}
		}
		queries[i].Titles = append(append(queries[i].Titles, meta.Title()), meta.AlternateTitles()...)
		queries[i].Year = meta.StartDate().Year
	}

	// ids given by the provider (or a previous binding) win
	if len(allIDs) != 0 {
		metas, err := provider.SearchByIDsContext(ctx, allIDs)
		if err != nil {
			return nil, nil, err
		}
		for i, ids := range idsOf {
			for _, id := range ids {
				if meta, ok := metas[id]; ok {
					matches[i], found[i] = metadata.Match{Metadata: meta, Score: 1, Bound: true}, true
					break
				}
			}
		}
	}

	var pending []int
	var pendingQueries []metadata.Query
	for i, query := range queries {
		if found[i] || !hasTitle(query) {
			continue
		}
		pending = append(pending, i)
		pendingQueries = append(pendingQueries, query)
	}
	if len(pending) == 0 {
		return matches, found, nil
	}
	closest, closestFound, err := provider.FindClosestManyContext(ctx, pendingQueries)
	if err != nil {
		return nil, nil, err
	}
	for j, i := range pending {
		matches[i], found[i] = closest[j], closestFound[j]
	}
	return matches, found, nil
}

func hasTitle(query metadata.Query) bool {
	for _, t := range query.Titles {
		if strings.TrimSpace(t) != "" {
			return true
		}
	}
	return false
}

// ReadChapter opens the chapter for reading and marks it as read if authorized.
// It will use os default app for resulting mimetype.
//
// E.g. `xdg-open` for Linux.
//
// It will also set the reading progress on Anilist, MyAnimeList and Kitsu
// if enabled in the ReadOptions.
//
// Note, that underlying filesystem must be mapped with OsFs
// in order for os to open it. Failing to open it is an *OpenError.
func (c *Client) ReadChapter(path string, chapter mangadata.Chapter, options ReadOptions) error {
	return c.ReadChapterContext(context.Background(), path, chapter, options)
}

// ReadChapterContext is ReadChapter with a context.
func (c *Client) ReadChapterContext(
	ctx context.Context,
	path string,
	chapter mangadata.Chapter,
	options ReadOptions,
) error {
	c.logger.Log("opening chapter %q from %s with the default app", chapter, path)

	if err := openFile(path); err != nil {
		return &OpenError{Path: path, Err: err}
	}

	sync := map[metadata.IDSource]bool{
		metadata.IDSourceAnilist:     options.SaveAnilist,
		metadata.IDSourceMyAnimeList: options.SaveMyAnimeList,
		metadata.IDSourceKitsu:       options.SaveKitsu,
	}
	progress := int(math.Trunc(float64(chapter.Info().Number)))
	manga := chapter.Volume().Manga()

	var errs []error
	for _, id := range c.metaOrder {
		if !sync[id] {
			continue
		}
		delete(sync, id)
		p := c.meta[id]

		meta, found, err := c.SearchByMangaContext(ctx, p, manga)
		if err == nil && !found {
			err = fmt.Errorf("no metadata found for manga %q", manga)
		}
		if err == nil {
			err = p.SetMangaProgressContext(ctx, meta.ID().Value(), progress)
		}
		if err != nil {
			c.logger.Log("setting manga progress on %q: %s", p, err)
			errs = append(errs, fmt.Errorf("set progress on %s: %w", p, err))
		}
	}
	for id, enabled := range sync {
		if enabled {
			errs = append(errs, fmt.Errorf("set progress: no metadata provider with source %d", id))
		}
	}
	return errors.Join(errs...)
}
