package libmangal

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/luevano/libmangal/mangadata"
	"github.com/luevano/libmangal/metadata"
	"github.com/spf13/afero"
	"golang.org/x/sync/errgroup"
)

// DownloadChapter downloads and writes chapter to disk with the given DownloadOptions.
//
// It will return resulting chapter download information via metadata.DownloadedChapter.
func (c *Client) DownloadChapter(chapter mangadata.Chapter, options DownloadOptions) (*metadata.DownloadedChapter, error) {
	return c.DownloadChapterContext(context.Background(), chapter, options)
}

// DownloadChapterContext is DownloadChapter with a context.
func (c *Client) DownloadChapterContext(
	ctx context.Context,
	chapter mangadata.Chapter,
	options DownloadOptions,
) (*metadata.DownloadedChapter, error) {
	if err := options.validate(); err != nil {
		return nil, fmt.Errorf("invalid download options: %w", err)
	}
	if reason := chapter.Info().Unavailable; reason != "" {
		return nil, fmt.Errorf("%w: %s", ErrUnavailable, reason)
	}
	c.logger.Log("downloading chapter %q as %s", chapter, options.Format)

	manga := chapter.Volume().Manga()
	// Found metadata will be replacing the incoming one,
	// even when no metadata is found (nil)
	if options.SearchMetadata {
		options.progress(DownloadProgress{Chapter: chapter, Stage: DownloadStageMetadata})
		m, err := c.SearchMetadataContext(ctx, manga)
		if err != nil {
			if options.Strict || ctx.Err() != nil {
				return nil, fmt.Errorf("search metadata: %w", err)
			}
			c.logger.Log("search metadata for manga %q: %s, downloading without it", manga, err)
		}
		manga.SetMetadata(m)
	}
	// Even after a metadata search, check if it is valid (nil for example)
	if err := metadata.Validate(manga.Metadata()); err != nil && options.Strict {
		return nil, fmt.Errorf("manga %q: %w", manga, err)
	}

	// a temp client is used to download everything
	// into temp memory, then it is moved into the actual
	// location provided to the client
	tmpClient := Client{
		provider: c.provider,
		options:  c.options,
		logger:   c.logger,
	}
	tmpClient.options.FS = afero.NewMemMapFs()

	dirs, err := c.chapterDirs(ctx, chapter, options)
	if err != nil {
		return nil, err
	}
	downChap, err := tmpClient.downloadChapterWithMetadata(ctx, chapter, options, dirs, func(path string) (bool, error) {
		return afero.Exists(c.options.FS, path)
	})
	if err != nil {
		return nil, err
	}

	// a re-downloaded images-format chapter replaces the old pages instead of merging with them
	var replace func(string) bool
	if options.Format == FormatImages && downChap.ChapterStatus == metadata.DownloadStatusOverwritten {
		chapterPath := filepath.Join(downChap.Directory, downChap.Filename)
		replace = func(path string) bool { return path == chapterPath }
	}
	if err := mergeDirectories(
		c.options.ModeDir, c.options.ModeFile,
		c.FS(), options.Directory,
		tmpClient.FS(), options.Directory,
		replace,
	); err != nil {
		return nil, fmt.Errorf("move to %s: %w", options.Directory, err)
	}
	if dirs.manga != "" {
		if err := c.rememberMangaDir(ctx, dirs.parent, filepath.Base(dirs.manga), NewMangaDirInfo(c.Info(), manga)); err != nil {
			return nil, fmt.Errorf("remember the directory of manga %q: %w", manga, err)
		}
	}

	return downChap, nil
}

