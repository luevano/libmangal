package libmangal

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/luevano/libmangal/mangadata"
	"github.com/luevano/libmangal/metadata"
	"github.com/pdfcpu/pdfcpu/pkg/api"
)

type mangaImage string

const (
	mangaImageCover  mangaImage = "cover"
	mangaImageBanner mangaImage = "banner"
)

var _ mangadata.PageWithImage = (*pageWithImage)(nil)

// pageWithImage is a mangadata.PageWithImage implementation.
type pageWithImage struct {
	mangadata.Page
	image []byte
}

// Image gets the image contents.
//
// Implementation should not make any external requests.
// Should only be exposed if the Page already contains image contents.
func (p *pageWithImage) Image() []byte {
	return p.image
}

// SetImage sets the image contents. This is used by DownloadOptions.ImageTransformer.
func (p *pageWithImage) SetImage(newImage []byte) {
	p.image = newImage
}

// getComicInfoXML gets the ComicInfoXML for the chapter.
//
// It tries to check if chapter implements ChapterWithComicInfoXML
// in case of failure it will use the provided metadata.
//
// The metadata that it uses as fallback could be set by the provider,
// by the client or by libmangal (when no metadata is found it searches for it).
func (c *Client) getComicInfoXML(
	mangaChapter mangadata.Chapter,
	metaChapter metadata.Chapter,
) (metadata.ComicInfoXML, error) {
	withComicInfoXML, ok := mangaChapter.(mangadata.ChapterWithComicInfoXML)
	if ok {
		comicInfo, found, err := withComicInfoXML.ComicInfoXML()
		if err != nil {
			return metadata.ComicInfoXML{}, err
		}
		if found {
			return comicInfo, nil
		}
	}
	return metadata.ToComicInfoXML(mangaChapter.Volume().Manga().Metadata(), metaChapter), nil
}

// savePDF saves pages in FormatPDF
func (c *Client) savePDF(
	ctx context.Context,
	pages []mangadata.PageWithImage,
	out io.Writer,
) error {
	c.logger.Log("saving %d pages as PDF", len(pages))

	// convert to readers
	images := make([]io.Reader, len(pages))
	for i, page := range pages {
		images[i] = bytes.NewReader(page.Image())
	}

	return api.ImportImages(ctx, nil, out, images, nil, nil)
}

// saveCBZ saves pages in FormatCBZ
func (c *Client) saveCBZ(
	pages []mangadata.PageWithImage,
	out io.Writer,
	comicInfoXml *metadata.ComicInfoXML,
	options metadata.ComicInfoXMLOptions,
) (metadata.DownloadStatus, error) {
	c.logger.Log("saving %d pages as CBZ", len(pages))

	zipWriter := zip.NewWriter(out)

	for i, page := range pages {
		writer, err := zipWriter.CreateHeader(&zip.FileHeader{
			Name:     fmt.Sprintf("%04d%s", i+1, page.Extension()),
			Method:   zip.Store,
			Modified: time.Now(),
		})
		if err != nil {
			return "", err
		}

		_, err = writer.Write(page.Image())
		if err != nil {
			return "", err
		}
	}

	ciXmlStatus := metadata.DownloadStatusMissingMetadata
	if comicInfoXml != nil {
		ciXmlStatus = metadata.DownloadStatusNew
		marshalled, err := comicInfoXml.Marshal(options)
		if err != nil {
			return "", err
		}

		writer, err := zipWriter.CreateHeader(&zip.FileHeader{
			Name:     metadata.FilenameComicInfoXML,
			Method:   zip.Store,
			Modified: time.Now(),
		})
		if err != nil {
			return "", err
		}

		_, err = writer.Write(marshalled)
		if err != nil {
			return "", err
		}
	}

	// closing writes the central directory
	if err := zipWriter.Close(); err != nil {
		return "", err
	}
	return ciXmlStatus, nil
}

func (c *Client) saveTAR(
	pages []mangadata.PageWithImage,
	out io.Writer,
) error {
	c.logger.Log("saving %d pages as TAR", len(pages))

	tarWriter := tar.NewWriter(out)

	for i, page := range pages {
		image := page.Image()
		err := tarWriter.WriteHeader(&tar.Header{
			Name:    fmt.Sprintf("%04d%s", i+1, page.Extension()),
			Size:    int64(len(image)),
			Mode:    int64(c.options.ModeFile),
			ModTime: time.Now(),
		})
		if err != nil {
			return err
		}

		_, err = tarWriter.Write(image)
		if err != nil {
			return err
		}
	}

	return tarWriter.Close()
}

