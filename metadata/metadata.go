package metadata

import (
	"errors"
	"fmt"
	"strconv"
)

const (
	FilenameComicInfoXML = "ComicInfo.xml"
	FilenameSeriesJSON   = "series.json"
	FilenameCoverJPG     = "cover.jpg"
	FilenameBannerJPG    = "banner.jpg"
)

type Status string

const (
	StatusFinished       Status = "FINISHED"
	StatusReleasing      Status = "RELEASING"
	StatusNotYetReleased Status = "NOT_YET_RELEASED"
	StatusCancelled      Status = "CANCELLED"
	StatusHiatus         Status = "HIATUS"
)

type IDSource uint8

const (
	IDSourceProvider IDSource = iota + 1
	IDSourceAnilist
	IDSourceMyAnimeList
	IDSourceKitsu
	IDSourceMangaUpdates
	IDSourceAnimePlanet
)

type IDCode string

const (
	IDCodeAnilist      IDCode = "al"
	IDCodeMyAnimeList  IDCode = "mal"
	IDCodeKitsu        IDCode = "kt"
	IDCodeMangaUpdates IDCode = "mu"
	IDCodeAnimePlanet  IDCode = "ap"
)

// Date is a simple date holder.
type Date struct {
	Year  int `json:"year"`
	Month int `json:"month"`
	Day   int `json:"day"`
}

// String formats the date as YYYY-MM-DD, leaving out the unknown (zero) parts:
// YYYY-MM, YYYY or empty.
func (d Date) String() string {
	switch {
	case d.Year == 0:
		return ""
	case d.Month == 0:
		return fmt.Sprintf("%d", d.Year)
	case d.Day == 0:
		return fmt.Sprintf("%d-%02d", d.Year, d.Month)
	default:
		return fmt.Sprintf("%d-%02d-%02d", d.Year, d.Month, d.Day)
	}
}

// ID is the ID information of the metadata.
type ID struct {
	// Raw id of the manga in the provider.
	// Must be non-empty unless IDSource is IDSourceProvider.
	// Must be an integer unless IDSource is IDSourceAnimePlanet.
	Raw string

	// Source of the metadata.
	// Must be non-zero.
	Source IDSource

	// Code of the metadata provider.
	// Must be non-empty.
	//
	// For the Provider metadata, the code must be short, ideally 2-3 chars.
	Code IDCode
}

// Value is the integer value of the ID.
//
// If the IDSource is either IDSourceProvider
// or IDSourceAnimePlanet then returns 0.
func (id ID) Value() int {
	// IDRaw already validated
	i, _ := strconv.Atoi(id.Raw)
	return i
}

func (id ID) validate() error {
	if id.Code == "" {
		return errors.New("empty code")
	}

	switch id.Source {
	case 0:
		return errors.New("zero source")
	case IDSourceProvider:
		return nil
	case IDSourceAnilist, IDSourceMyAnimeList, IDSourceKitsu, IDSourceMangaUpdates:
		i, err := strconv.Atoi(id.Raw)
		if err != nil {
			return fmt.Errorf("%s id %q isn't an integer", id.Code, id.Raw)
		}
		if i < 1 {
			return fmt.Errorf("%s id %d isn't positive", id.Code, i)
		}
	case IDSourceAnimePlanet:
		if id.Raw == "" {
			return fmt.Errorf("empty %s id", id.Code)
		}
	default:
		return fmt.Errorf("unknown source %d", id.Source)
	}
	return nil
}

// Metadata is the general metadata information about a manga.
//
// In its most basic form, it's just the metadata that is available from the provider.
// It contains the necessary fields to build the series.json and ComicInfo.xml files.
type Metadata interface {
	// String is the short representation of the manga.
	// Must be non-empty.
	//
	// At the minimum it should return "`Title` (`Year`)", else
	// "`Title` (`Year`) [`IDCode`id-`ID`]" if available.
	String() string

	// Title is the English title of the manga.
	// Must be non-empty.
	//
	// If English is not available, then in order of availability:
	// Romaji (the romanized title) or Native (usually Kanji).
	Title() string

	// AlternateTitles is a list of alternative titles in order of relevance.
	AlternateTitles() []string

	// Score is the community score for the manga.
	//
	// Accepted values are between 0.0 and 5.0.
	Score() float32

	// Description is the description/summary for the manga.
	Description() string

	// Cover is the cover image of the manga.
	Cover() string

	// Banner is the banner image of the manga.
	Banner() string

	// Tags is the list of tags associated with the manga.
	Tags() []string

	// Genres is the list of genres associated with the manga.
	Genres() []string

	// Characters is the list of characters, in order of relevance.
	Characters() []string

	// Authors (or Writers) is the list of authors, in order of relevance.
	// Must contain at least one author.
	Authors() []string

	// Artists is the list of artists, in order of relevance.
	Artists() []string

	// Translators is the list of translators, in order of relevance.
	Translators() []string

	// Letterers is the list of letterers, in order of relevance.
	Letterers() []string

	// StartDate is the date the manga started publishing.
	// Must be non-zero.
	StartDate() Date

	// EndDate is the date the manga ended publishing.
	EndDate() Date

	// Publisher of the manga.
	Publisher() string

	// Current status of the manga.
	// Must be non-empty.
	//
	// One of: FINISHED, RELEASING, NOT_YET_RELEASED, CANCELLED, HIATUS
	Status() Status

	// Format the original publication.
	//
	// For example: TBP, HC, Web, Digital, etc..
	Format() string

	// Country of origin of the manga. ISO 3166-1 alpha-2 country code.
	Country() string

	// Chapter count until this point.
	Chapters() int

	// Extra notes to be added.
	Notes() string

	// URL is the source URL of the metadata.
	URL() string

	// ID is the ID information of the metadata.
	// Must be valid (ID.Validate).
	ID() ID

	// ExtraIDs is a list of extra available IDs in the metadata provider.
	// Each extra ID must be valid (ID.Validate).
	ExtraIDs() []ID
}

// Validate will make sure the Metadata is valid/usable
// to write enough metadata to files.
//
// At the very least checks that: Title, Description, Authors,
// StartDate and Status are non-empty/non-zero.
func Validate(m Metadata) error {
	if err := validate(m); err != nil {
		return fmt.Errorf("invalid metadata: %w", err)
	}
	return nil
}

func validate(m Metadata) error {
	switch {
	case m == nil:
		return errors.New("nil metadata")
	case m.String() == "":
		return errors.New("empty String()")
	case m.Title() == "":
		return errors.New("empty title")
	case len(m.Authors()) == 0:
		return errors.New("no authors")
	case m.StartDate() == (Date{}):
		return errors.New("no start date")
	case m.Status() == "":
		return errors.New("empty status")
	}

	if err := m.ID().validate(); err != nil {
		return fmt.Errorf("id: %w", err)
	}
	for _, id := range m.ExtraIDs() {
		if err := id.validate(); err != nil {
			return fmt.Errorf("extra id: %w", err)
		}
	}
	return nil
}