// downloadChapterWithMetadata prepares the chapter and its metadata
// to be downloaded, as well as downloading metadata such as
// the series.json file and cover/banner images, if any.
func (c *Client) downloadChapterWithMetadata(
	ctx context.Context,
	chapter mangadata.Chapter,
	options DownloadOptions,
	dirs chapterDirs,
	existsFunc func(string) (bool, error),
) (*metadata.DownloadedChapter, error) {
	directory := dirs.chapter
	seriesJSONDir, coverDir, bannerDir := dirs.meta, dirs.meta, dirs.meta

	err := c.options.FS.MkdirAll(directory, c.options.ModeDir)
	if err != nil {
		return nil, err
	}

	manga := chapter.Volume().Manga()
	if dirs.manga != "" {
		if err := WriteMangaDirInfo(c.options.FS, dirs.manga, NewMangaDirInfo(c.Info(), manga), c.options.ModeFile); err != nil {
			return nil, err
		}
	}

	chapterFilename := c.ChapterName(chapter, options.Format)
	chapterPath := filepath.Join(directory, chapterFilename)

	chapterExists, err := existsFunc(chapterPath)
	if err != nil {
		return nil, err
	}
	skipped := "already downloaded"

	// the same chapter may be there under another name (e.g. from another provider)
	if options.SkipIfExists && !chapterExists && c.options.ChapterIndex != nil {
		mangaDir := dirs.manga
		if mangaDir == "" {
			mangaDir = dirs.chapter
		}
		existing, provider, found, err := findChapterIn(ctx, c.options.ChapterIndex, mangaDir, chapter, options.Format)
		if err != nil {
			return nil, fmt.Errorf("chapter index: %w", err)
		}
		if found {
			if chapterExists, err = existsFunc(existing); err != nil {
				return nil, err
			}
		}
		if chapterExists {
			skipped = fmt.Sprintf("already downloaded from %s as %q", provider, filepath.Base(existing))
			directory, chapterFilename = filepath.Dir(existing), filepath.Base(existing)
		}
	}

	// Data about downloaded chapter
	downChap := &metadata.DownloadedChapter{
		Number:             chapter.Info().Number,
		Title:              chapter.Info().Title,
		Filename:           chapterFilename,
		Directory:          directory,
		ChapterStatus:      metadata.DownloadStatusExists,
		SeriesJSONStatus:   metadata.DownloadStatusSkip,
		ComicInfoXMLStatus: metadata.DownloadStatusSkip, // only CBZ writes it
		CoverStatus:        metadata.DownloadStatusSkip,
		BannerStatus:       metadata.DownloadStatusSkip,
	}

	if chapterExists && options.SkipIfExists {
		downChap.Skipped = skipped
	}

	if !chapterExists || !options.SkipIfExists {
		ciXmlStatus, pages, err := c.downloadChapter(ctx, chapter, chapterPath, options)
		if err != nil {
			return nil, err
		}
		downChap.ComicInfoXMLStatus = ciXmlStatus
		downChap.Pages = pages

		downChap.ChapterStatus = metadata.DownloadStatusNew
		if chapterExists {
			downChap.ChapterStatus = metadata.DownloadStatusOverwritten
		}
	}

	if metadata.Validate(manga.Metadata()) != nil {
		downChap.SeriesJSONStatus = metadata.DownloadStatusMissingMetadata
		downChap.CoverStatus = metadata.DownloadStatusMissingMetadata
		downChap.BannerStatus = metadata.DownloadStatusMissingMetadata
		return downChap, nil
	}

	skip := options.SkipSeriesJSONIfOngoing && manga.Metadata().Status() == metadata.StatusReleasing
	if options.WriteSeriesJSON && !skip {
		path := filepath.Join(seriesJSONDir, metadata.FilenameSeriesJSON)
		downChap.SeriesJSONStatus, err = c.writeMetadataFile(path, existsFunc, options.Strict, func() ([]byte, error) {
			var buf bytes.Buffer
			err := c.writeSeriesJSON(manga, &buf)
			return buf.Bytes(), err
		})
		if err != nil {
			return nil, err
		}
	}

	if options.DownloadMangaCover {
		path := filepath.Join(coverDir, metadata.FilenameCoverJPG)
		downChap.CoverStatus, err = c.writeMetadataFile(path, existsFunc, options.Strict, func() ([]byte, error) {
			var buf bytes.Buffer
			err := c.downloadMangaImage(ctx, manga, mangaImageCover, &buf)
			return buf.Bytes(), err
		})
		if err != nil {
			return nil, err
		}
	}

	if options.DownloadMangaBanner {
		path := filepath.Join(bannerDir, metadata.FilenameBannerJPG)
		downChap.BannerStatus, err = c.writeMetadataFile(path, existsFunc, options.Strict, func() ([]byte, error) {
			var buf bytes.Buffer
			err := c.downloadMangaImage(ctx, manga, mangaImageBanner, &buf)
			return buf.Bytes(), err
		})
		if err != nil {
			return nil, err
		}
	}

	return downChap, nil
}

