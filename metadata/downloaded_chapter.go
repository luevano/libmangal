package metadata

import "path/filepath"

// DownloadedChapter provides general information about the downloaded chapter,
// and status for the metadata when the chapter was downloaded.
type DownloadedChapter struct {
	// Number of the chapter.
	Number float32 `json:"number"`

	// Title of the chapter.
	Title string `json:"title"`

	// Filename as written to system.
	Filename string `json:"filename"`

	// Directory of the chapter, under DownloadOptions.Directory.
	Directory string `json:"directory"`

	// Pages is how many pages were downloaded, 0 if the chapter was skipped (already downloaded).
	Pages int `json:"pages"`

	// ChapterStatus is the status of the downloaded chapter.
	ChapterStatus DownloadStatus `json:"chapter_status"`

	// SeriesJSONStatus is the status of the downloaded series.json.
	SeriesJSONStatus DownloadStatus `json:"series_json_status"`

	// ComicInfoXMLStatus is the status of the downloaded ComicInfo.xml.
	ComicInfoXMLStatus DownloadStatus `json:"comicinfo_xml_status"`

	// CoverStatus is the status of the downloaded cover image.
	CoverStatus DownloadStatus `json:"cover_status"`

	// BannerStatus is the status of the downloaded banner image.
	BannerStatus DownloadStatus `json:"banner_status"`
}

func (d *DownloadedChapter) Path() string {
	return filepath.Join(d.Directory, d.Filename)
}

type DownloadStatus string

const (
	DownloadStatusNew             DownloadStatus = "new"
	DownloadStatusSkip            DownloadStatus = "skip"
	DownloadStatusExists          DownloadStatus = "exists"
	DownloadStatusFailed          DownloadStatus = "failed"
	DownloadStatusMissingMetadata DownloadStatus = "missing_metadata"
	DownloadStatusOverwritten     DownloadStatus = "overwritten"
)
