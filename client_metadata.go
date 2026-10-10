package libmangal

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/luevano/libmangal/mangadata"
	"github.com/luevano/libmangal/metadata"
	"github.com/skratchdot/open-golang/open"
)

// SetMetadataProvider will add or update the given metadata Provider.
func (c *Client) SetMetadataProvider(provider *metadata.ProviderWithCache) error {
	if provider == nil {
		return errors.New("Provider must be non-nil")
	}

	id := provider.Info().Source
	if id < 1 {
		return errors.New(fmt.Sprintf("invalid metadata Provider IDSource %d (ID %q)", id, provider.Info().ID))
	}

	provider.SetLogger(c.logger)
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
		return nil, errors.New(fmt.Sprintf("no metadata Provider found with IDSource %d", id))
	}
	return p, nil
}

// SearchMetadata will search for metadata on the available metadata providers.
//
// Tries to search manga metadata in the following order:
//
// 1. If the manga contains non-nil metadata, by its metadata ID if available.
//
// 2. If the manga Title field is binded to a metadata ID.
//
// 3. Find closest manga metadata (FindClosest) by using the manga Title field.
func (c *Client) SearchMetadata(
	ctx context.Context,
	manga mangadata.Manga,
) (metadata.Metadata, error) {
	c.logger.Log("searching metadata for manga %q on all available providers", manga)

	if len(c.meta) == 0 {
		return nil, errors.New("no metadata Providers available")
	}

	// a failing provider shouldn't stop the others from being searched
	var errs []error
	for _, id := range c.metaOrder {
		p := c.meta[id]
		meta, found, err := c.SearchByManga(ctx, p, manga)
		if err != nil {
			c.logger.Log("searching metadata for manga %q on %q: %s", manga, p, err)
			errs = append(errs, err)
			continue
		}
		if !found {
			c.logger.Log("no metadata found for manga %q on %q", manga, p)
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
// Tries to search manga metadata in the following order:
//
// 1. If the manga contains non-nil metadata, by its metadata ID if available.
//
// 2. If the manga title is binded to a metadata ID.
//
// 3. Find closest manga metadata (FindClosest) by using the manga Title field.
func (c *Client) SearchByManga(
	ctx context.Context,
	provider *metadata.ProviderWithCache,
	manga mangadata.Manga,
) (metadata.Metadata, bool, error) {
	c.logger.Log("searching metadata by (libmangal) manga on %q", c.Info().Name)

	// Try to search by metadata ID if it is available
	if meta := manga.Metadata(); meta != nil {
		source := provider.Info().Source
		for _, id := range append([]metadata.ID{meta.ID()}, meta.ExtraIDs()...) {
			if id.Source != source || id.Value() == 0 {
				continue
			}
			found, ok, err := provider.SearchByID(ctx, id.Value())
			if err != nil {
				return nil, false, err
			}
			if ok {
				return found, true, nil
			}
		}
	}

	// Else try to search by the title, this doesn't ensure
	// that the found manga metadata is 100% corresponding to
	// the manga requested, there are some instances in which
	// the result will be wrong
	title := manga.Info().Title
	return provider.FindClosest(ctx, title, 3, 3)
}

// ReadChapter opens the chapter for reading and marks it as read if authorized.
// It will use os default app for resulting mimetype.
//
// E.g. `xdg-open` for Linux.
//
// It will also sync read chapter with your Anilist profile
// if it's configured.
//
// Note, that underlying filesystem must be mapped with OsFs
// in order for os to open it.
func (c *Client) ReadChapter(
	ctx context.Context,
	path string,
	chapter mangadata.Chapter,
	options ReadOptions,
) error {
	c.logger.Log("opening chapter %q from %s with the default app", chapter, path)

	err := open.Run(path)
	if err != nil {
		return err
	}

	if !options.SaveAnilist || !options.SaveHistory {
		return nil
	}

	if len(c.meta) == 0 {
		return errors.New("no metadata Providers available to mark chapter as read")
	}

	var setProgressErrors []error
	progress := int(math.Trunc(float64(chapter.Info().Number)))
	mangaTitle := chapter.Volume().Manga().Info().Title
	for id, p := range c.meta {
		metaID := 0
		// TODO: find a better way to get the metadata for the current provider
		meta, found, err := p.FindClosest(ctx, mangaTitle, 3, 3)
		if err != nil {
			goto addError
		}
		if !found {
			err = errors.New("no manga metadata found with Provider ID " + string(id))
			goto addError
		}
		metaID = meta.ID().Value()

		// TODO: better handle this mess
		if (id == metadata.IDSourceAnilist && options.SaveAnilist) ||
			(id == metadata.IDSourceMyAnimeList && options.SaveMyAnimeList) {
			err = p.SetMangaProgress(ctx, metaID, progress)
		}
		if err != nil {
			goto addError
		}

		continue
	addError:
		c.logger.Log("error while setting manga progress for Provider ID %q: %s", string(id), err.Error())
		setProgressErrors = append(setProgressErrors, err)
	}

	// TODO: save to local history

	if len(setProgressErrors) != 0 {
		return errors.Join(setProgressErrors...)
	}
	return nil
}
