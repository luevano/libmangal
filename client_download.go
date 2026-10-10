package libmangal

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"

	"github.com/luevano/libmangal/mangadata"
	"github.com/luevano/libmangal/metadata"
	"github.com/spf13/afero"
	"golang.org/x/sync/errgroup"
)

// DownloadChapter downloads and writes chapter to disk with the given DownloadOptions.
//
// It will return resulting chapter download information via metadata.DownloadedChapter.
func (c *Client) DownloadChapter(
	ctx context.Context,
	chapter mangadata.Chapter,
	options DownloadOptions,
) (*metadata.DownloadedChapter, error) {
	if reason := chapter.Info().Unavailable; reason != "" {
		return nil, fmt.Errorf("unavailable: %s", reason)
	}
	c.logger.Log("downloading chapter %q as %s", chapter, options.Format)

	manga := chapter.Volume().Manga()
	// Found metadata will be replacing the incoming one,
	// even when no metadata is found (nil)
	if options.SearchMetadata {
		m, err := c.SearchMetadata(ctx, manga)
		if err != nil {
			return nil, err
		}
		manga.SetMetadata(m)
	}
	// Even after a metadata search, check if it is valid (nil for example)
	if err := metadata.Validate(manga.Metadata()); err != nil && options.Strict {
		return nil, fmt.Errorf("no valid metadata for manga %q: %s", manga, err.Error())
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

	downChap, err := tmpClient.downloadChapterWithMetadata(ctx, chapter, options, func(path string) (bool, error) {
		return afero.Exists(c.options.FS, path)
	})
	if err != nil {
		return nil, err
	}

	if err := mergeDirectories(
		c.options.ModeDir,
		c.FS(), options.Directory,
		tmpClient.FS(), options.Directory,
	); err != nil {
		return nil, err
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
	existsFunc func(string) (bool, error),
) (*metadata.DownloadedChapter, error) {
	directory := options.Directory

	var (
		seriesJSONDir = directory
		coverDir      = directory
		bannerDir     = directory
	)

	if options.CreateProviderDir {
		directory = filepath.Join(directory, c.ProviderName(c.provider.Info()))
	}

	if options.CreateMangaDir {
		directory = filepath.Join(directory, c.MangaName(chapter.Volume().Manga()))
		seriesJSONDir = directory
		coverDir = directory
		bannerDir = directory
	}

	if options.CreateVolumeDir {
		directory = filepath.Join(directory, c.VolumeName(chapter.Volume()))
	}

	err := c.options.FS.MkdirAll(directory, c.options.ModeDir)
	if err != nil {
		return nil, err
	}

	chapterFilename := c.ChapterName(chapter, options.Format)
	chapterPath := filepath.Join(directory, chapterFilename)

	chapterExists, err := existsFunc(chapterPath)
	if err != nil {
		return nil, err
	}

	manga := chapter.Volume().Manga()
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

	if !chapterExists || !options.SkipIfExists {
		ciXmlStatus, err := c.downloadChapter(ctx, chapter, chapterPath, options)
		if err != nil {
			return nil, err
		}
		downChap.ComicInfoXMLStatus = ciXmlStatus

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
			return "", err
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
) (metadata.DownloadStatus, error) {
	pages, err := c.ChapterPages(ctx, chapter)
	if err != nil {
		return "", err
	}

	downloadedPages, err := c.DownloadPagesInBatch(ctx, pages)
	if err != nil {
		return "", err
	}

	for _, page := range downloadedPages {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		image, err := options.ImageTransformer(page.Image())
		if err != nil {
			return "", err
		}

		page.SetImage(image)
	}

	// Only CBZ writes the ComicInfo.xml, so by default it's skipped
	ciXmlStatusSkip := metadata.DownloadStatusSkip
	switch options.Format {
	case FormatPDF:
		file, err := c.options.FS.Create(path)
		if err != nil {
			return "", err
		}
		defer file.Close()

		return ciXmlStatusSkip, c.savePDF(downloadedPages, file)
	case FormatTAR:
		file, err := c.options.FS.Create(path)
		if err != nil {
			return "", err
		}
		defer file.Close()

		return ciXmlStatusSkip, c.saveTAR(downloadedPages, file)
	case FormatTARGZ:
		file, err := c.options.FS.Create(path)
		if err != nil {
			return "", err
		}
		defer file.Close()

		return ciXmlStatusSkip, c.saveTARGZ(downloadedPages, file)
	case FormatZIP:
		file, err := c.options.FS.Create(path)
		if err != nil {
			return "", err
		}
		defer file.Close()

		return ciXmlStatusSkip, c.saveZIP(downloadedPages, file)
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
			}
			ciXML, err := c.getComicInfoXML(chapter, metaChapter)
			switch {
			case err == nil:
				comicInfoXML = &ciXML
			case options.Strict:
				return "", err
			default:
				c.logger.Log("getting %s: %s", metadata.FilenameComicInfoXML, err)
				ciXMLFailed = true
			}
		}

		file, err := c.options.FS.Create(path)
		if err != nil {
			return "", err
		}
		defer file.Close()

		status, err := c.saveCBZ(downloadedPages, file, comicInfoXML, options.ComicInfoXMLOptions)
		if err == nil && ciXMLFailed {
			status = metadata.DownloadStatusFailed
		}
		return status, err
	case FormatImages:
		if err := c.options.FS.MkdirAll(path, c.options.ModeDir); err != nil {
			return "", err
		}

		for i, page := range downloadedPages {
			name := fmt.Sprintf("%04d%s", i+1, page.Extension())
			err := afero.WriteFile(
				c.options.FS,
				filepath.Join(path, name),
				page.Image(),
				c.options.ModeFile,
			)
			if err != nil {
				return "", err
			}
		}

		return ciXmlStatusSkip, nil
	default:
		// format validation was done before
		panic("unreachable")
	}
}

// DownloadPagesInBatch downloads multiple pages in batch
// by calling DownloadPage for each page in a separate goroutines.
//
// If any of the pages fails to download it will stop downloading other pages
// and return error immediately.
func (c *Client) DownloadPagesInBatch(
	ctx context.Context,
	pages []mangadata.Page,
) ([]mangadata.PageWithImage, error) {
	if len(pages) == 0 {
		return nil, fmt.Errorf("no pages provided for chapter")
	}
	c.logger.Log("downloading %d pages", len(pages))

	g, ctx := errgroup.WithContext(ctx)
	downloadedPages := make([]mangadata.PageWithImage, len(pages))
	for i, page := range pages {
		g.Go(func() error {
			c.logger.Log("page #%03d: downloading", i+1)

			downloaded, err := c.DownloadPage(ctx, page)
			if err != nil {
				return err
			}

			c.logger.Log("page #%03d: done", i+1)

			downloadedPages[i] = downloaded
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return downloadedPages, nil
}

// DownloadPage downloads a page contents (image).
func (c *Client) DownloadPage(
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