// writeMetadataFile writes path with the content only once it's ready, so a failure
// doesn't leave an empty file that later counts as already downloaded.
//
// Failures are only returned when strict, otherwise they're logged and reported as failed.
func (c *Client) writeMetadataFile(
	path string,
	existsFunc func(string) (bool, error),
	strict bool,
	content func() ([]byte, error),
) (metadata.DownloadStatus, error) {
	exists, err := existsFunc(path)
	if err != nil {
		return "", err
	}
	if exists {
		return metadata.DownloadStatusExists, nil
	}

	data, err := content()
	if err != nil {
		if strict {
			return "", fmt.Errorf("%s: %w", filepath.Base(path), err)
		}
		c.logger.Log("writing %s: %s", filepath.Base(path), err)
		return metadata.DownloadStatusFailed, nil
	}
	if err := afero.WriteFile(c.options.FS, path, data, c.options.ModeFile); err != nil {
		return "", err
	}
	return metadata.DownloadStatusNew, nil
}

// downloadChapter is a wrapper of DownloadPagesInBatch which wraps the
// pages in the desired format to write to disk.
func (c *Client) downloadChapter(
	ctx context.Context,
	chapter mangadata.Chapter,
	path string,
	options DownloadOptions,
) (metadata.DownloadStatus, int, error) {
	options.progress(DownloadProgress{Chapter: chapter, Stage: DownloadStagePages})
	pages, err := c.ChapterPagesContext(ctx, chapter)
	if err != nil {
		return "", 0, err
	}
	if err := checkPageNames(pages); err != nil {
		return "", 0, err
	}

	total := len(pages)
	images := func(done int) {
		options.progress(DownloadProgress{Chapter: chapter, Stage: DownloadStageImages, Done: done, Total: total})
	}
	if total > 0 {
		images(0)
	}
	downloadedPages, err := c.downloadPages(ctx, pages, images)
	if err != nil {
		return "", 0, err
	}
	options.progress(DownloadProgress{Chapter: chapter, Stage: DownloadStageSaving, Done: total, Total: total})

	if options.ImageTransformer != nil {
		for i, page := range downloadedPages {
			if err := ctx.Err(); err != nil {
				return "", 0, err
			}
			image, err := options.ImageTransformer(page.Image())
			if err != nil {
				return "", 0, fmt.Errorf("transform page %d: %w", i+1, err)
			}
			page.SetImage(image)
		}
	}

	if options.Format == FormatImages {
		if err := c.options.FS.MkdirAll(path, c.options.ModeDir); err != nil {
			return "", 0, err
		}
		for i, page := range downloadedPages {
			name, err := pageName(i, page)
			if err != nil {
				return "", 0, err
			}
			if err := afero.WriteFile(c.options.FS, filepath.Join(path, name), page.Image(), c.options.ModeFile); err != nil {
				return "", 0, err
			}
		}
		return metadata.DownloadStatusSkip, len(downloadedPages), nil
	}

	file, err := c.options.FS.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, c.options.ModeFile)
	if err != nil {
		return "", 0, err
	}
	ciXMLStatus, err := c.writeChapter(ctx, chapter, downloadedPages, file, options)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return "", 0, errors.Join(err, c.options.FS.Remove(path))
	}
	return ciXMLStatus, len(downloadedPages), nil
}

