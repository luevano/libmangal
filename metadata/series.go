package metadata

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// monthYear formats a date like "November 2011", or just "2011" without a month.
func monthYear(d Date) string {
	switch {
	case d.Year == 0:
		return ""
	case d.Month < 1 || d.Month > 12:
		return strconv.Itoa(d.Year)
	default:
		return fmt.Sprintf("%s %d", time.Month(d.Month), d.Year)
	}
}

func ToSeriesJSON(m Metadata) SeriesJSON {
	// series.json only has these two
	status := "Continuing"
	switch m.Status() {
	case StatusFinished, StatusCancelled:
		status = "Ended"
	}

	// Format should be:
	// "November 2011 - July 2016"
	// "June 2021 - Present"
	pubEndDate := monthYear(m.EndDate())
	if pubEndDate == "" {
		pubEndDate = "Present"
	}
	publicationRun := monthYear(m.StartDate()) + " - " + pubEndDate

	return SeriesJSON{
		Type:                 "comicSeries",
		Name:                 m.Title(),
		DescriptionFormatted: m.Description(),
		DescriptionText:      StripHTML(m.Description()),
		Status:               status,
		Year:                 m.StartDate().Year,
		ComicImage:           m.Cover(),
		Publisher:            m.Publisher(),
		ComicID:              m.ID().Value(),
		BookType:             "Print",
		TotalIssues:          m.Chapters(),
		PublicationRun:       publicationRun,
	}
}

// SeriesJSON v1.0.2 is similar to ComicInfoXML but designed for
// the series as a whole rather than a single chapter. Defined by MyLar.
//
// https://github.com/mylar3/mylar3/wiki/series.json-schema-%28version-1.0.2%29
type SeriesJSON struct {
	Type                 string `json:"type"`
	Name                 string `json:"name"`
	DescriptionFormatted string `json:"description_formatted"`
	DescriptionText      string `json:"description_text"`
	Status               string `json:"status"`
	Year                 int    `json:"year"`
	ComicImage           string `json:"comic_image"`
	Publisher            string `json:"publisher"`
	ComicID              int    `json:"comicid"`
	BookType             string `json:"booktype"`
	TotalIssues          int    `json:"total_issues"`
	PublicationRun       string `json:"publication_run"`
}

func (s SeriesJSON) Marshal() ([]byte, error) {
	buffer := &bytes.Buffer{}
	enc := json.NewEncoder(buffer)
	enc.SetIndent("", "  ")
	err := enc.Encode(s.wrapper())

	return buffer.Bytes(), err
}

func (s SeriesJSON) wrapper() seriesJSONWrapper {
	return seriesJSONWrapper{Metadata: s}
}

type seriesJSONWrapper struct {
	Metadata SeriesJSON `json:"metadata"`
}
