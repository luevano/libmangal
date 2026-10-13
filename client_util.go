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
	"regexp"
	"strings"
	"time"

	"github.com/luevano/libmangal/mangadata"
	"github.com/luevano/libmangal/metadata"
	"github.com/pdfcpu/pdfcpu/pkg/api"
)

type mangaImage string

// maxMangaImageSize caps cover and banner downloads.
const maxMangaImageSize = 20 << 20

// pageExtension is what a page extension must look like: it ends up in file paths and
// archive entry names, so anything else could escape the chapter.
var pageExtension = regexp.MustCompile(`^\.[A-Za-z0-9]{1,8}$`)

// pageName is the file (or archive entry) name of the i-th page (0-based).
func pageName(i int, page mangadata.Page) (string, error) {
	ext := page.Extension()
	if !pageExtension.MatchString(ext) {
		return "", fmt.Errorf("page %d: invalid extension %q", i+1, ext)
	}
	return fmt.Sprintf("%04d%s", i+1, ext), nil
}

// checkPageNames fails if any page has an invalid extension.
func checkPageNames[P mangadata.Page](pages []P) error {
	for i, page := range pages {
		if _, err := pageName(i, page); err != nil {
			return err
		}
	}
	return nil
}

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
	if err := writeZIPPages(zipWriter, pages); err != nil {
		return "", err
	}

	ciXmlStatus := metadata.DownloadStatusMissingMetadata
	if comicInfoXml != nil {
		ciXmlStatus = metadata.DownloadStatusNew
		marshalled, err := comicInfoXml.Marshal(options)
		if err != nil {
			return "", err
		}
		if err := writeZIPFile(zipWriter, metadata.FilenameComicInfoXML, marshalled); err != nil {
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
		name, err := pageName(i, page)
		if err != nil {
			return err
		}
		image := page.Image()
		err = tarWriter.WriteHeader(&tar.Header{
			Name:    name,
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
	if err := writeZIPPages(zipWriter, pages); err != nil {
		return err
	}
	return zipWriter.Close()
}

func writeZIPPages(zipWriter *zip.Writer, pages []mangadata.PageWithImage) error {
	for i, page := range pages {
		name, err := pageName(i, page)
		if err != nil {
			return err
		}
		if err := writeZIPFile(zipWriter, name, page.Image()); err != nil {
			return err
		}
	}
	return nil
}

func writeZIPFile(zipWriter *zip.Writer, name string, data []byte) error {
	writer, err := zipWriter.CreateHeader(&zip.FileHeader{
		Name:     name,
		Method:   zip.Store,
		Modified: time.Now(),
	})
	if err != nil {
		return err
	}
	_, err = writer.Write(data)
	return err
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
	URL, fromProvider, err := mangaImageURL(manga, mangaImage)
	if err != nil {
		return err
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

	data, err := io.ReadAll(io.LimitReader(response.Body, maxMangaImageSize+1))
	if err != nil {
		return fmt.Errorf("download %s: %w", mangaImage, err)
	}
	if len(data) > maxMangaImageSize {
		return fmt.Errorf("download %s: GET %s: bigger than %d MB", mangaImage, URL, maxMangaImageSize>>20)
	}
	// some image hosts answer application/octet-stream, so the content decides then
	contentType := response.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		contentType = http.DetectContentType(data)
	}
	if !strings.HasPrefix(contentType, "image/") {
		return fmt.Errorf("download %s: GET %s: not an image (%s)", mangaImage, URL, contentType)
	}
	if _, err := out.Write(data); err != nil {
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

// mangaImageURL returns the image's URL, and whether it's the provider's
// instead of the metadata's.
func mangaImageURL(manga mangadata.Manga, image mangaImage) (string, bool, error) {
	var fromProvider string
	var fromMetadata func(metadata.Metadata) string
	switch image {
	case mangaImageCover:
		fromProvider, fromMetadata = manga.Info().Cover, metadata.Metadata.Cover
	case mangaImageBanner:
		fromProvider, fromMetadata = manga.Info().Banner, metadata.Metadata.Banner
	default:
		return "", false, fmt.Errorf("unknown manga image type %q", image)
	}
	if fromProvider != "" {
		return fromProvider, true, nil
	}
	if meta := manga.Metadata(); meta != nil {
		return fromMetadata(meta), false, nil
	}
	return "", false, nil
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
