package myanimelist

import (
	"strconv"
	"strings"

	"github.com/luevano/libmangal/metadata"
)

const mangaURL = "https://myanimelist.net/manga/"

var _ metadata.Metadata = (*Manga)(nil)

type Status string

const (
	StatusFinished            Status = "finished"
	StatusCurrentlyPublishing Status = "currently_publishing"
	StatusNotYetPublished     Status = "not_yet_published"
	StatusOnHiatus            Status = "on_hiatus"
	StatusDiscontinued        Status = "discontinued"
)

// Manga is a metadata.Metadata implementation
// for MyAnimeList manga metadata.
//
// Note that Manga fields don't match the incoming json
// fields to avoid collisions with the interface.
type Manga struct {
	IDProvider    int    `json:"id"`
	TitleProvider string `json:"title"`
	MainPicture   struct {
		Large  string `json:"large"`
		Medium string `json:"medium"`
	} `json:"main_picture"`
	AlternativeTitles struct {
		Synonyms []string `json:"synonyms"`
		En       string   `json:"en"`
		Ja       string   `json:"ja"`
	} `json:"alternative_titles"`
	DateStart  date    `json:"start_date"`
	DateEnd    date    `json:"end_date"`
	Synopsis   string  `json:"synopsis"`
	Mean       float32 `json:"mean"`
	Rank       int     `json:"rank"`
	Popularity int     `json:"popularity"`
	NSFW       string  `json:"nsfw" jsonschema:"enum=white,enum=gray,enum=black"`
	GenreList  []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"genres"`
	CreatedAt         string `json:"created_at"`
	UpdatedAt         string `json:"updated_at"`
	MediaType         string `json:"media_type" jsonschema:"enum=unknown,enum=manga,enum=novel,enum=one_shot,enum=doujinshi,enum=manhwa,enum=manhua,enum=oel"`
	PublicationStatus Status `json:"status"`
	NumVolumes        int    `json:"num_volumes"`
	NumChapters       int    `json:"num_chapters"`
	AuthorList        []struct {
		Node authorListNode `json:"node"`
		Role string         `json:"role"`
	} `json:"authors"`
}

type date string

// toMetadataDate parses "YYYY", "YYYY-MM" or "YYYY-MM-DD", unknown parts are left as zero.
//
// From: https://myanimelist.net/apiconfig/references/api/v2#section/Common-formats
func (d date) toMetadataDate() metadata.Date {
	var parsed metadata.Date
	parts := strings.SplitN(string(d), "-", 3)
	fields := []*int{&parsed.Year, &parsed.Month, &parsed.Day}
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil {
			break
		}
		*fields[i] = n
	}
	return parsed
}

type authorListNode struct {
	ID        int    `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// name is "First Last", without a comma since ComicInfo.xml lists are comma separated.
// Either name can be empty.
func (a authorListNode) name() string {
	return strings.TrimSpace(a.FirstName + " " + a.LastName)
}

// String is the short representation of the manga.
// Must be non-empty.
//
// At the minimum it should return "`Title` (`Year`)", else
// "`Title` (`Year`) [`IDCode`id-`ID`]" if available.
func (m *Manga) String() string {
	base := m.Title() + " (" + strconv.Itoa(m.StartDate().Year)
	if m.ID().Value() == 0 {
		return base + ")"
	}
	return base + ") [" + string(m.ID().Code) + "id-" + strconv.Itoa(m.ID().Value()) + "]"
}

// Title is the English title of the manga.
// Must be non-empty.
//
// If English is not available, then in order of availability:
// Romaji (the romanized title) or Native (usually Kanji).
func (m *Manga) Title() string {
	return m.TitleProvider
}

// AlternateTitles is a list of alternative titles in order of relevance.
func (m *Manga) AlternateTitles() []string {
	var titles []string
	if m.AlternativeTitles.En != "" {
		titles = append(titles, m.AlternativeTitles.En)
	}
	if m.AlternativeTitles.Ja != "" {
		titles = append(titles, m.AlternativeTitles.Ja)
	}
	titles = append(titles, m.AlternativeTitles.Synonyms...)
	return titles
}

// Score is the community score for the manga.
//
// Accepted values are between 0.0 and 5.0.
func (m *Manga) Score() float32 {
	return m.Mean / 2.0
}

// Description is the description/summary for the manga.
func (m *Manga) Description() string {
	return m.Synopsis
}

// Cover is the cover image of the manga.
func (m *Manga) Cover() string {
	if pic := m.MainPicture.Large; pic != "" {
		return pic
	}
	return m.MainPicture.Medium
}

// Banner is the banner image of the manga.
func (m *Manga) Banner() string {
	return "" // MAL doesn't provide banners
}

// Tags is the list of tags associated with the manga.
func (m *Manga) Tags() []string {
	// There is no easy way to separate "tags" from the "genres",
	// but this could help in the future: https://myanimelist.net/manga/genre/info
	return []string{}
}

// Genres is the list of genres associated with the manga.
func (m *Manga) Genres() []string {
	// Same as with tags, the genres include everything mixed together.
	genres := make([]string, len(m.GenreList))
	for i, g := range m.GenreList {
		genres[i] = g.Name
	}
	return genres
}

// Characters is the list of characters, in order of relevance.
func (m *Manga) Characters() []string {
	// No public documented enpoint available as of now. For more:
	// https://myanimelist.net/forum/?topicid=1973141
	return []string{}
}

// Authors (or Writers) is the list of authors, in order of relevance.
// Must contain at least one author.
func (m *Manga) Authors() []string {
	var authors []string
	for _, author := range m.AuthorList {
		role := strings.ToLower(author.Role)
		if strings.Contains(role, "story") {
			if name := author.Node.name(); name != "" {
				authors = append(authors, name)
			}
		}
	}
	return authors
}

// Artists is the list of artists, in order of relevance.
func (m *Manga) Artists() []string {
	var artists []string
	for _, author := range m.AuthorList {
		role := strings.ToLower(author.Role)
		if strings.Contains(role, "art") {
			if name := author.Node.name(); name != "" {
				artists = append(artists, name)
			}
		}
	}
	return artists
}

// Translators is the list of translators, in order of relevance.
func (m *Manga) Translators() []string {
	var translators []string
	for _, author := range m.AuthorList {
		role := strings.ToLower(author.Role)
		if strings.Contains(role, "translator") { // TODO: check the actual role name (if any)
			if name := author.Node.name(); name != "" {
				translators = append(translators, name)
			}
		}
	}
	return translators
}

// Letterers is the list of letterers, in order of relevance.
func (m *Manga) Letterers() []string {
	var letterers []string
	for _, author := range m.AuthorList {
		role := strings.ToLower(author.Role)
		if strings.Contains(role, "lettering") { // TODO: check the actual role name (if any)
			if name := author.Node.name(); name != "" {
				letterers = append(letterers, name)
			}
		}
	}
	return letterers
}

// StartDate is the date the manga started publishing.
// Must be non-zero.
func (m *Manga) StartDate() metadata.Date {
	return m.DateStart.toMetadataDate()
}

// EndDate is the date the manga ended publishing.
func (m *Manga) EndDate() metadata.Date {
	return m.DateEnd.toMetadataDate()
}

// Publisher of the manga.
func (m *Manga) Publisher() string {
	return ""
}

// Current status of the manga.
// Must be non-empty.
//
// One of: FINISHED, RELEASING, NOT_YET_RELEASED, CANCELLED, HIATUS
func (m *Manga) Status() metadata.Status {
	switch m.PublicationStatus {
	case StatusFinished:
		return metadata.StatusFinished
	case StatusCurrentlyPublishing:
		return metadata.StatusReleasing
	case StatusNotYetPublished:
		return metadata.StatusNotYetReleased
	case StatusOnHiatus:
		return metadata.StatusHiatus
	case StatusDiscontinued:
		return metadata.StatusCancelled
	default:
		// assume that if it is unknown it hasn't been released...
		return metadata.StatusNotYetReleased
	}
}

// Format the original publication.
//
// For example: TBP, HC, Web, Digital, etc..
func (m *Manga) Format() string {
	return ""
}

// Country of origin of the manga. ISO 3166-1 alpha-2 country code.
func (m *Manga) Country() string {
	return ""
}

// Chapter count until this point.
func (m *Manga) Chapters() int {
	return m.NumChapters
}

// Extra notes to be added.
func (m *Manga) Notes() string {
	return ""
}

// URL is the source URL of the metadata.
func (m *Manga) URL() string {
	return mangaURL + strconv.Itoa(m.IDProvider)
}

// ID is the ID information of the metadata.
// Must be valid (ID.Validate).
func (m *Manga) ID() metadata.ID {
	return metadata.ID{
		Raw:    strconv.Itoa(m.IDProvider),
		Source: metadata.IDSourceMyAnimeList,
		Code:   metadata.IDCodeMyAnimeList,
	}
}

// ExtraIDs is a list of extra available IDs in the metadata provider.
// Each extra ID must be valid (ID.Validate).
func (m *Manga) ExtraIDs() []metadata.ID {
	return []metadata.ID{}
}
