package libmangal

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http"

	"github.com/luevano/libmangal/mangadata"
	"github.com/luevano/libmangal/metadata"
	"github.com/spf13/afero"
)

const (
	defaultUserAgent string      = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:126.0) Gecko/20100101 Firefox/126.0"
	defaultModeDir   fs.FileMode = 0o755
	defaultModeFile  fs.FileMode = 0o644

	defaultConcurrentPages = 8
)

// ReadOptions configures the reader options.
type ReadOptions struct {
	// SaveHistory will save the chapter to the local history. Not implemented yet.
	SaveHistory bool

	// SaveAnilist will set the Anilist reading progress, needs to be logged in.
	SaveAnilist bool

	// SaveMyAnimeList will set the MyAnimeList reading progress, needs to be logged in.
	SaveMyAnimeList bool
}

// DefaultReadOptions constructs default ReadOptions.
func DefaultReadOptions() ReadOptions {
	return ReadOptions{
		SaveHistory:     false,
		SaveAnilist:     false,
		SaveMyAnimeList: false,
	}
}

// DownloadOptions configures Chapter downloading.
type DownloadOptions struct {
	// Format in which a chapter must be downloaded.
	Format Format

	// Directory is the directory where manga will be downloaded to.
	Directory string

	// CreateProviderDir will create provider directory.
	CreateProviderDir bool

	// CreateMangaDir will create manga directory.
	CreateMangaDir bool

	// CreateVolumeDir will create volume directory.
	//
	// If CreateMangaDir is also true, volume directory
	// will be created under it.
	CreateVolumeDir bool

	// Strict means that if the metadata is invalid or if an error occurs during
	// metadata files creation, the chapter will not be written to disk.
	//
	// Some metadata is potentially written to disk.
	Strict bool

	// SkipIfExists will skip downloading chapter if its already downloaded (exists at path).
	//
	// However, metadata will still be created if needed.
	SkipIfExists bool

	// SearchMetadata will search for metadata on the available metadata providers,
	// and use the found metadata regardless of the incoming manga metadata which
	// could result in `nil` metadata when not found.
	//
	// Search priority is always by ID (if provided as part of one of the metadata fields), then by the title.
	SearchMetadata bool

	// DownloadMangaCover or not. Will not download cover again if its already downloaded.
	DownloadMangaCover bool

	// DownloadMangaBanner or not. Will not download banner again if its already downloaded.
	DownloadMangaBanner bool

	// WriteSeriesJSON write metadata series.json file in the manga directory.
	WriteSeriesJSON bool

	// SkipSeriesJSONIfOngoing will avoid writing series.json file for ongoing series,
	// due to lack of TotalIssues metadata provided by Anilist for example.
	//
	// There are issues with _some_ parsers when the TotalIssues is zero (I see you, Komga),
	// so this is a workaround. Also this avoids checks to overwrite series.json on each chapter,
	// silly, no (if only Komga would fix that)?
	SkipSeriesJSONIfOngoing bool

	// WriteComicInfoXML write metadata ComicInfo.xml file to the .cbz archive when
	// downloading with FormatCBZ.
	WriteComicInfoXML bool

	// ComicInfoXMLOptions options to use for ComicInfo.xml when WriteComicInfoXml is true.
	ComicInfoXMLOptions metadata.ComicInfoXMLOptions

	// ImageTransformer is applied for each image for the chapter.
	//
	// E.g. grayscale effect. Nil leaves the images as they are.
	ImageTransformer func([]byte) ([]byte, error)
}

func (o DownloadOptions) validate() error {
	if !o.Format.IsAFormat() {
		return fmt.Errorf("unknown format %d", o.Format)
	}
	return nil
}

// DefaultDownloadOptions constructs default DownloadOptions.
func DefaultDownloadOptions() DownloadOptions {
	return DownloadOptions{
		Format:                  FormatCBZ,
		Directory:               ".",
		CreateProviderDir:       false,
		CreateMangaDir:          true,
		CreateVolumeDir:         false,
		Strict:                  true,
		SkipIfExists:            true,
		SearchMetadata:          true,
		DownloadMangaCover:      false,
		DownloadMangaBanner:     false,
		WriteSeriesJSON:         false,
		SkipSeriesJSONIfOngoing: true, // Sensible default to avoid external parser issues.
		WriteComicInfoXML:       false,
		ImageTransformer: func(img []byte) ([]byte, error) {
			return img, nil
		},
		ComicInfoXMLOptions: metadata.DefaultComicInfoOptions(),
	}
}

// ClientOptions is options that client would use during its runtime.
type ClientOptions struct {
	// TODO: move HTTPClient (and UserAgent) out of the options?
	//
	// HTTPClient is http client that client would use for requests.
	HTTPClient *http.Client

	// UserAgent to use when making HTTP requests.
	UserAgent string

	// FS is a file system abstraction that the client will use.
	FS afero.Fs

	// ModeDir is the permission bits used for all dirs created.
	ModeDir fs.FileMode

	// ModeFile is the permission bits used for all files created.
	ModeFile fs.FileMode

	// ConcurrentPages is how many pages of a chapter are downloaded at the same time.
	// Zero or less means no limit.
	ConcurrentPages int

	// ProviderName determines the provider directory name.
	ProviderName func(
		provider ProviderInfo,
	) string

	// MangaName determines the manga directory name.
	MangaName func(
		provider ProviderInfo,
		manga mangadata.Manga,
	) string

	// VolumeName determines the volume directory name.
	// E.g. "Vol. 1" or "Volume 1"
	VolumeName func(
		provider ProviderInfo,
		volume mangadata.Volume,
	) string

	// ChapterName determines the chapter file name.
	// E.g. "[001] chapter 1" or "Chainsaw Man - Ch. 1"
	ChapterName func(
		provider ProviderInfo,
		chapter mangadata.Chapter,
	) string
}

func (o ClientOptions) validate() error {
	switch {
	case o.HTTPClient == nil:
		return errors.New("nil HTTPClient")
	case o.FS == nil:
		return errors.New("nil FS")
	case o.ProviderName == nil, o.MangaName == nil, o.VolumeName == nil, o.ChapterName == nil:
		return errors.New("nil ProviderName, MangaName, VolumeName or ChapterName")
	}
	return nil
}

// DefaultClientOptions constructs default ClientOptions, with default Anilist options as well.
func DefaultClientOptions() ClientOptions {
	return ClientOptions{
		HTTPClient:      &http.Client{},
		UserAgent:       defaultUserAgent,
		ModeDir:         defaultModeDir,
		ModeFile:        defaultModeFile,
		FS:              afero.NewOsFs(),
		ConcurrentPages: defaultConcurrentPages,
		ProviderName: func(provider ProviderInfo) string {
			return sanitizePath(provider.Name)
		},
		MangaName: func(_ ProviderInfo, manga mangadata.Manga) string {
			return sanitizePath(manga.Info().Title)
		},
		VolumeName: func(_ ProviderInfo, volume mangadata.Volume) string {
			return sanitizePath(fmt.Sprintf("Vol. %.1f", volume.Info().Number))
		},
		ChapterName: func(_ ProviderInfo, chapter mangadata.Chapter) string {
			info := chapter.Info()
			number := fmt.Sprintf("%06.1f", info.Number)
			return sanitizePath(fmt.Sprintf("[%s] %s", number, info.Title))
		},
	}
}
