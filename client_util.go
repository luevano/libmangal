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
	"time"

	"github.com/luevano/libmangal/mangadata"
	"github.com/luevano/libmangal/metadata"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/spf13/afero"
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
	pages []mangadata.PageWithImage,
	out io.Writer,
) error {
	c.logger.Log("saving %d pages as PDF", len(pages))

	// convert to readers
	images := make([]io.Reader, len(pages))
	for i, page := range pages {
		images[i] = bytes.NewReader(page.Image())
	}

	return api.ImportImages(nil, out, images, nil, nil)
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
// Manga is required to set Referer header.
func (c *Client) downloadMangaImage(
	ctx context.Context,
	manga mangadata.Manga,
	mangaImage mangaImage,
	out io.Writer,
) error {
	c.logger.Log("downloading %s image", mangaImage)
	var URL string
	switch mangaImage {
	case mangaImageCover:
		URL = getCover(manga)
	case mangaImageBanner:
		URL = getBanner(manga)
	default:
		return fmt.Errorf("unknown manga image type %q to download", mangaImage)
	}
	if URL == "" {
		msgFmt := "%s image url not found"
		c.logger.Log(msgFmt, mangaImage)
		return fmt.Errorf(msgFmt, mangaImage)
	}
	c.logger.Log("%s image url: %s", mangaImage, URL)

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, URL, nil)
	if err != nil {
		return err
	}

	// TODO: change referer? this asumes that the cover/banner URL comes
	// from the manga site itself, what if it comes from anilist?
	request.Header.Set("Referer", manga.Info().URL)
	request.Header.Set("User-Agent", c.options.UserAgent)
	request.Header.Set("Accept", "image/webp,image/apng,image/*,*/*;q=0.8")

	response, err := c.options.HTTPClient.Do(request)
	if err != nil {
		return err
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected http status: %s", response.Status)
	}

	_, err = io.Copy(out, response.Body)
	return err
}

// getCover returns the cover image URL.
func getCover(manga mangadata.Manga) string {
	cover := manga.Info().Cover
	if cover != "" {
		return cover
	}
	if manga.Metadata() != nil {
		cover = manga.Metadata().Cover()
	}
	return cover
}

// getBanner returns the banner image URL.
func getBanner(manga mangadata.Manga) string {
	banner := manga.Info().Banner
	if banner != "" {
		return banner
	}
	if manga.Metadata() != nil {
		banner = manga.Metadata().Banner()
	}
	return banner
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

// removeChapter will remove chapter at given path.
//
// Doesn't matter if it's a directory or a file.
//
// Currently unused.
func (c *Client) removeChapter(chapterPath string) error {
	c.logger.Log("removing %s", chapterPath)

	isDir, err := afero.IsDir(c.options.FS, chapterPath)
	if err != nil {
		return err
	}

	if isDir {
		return c.options.FS.RemoveAll(chapterPath)
	}

	return c.options.FS.Remove(chapterPath)
}