// writeChapter writes pages to out in options.Format (not FormatImages).
func (c *Client) writeChapter(
	ctx context.Context,
	chapter mangadata.Chapter,
	downloadedPages []mangadata.PageWithImage,
	out io.Writer,
	options DownloadOptions,
) (metadata.DownloadStatus, error) {
	// Only CBZ writes the ComicInfo.xml, so by default it's skipped
	ciXMLStatus := metadata.DownloadStatusSkip
	var err error
	switch options.Format {
	case FormatPDF:
		err = c.savePDF(ctx, downloadedPages, out)
	case FormatTAR:
		err = c.saveTAR(downloadedPages, out)
	case FormatTARGZ:
		err = c.saveTARGZ(downloadedPages, out)
	case FormatZIP:
		err = c.saveZIP(downloadedPages, out)
	case FormatCBZ:
		var comicInfoXML *metadata.ComicInfoXML
		ciXMLFailed := false
		if options.WriteComicInfoXML && metadata.Validate(chapter.Volume().Manga().Metadata()) == nil {
			mangaChapter := chapter.Info()
			metaChapter := metadata.Chapter{
				Title:           mangaChapter.Title,
				URL:             mangaChapter.URL,
				Number:          mangaChapter.Number,
				Date:            mangaChapter.Date,
				ScanlationGroup: mangaChapter.ScanlationGroup,
				Pages:           len(downloadedPages),
				Volume:          chapter.Volume().Info().Number,
			}
			ciXML, err := c.getComicInfoXML(chapter, metaChapter)
			switch {
			case err == nil:
				comicInfoXML = &ciXML
			case options.Strict:
				return "", fmt.Errorf("%s: %w", metadata.FilenameComicInfoXML, err)
			default:
				c.logger.Log("getting %s: %s", metadata.FilenameComicInfoXML, err)
				ciXMLFailed = true
			}
		}

		ciXMLStatus, err = c.saveCBZ(downloadedPages, out, comicInfoXML, options.ComicInfoXMLOptions)
		if ciXMLFailed {
			ciXMLStatus = metadata.DownloadStatusFailed
		}
	default:
		err = fmt.Errorf("unknown format %d", options.Format)
	}
	if err != nil {
		return "", fmt.Errorf("write %s: %w", options.Format, err)
	}
	return ciXMLStatus, nil
}

// DownloadPagesInBatch downloads multiple pages in batch
// by calling DownloadPage for each page, ClientOptions.ConcurrentPages at a time.
//
// If any of the pages fails to download it will stop downloading other pages
// and return error immediately.
func (c *Client) DownloadPagesInBatch(pages []mangadata.Page) ([]mangadata.PageWithImage, error) {
	return c.DownloadPagesInBatchContext(context.Background(), pages)
}

// DownloadPagesInBatchContext is DownloadPagesInBatch with a context.
func (c *Client) DownloadPagesInBatchContext(
	ctx context.Context,
	pages []mangadata.Page,
) ([]mangadata.PageWithImage, error) {
	return c.downloadPages(ctx, pages, nil)
}

// downloadPages is DownloadPagesInBatchContext, onDone (if not nil) gets the count of
// downloaded pages after each one, one call at a time.
func (c *Client) downloadPages(
	ctx context.Context,
	pages []mangadata.Page,
	onDone func(done int),
) ([]mangadata.PageWithImage, error) {
	if len(pages) == 0 {
		return nil, errors.New("no pages")
	}
	c.logger.Log("downloading %d pages", len(pages))

	g, ctx := errgroup.WithContext(ctx)
	if c.options.ConcurrentPages > 0 {
		g.SetLimit(c.options.ConcurrentPages)
	}
	downloadedPages := make([]mangadata.PageWithImage, len(pages))
	var mu sync.Mutex
	done := 0
	for i, page := range pages {
		g.Go(func() error {
			c.logger.Log("page #%03d: downloading", i+1)

			downloaded, err := c.DownloadPageContext(ctx, page)
			if err != nil {
				return fmt.Errorf("page %d: %w", i+1, err)
			}

			c.logger.Log("page #%03d: done", i+1)

			downloadedPages[i] = downloaded
			if onDone != nil {
				mu.Lock()
				done++
				onDone(done)
				mu.Unlock()
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return downloadedPages, nil
}

// DownloadPage downloads a page contents (image).
func (c *Client) DownloadPage(page mangadata.Page) (mangadata.PageWithImage, error) {
	return c.DownloadPageContext(context.Background(), page)
}

// DownloadPageContext is DownloadPage with a context.
func (c *Client) DownloadPageContext(
	ctx context.Context,
	page mangadata.Page,
) (mangadata.PageWithImage, error) {
	if withImage, ok := page.(mangadata.PageWithImage); ok {
		return withImage, nil
	}

	image, err := c.provider.GetPageImage(ctx, page)
	if err != nil {
		return nil, err
	}

	return &pageWithImage{
		Page:  page,
		image: image,
	}, nil
}