func (c *Client) saveTARGZ(
	pages []mangadata.PageWithImage,
	out io.Writer,
) error {
	c.logger.Log("bundling TAR into GZIP")

	gzipWriter := gzip.NewWriter(out)
	if err := c.saveTAR(pages, gzipWriter); err != nil {
		return err
	}
	return gzipWriter.Close()
}

func (c *Client) saveZIP(
	pages []mangadata.PageWithImage,
	out io.Writer,
) error {
	c.logger.Log("saving %d pages as ZIP", len(pages))

	zipWriter := zip.NewWriter(out)

	for i, page := range pages {
		writer, err := zipWriter.CreateHeader(&zip.FileHeader{
			Name:     fmt.Sprintf("%04d%s", i+1, page.Extension()),
			Method:   zip.Store,
			Modified: time.Now(),
		})
		if err != nil {
			return err
		}

		_, err = writer.Write(page.Image())
		if err != nil {
			return err
		}
	}

	return zipWriter.Close()
}

// downloadMangaImage will download image related to manga.
//
// For example this can be either banner image or cover image.
//
// The manga page's origin is sent as the Referer for images that come from the provider,
// like a browser does for cross-origin images (some image hosts refuse the full page url).
func (c *Client) downloadMangaImage(
	ctx context.Context,
	manga mangadata.Manga,
	mangaImage mangaImage,
	out io.Writer,
) error {
	c.logger.Log("downloading %s image", mangaImage)
	var URL string
	var fromProvider bool
	switch mangaImage {
	case mangaImageCover:
		URL, fromProvider = getCover(manga)
	case mangaImageBanner:
		URL, fromProvider = getBanner(manga)
	default:
		return fmt.Errorf("unknown manga image type %q", mangaImage)
	}
	if URL == "" {
		return fmt.Errorf("no %s image url", mangaImage)
	}
	c.logger.Log("%s image url: %s", mangaImage, URL)

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, URL, nil)
	if err != nil {
		return fmt.Errorf("download %s: %w", mangaImage, err)
	}

	if fromProvider {
		if referer := origin(manga.Info().URL); referer != "" {
			request.Header.Set("Referer", referer)
		}
	}
	request.Header.Set("User-Agent", c.options.UserAgent)
	request.Header.Set("Accept", "image/webp,image/apng,image/*,*/*;q=0.8")

	response, err := c.options.HTTPClient.Do(request)
	if err != nil {
		return fmt.Errorf("download %s: %w", mangaImage, err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: GET %s: %d %s", mangaImage, URL, response.StatusCode, strings.ToLower(http.StatusText(response.StatusCode)))
	}

	if _, err := io.Copy(out, response.Body); err != nil {
		return fmt.Errorf("download %s: %w", mangaImage, err)
	}
	return nil
}

// origin of rawURL ("https://host/"), empty if it isn't an absolute url.
func origin(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host + "/"
}

// getCover returns the cover image URL, and whether it's the provider's
// instead of the metadata's.
func getCover(manga mangadata.Manga) (string, bool) {
	if cover := manga.Info().Cover; cover != "" {
		return cover, true
	}
	if manga.Metadata() != nil {
		return manga.Metadata().Cover(), false
	}
	return "", false
}

// getBanner returns the banner image URL, and whether it's the provider's
// instead of the metadata's.
func getBanner(manga mangadata.Manga) (string, bool) {
	if banner := manga.Info().Banner; banner != "" {
		return banner, true
	}
	if manga.Metadata() != nil {
		return manga.Metadata().Banner(), false
	}
	return "", false
}

// getSeriesJSON gets SeriesJSON from the chapter.
//
// It tries to check if chapter manga implements MangaWithSeriesJSON
// in case of failure it will use the provided metadata.
//
// The metadata that it uses as fallback could be set by the provider,
// by the client or by libmangal (when no metadata is found it searches for it).
func (c *Client) getSeriesJSON(
	manga mangadata.Manga,
) (metadata.SeriesJSON, error) {
	withSeriesJSON, ok := manga.(mangadata.MangaWithSeriesJSON)
	if ok {
		seriesJSON, found, err := withSeriesJSON.SeriesJSON()
		if err != nil {
			return metadata.SeriesJSON{}, err
		}
		if found {
			return seriesJSON, nil
		}
	}

	return metadata.ToSeriesJSON(manga.Metadata()), nil
}

// writeSeriesJSON writes the series.json file to disk.
func (c *Client) writeSeriesJSON(
	manga mangadata.Manga,
	out io.Writer,
) error {
	c.logger.Log("writing %s", metadata.FilenameSeriesJSON)

	seriesJSON, err := c.getSeriesJSON(manga)
	if err != nil {
		return err
	}

	marshalled, err := seriesJSON.Marshal()
	if err != nil {
		return err
	}

	_, err = out.Write(marshalled)
	return err
}
