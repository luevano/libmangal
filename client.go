package libmangal

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/luevano/libmangal/logger"
	"github.com/luevano/libmangal/mangadata"
	"github.com/luevano/libmangal/metadata"
	"github.com/spf13/afero"
)

// Client is a wrapper around Provider with extended functionality.
//
// It's the core of libmangal.
type Client struct {
	provider Provider
	meta     map[metadata.IDSource]*metadata.ProviderWithCache
	// metaOrder is the order metadata providers were set in, their search priority
	metaOrder []metadata.IDSource
	options   ClientOptions
	logger    *logger.Logger
	mangaDirs mangaDirIndex
}

// NewClient creates a new client from given ProviderLoader.
//
// ClientOptions must be non-zero. Use DefaultClientOptions for defaults.
// It will validate ProviderLoader.Info and load the provider.
func NewClient(loader ProviderLoader, options ClientOptions) (*Client, error) {
	return NewClientContext(context.Background(), loader, options)
}

// NewClientContext is NewClient with a context.
func NewClientContext(
	ctx context.Context,
	loader ProviderLoader,
	options ClientOptions,
) (*Client, error) {
	if err := options.validate(); err != nil {
		return nil, fmt.Errorf("invalid client options: %w", err)
	}
	providerInfo := loader.Info()

	if err := providerInfo.Validate(); err != nil {
		return nil, fmt.Errorf("provider %q: invalid info: %w", providerInfo.ID, err)
	}

	provider, err := loader.Load(ctx)
	if err != nil {
		return nil, err
	}

	logger := logger.NewLogger()
	provider.SetLogger(logger)

	return &Client{
		provider: provider,
		meta:     map[metadata.IDSource]*metadata.ProviderWithCache{},
		options:  options,
		logger:   logger,
	}, nil
}

func (c *Client) Close() error {
	return c.provider.Close()
}

func (c *Client) String() string {
	return c.provider.Info().Name
}

// Info returns info about provider.
func (c *Client) Info() ProviderInfo {
	return c.provider.Info()
}

// Logger returns the client's Logger.
func (c *Client) Logger() *logger.Logger {
	return c.logger
}

// FS returns the client's FileSystem.
func (c *Client) FS() afero.Fs {
	return c.options.FS
}

// SearchMangas searches for mangas with the given query.
func (c *Client) SearchMangas(query string) ([]mangadata.Manga, error) {
	return c.SearchMangasContext(context.Background(), query)
}

// SearchMangasContext is SearchMangas with a context.
func (c *Client) SearchMangasContext(ctx context.Context, query string) ([]mangadata.Manga, error) {
	return c.provider.SearchMangas(ctx, query)
}

// MangaByID gets the manga with the given id (mangadata.MangaInfo.ID), the same manga
// a search would return.
//
// found is false, with a nil error, when there is no such manga. If the provider doesn't
// implement MangaByIDProvider the error matches ErrMangaByIDUnsupported.
func (c *Client) MangaByID(id string) (mangadata.Manga, bool, error) {
	return c.MangaByIDContext(context.Background(), id)
}

// MangaByIDContext is MangaByID with a context.
func (c *Client) MangaByIDContext(ctx context.Context, id string) (mangadata.Manga, bool, error) {
	p, ok := c.provider.(MangaByIDProvider)
	if !ok {
		return nil, false, fmt.Errorf("provider %q: %w", c.provider.Info().ID, ErrMangaByIDUnsupported)
	}
	manga, found, err := p.MangaByID(ctx, id)
	if err != nil {
		return nil, false, fmt.Errorf("get manga %q: %w", id, err)
	}
	if !found || manga == nil {
		return nil, false, nil
	}
	return manga, true, nil
}

// MangaVolumes gets volumes of the given manga.
func (c *Client) MangaVolumes(manga mangadata.Manga) ([]mangadata.Volume, error) {
	return c.MangaVolumesContext(context.Background(), manga)
}

// MangaVolumesContext is MangaVolumes with a context.
func (c *Client) MangaVolumesContext(ctx context.Context, manga mangadata.Manga) ([]mangadata.Volume, error) {
	return c.provider.MangaVolumes(ctx, manga)
}

// VolumeChapters gets chapters of the given volume.
func (c *Client) VolumeChapters(volume mangadata.Volume) ([]mangadata.Chapter, error) {
	return c.VolumeChaptersContext(context.Background(), volume)
}

// VolumeChaptersContext is VolumeChapters with a context.
func (c *Client) VolumeChaptersContext(ctx context.Context, volume mangadata.Volume) ([]mangadata.Chapter, error) {
	return c.provider.VolumeChapters(ctx, volume)
}

// ChapterPages gets pages of the given chapter.
func (c *Client) ChapterPages(chapter mangadata.Chapter) ([]mangadata.Page, error) {
	return c.ChapterPagesContext(context.Background(), chapter)
}

// ChapterPagesContext is ChapterPages with a context. A mangadata.WrappedChapter's pages
// are its provider's chapter's.
func (c *Client) ChapterPagesContext(ctx context.Context, chapter mangadata.Chapter) ([]mangadata.Page, error) {
	for {
		w, ok := chapter.(mangadata.WrappedChapter)
		if !ok {
			break
		}
		chapter = w.Unwrap()
	}
	return c.provider.ChapterPages(ctx, chapter)
}

// ProviderName determines the provider directory name.
//
// Names from ClientOptions that aren't a single path element (empty, ".", "..",
// or with a path separator) are sanitized with SanitizeName, for all the name methods.
func (c *Client) ProviderName(provider ProviderInfo) string {
	return c.safeName("provider", c.options.ProviderName(provider))
}

// MangaName determines the manga directory name.
func (c *Client) MangaName(manga mangadata.Manga) string {
	return c.safeName("manga", c.options.MangaName(c.Info(), manga))
}

// VolumeName determines the volume directory name.
// E.g. "Vol. 1" or "Volume 1"
func (c *Client) VolumeName(volume mangadata.Volume) string {
	return c.safeName("volume", c.options.VolumeName(c.Info(), volume))
}

// ChapterName determines the chapter file name.
// E.g. "[001] chapter 1" or "Chainsaw Man - Ch. 1"
func (c *Client) ChapterName(chapter mangadata.Chapter, format Format) string {
	return c.safeName("chapter", c.options.ChapterName(c.Info(), chapter)) + format.Extension()
}

// safeName keeps name from escaping the directory it's joined to.
func (c *Client) safeName(what, name string) string {
	switch {
	case name == "", name == ".", name == "..", strings.ContainsAny(name, "/"+string(filepath.Separator)):
		safe := SanitizeNameOS(name)
		c.logger.Log("%s name %q isn't a valid file name, using %q", what, name, safe)
		return safe
	}
	return name
}
