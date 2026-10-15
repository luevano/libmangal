package metadata

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ReadSeriesJSON reads a series.json, what SeriesJSON.Marshal writes (the fields under
// "metadata"). Fields of an unexpected type are left empty instead of failing the read:
// other tools write series.json too.
func ReadSeriesJSON(r io.Reader) (SeriesJSON, error) {
	var wrapper seriesJSONWrapper
	err := json.NewDecoder(r).Decode(&wrapper)
	if _, ok := errors.AsType[*json.UnmarshalTypeError](err); ok {
		err = nil
	}
	if err != nil {
		return SeriesJSON{}, fmt.Errorf("read %s: %w", FilenameSeriesJSON, err)
	}
	return wrapper.Metadata, nil
}

// comicInfoXMLRead is comicInfoXMLWrapper as strings, so a field another tool wrote in
// an unexpected way doesn't fail the whole read.
type comicInfoXMLRead struct {
	Title           string `xml:"Title"`
	Series          string `xml:"Series"`
	Number          string `xml:"Number"`
	Count           string `xml:"Count"`
	Volume          string `xml:"Volume"`
	Summary         string `xml:"Summary"`
	Notes           string `xml:"Notes"`
	Year            string `xml:"Year"`
	Month           string `xml:"Month"`
	Day             string `xml:"Day"`
	Writer          string `xml:"Writer"`
	Penciller       string `xml:"Penciller"`
	Letterer        string `xml:"Letterer"`
	Translator      string `xml:"Translator"`
	Publisher       string `xml:"Publisher"`
	Genre           string `xml:"Genre"`
	Tags            string `xml:"Tags"`
	Web             string `xml:"Web"`
	PageCount       string `xml:"PageCount"`
	LanguageISO     string `xml:"LanguageISO"`
	Format          string `xml:"Format"`
	Characters      string `xml:"Characters"`
	ScanInformation string `xml:"ScanInformation"`
	StoryArc        string `xml:"StoryArc"`
	StoryArcNumber  string `xml:"StoryArcNumber"`
	AgeRating       string `xml:"AgeRating"`
	CommunityRating string `xml:"CommunityRating"`
	Review          string `xml:"Review"`
	GTIN            string `xml:"GTIN"`
}

// ReadComicInfoXML reads a ComicInfo.xml, what ComicInfoXML.Marshal writes. Lists are
// split on commas, the Notes lose the footer Marshal adds, and numbers that don't parse
// are 0 (a missing or invalid Number sets NoNumber). Country stays empty: the file only
// has the reading direction.
func ReadComicInfoXML(r io.Reader) (ComicInfoXML, error) {
	var x comicInfoXMLRead
	if err := xml.NewDecoder(r).Decode(&x); err != nil {
		return ComicInfoXML{}, fmt.Errorf("read %s: %w", FilenameComicInfoXML, err)
	}
	c := ComicInfoXML{
		Title:           x.Title,
		Series:          x.Series,
		Web:             x.Web,
		Genres:          splitList(x.Genre),
		Summary:         x.Summary,
		Count:           atoi(x.Count),
		PageCount:       atoi(x.PageCount),
		Characters:      splitList(x.Characters),
		Year:            atoi(x.Year),
		Month:           atoi(x.Month),
		Day:             atoi(x.Day),
		Publisher:       x.Publisher,
		LanguageISO:     x.LanguageISO,
		StoryArc:        x.StoryArc,
		StoryArcNumber:  atoi(x.StoryArcNumber),
		ScanInformation: x.ScanInformation,
		AgeRating:       x.AgeRating,
		CommunityRating: atof(x.CommunityRating),
		Review:          x.Review,
		GTIN:            x.GTIN,
		Format:          x.Format,
		Writers:         splitList(x.Writer),
		Pencillers:      splitList(x.Penciller),
		Letterers:       splitList(x.Letterer),
		Translators:     splitList(x.Translator),
		Tags:            splitList(x.Tags),
		Notes:           strings.TrimRight(strings.TrimSuffix(x.Notes, comicInfoNotesFooter), "\n"),
		Volume:          atof(x.Volume),
	}
	if n, err := strconv.ParseFloat(strings.TrimSpace(x.Number), 32); err == nil {
		c.Number = float32(n)
	} else {
		c.NoNumber = true
	}
	return c, nil
}

func splitList(s string) []string {
	var list []string
	for item := range strings.SplitSeq(s, ",") {
		if item = strings.TrimSpace(item); item != "" {
			list = append(list, item)
		}
	}
	return list
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

func atof(s string) float32 {
	n, _ := strconv.ParseFloat(strings.TrimSpace(s), 32)
	return float32(n)
}
