package anilist

import (
	"strconv"
	"strings"

	"github.com/luevano/libmangal/metadata"
)

var _ metadata.Metadata = (*Manga)(nil)

// Manga is a metadata.Metadata implementation
// for Anilist manga metadata.
//
// Note that Manga fields don't match the incoming json
// fields to avoid collisions with the interface.
type Manga struct {
	TitleGroup struct {
		English string `json:"english" jsonschema:"description=English title of the manga."`
		Romaji  string `json:"romaji" jsonschema:"description=Romanized title of the manga."`
		Native  string `json:"native" jsonschema:"description=Native title of the manga. Usually in kanji."`
	} `json:"title"`
	AverageScore int    `json:"averageScore" jsonschema:"description=Average score of the manga on Anilist."`
	Summary      string `json:"description" jsonschema:"description=Description of the manga in html format."`
	CoverImage   struct {
		ExtraLarge string `json:"extraLarge" jsonschema:"description=URL of the extra large cover image. If the image is not available, large will be used instead."`
		Large      string `json:"large" jsonschema:"description=URL of the large cover image."`
		Medium     string `json:"medium" jsonschema:"description=URL of the medium cover image."`
		Color      string `json:"color" jsonschema:"description=Average color of the cover image."`
	} `json:"coverImage" jsonschema:"description=Cover image of the manga."`
	BannerImage string `json:"bannerImage" jsonschema:"description=Banner image of the manga."`
	TagList     []struct {
		Name        string `json:"name" jsonschema:"description=Name of the tag."`
		Description string `json:"description" jsonschema:"description=Description of the tag."`
		Rank        int    `json:"rank" jsonschema:"description=Rank of the tag. How relevant it is to the manga from 1 to 100."`
	} `json:"tags"`
	GenreList     []string `json:"genres" jsonschema:"description=Genres of the manga."`
	CharacterList struct {
		Nodes []struct {
			Name struct {
				Full   string `json:"full" jsonschema:"description=Full name of the character."`
				Native string `json:"native" jsonschema:"description=Native name of the character. Usually in kanji."`
			} `json:"name"`
		} `json:"nodes"`
	} `json:"characters"`
	Staff struct {
		Edges []struct {
			Role string `json:"role" jsonschema:"description=Role of the staff member."`
			Node struct {
				Name struct {
					Full string `json:"full" jsonschema:"description=Full name of the staff member."`
				} `json:"name"`
			} `json:"node"`
		} `json:"edges"`
	} `json:"staff"`
	DateStart         metadata.Date   `json:"startDate" jsonschema:"description=Date the manga started publishing."`
	DateEnd           metadata.Date   `json:"endDate" jsonschema:"description=Date the manga ended publishing."`
	Synonyms          []string        `json:"synonyms" jsonschema:"description=Synonyms of the manga (Alternative titles)."`
	PublicationStatus metadata.Status `json:"status" jsonschema:"enum=FINISHED,enum=RELEASING,enum=NOT_YET_RELEASED,enum=CANCELLED,enum=HIATUS"`
	ChapterCount      int             `json:"chapters" jsonschema:"description=Amount of chapters the manga has when complete."`
	SiteURL           string          `json:"siteUrl" jsonschema:"description=URL of the manga on AnilistSearch."`
	CountryOfOrigin   string          `json:"countryOfOrigin" jsonschema:"description=Country of origin of the manga. ISO 3166-1 alpha-2 country code."`
	IDProvider        int             `json:"id" jsonschema:"description=ID of the manga on AnilistSearch."`
	IDMal             int             `json:"idMal" jsonschema:"description=ID of the manga on MyAnimeList."`
	External          []struct {
		URL string `json:"url" jsonschema:"description=URL of the external link."`
	} `json:"externalLinks" jsonschema:"description=External links related to the manga."`
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
	t := strings.TrimSpace(m.TitleGroup.English)
	if t == "" {
		t = strings.TrimSpace(m.TitleGroup.Romaji)
	}
	if t == "" {
		t = strings.TrimSpace(m.TitleGroup.Native)
	}
	return t
}

// AlternateTitles is a list of alternative titles in order of relevance.
func (m *Manga) AlternateTitles() []string {
	title := m.Title()

	var titles []string
	ts := []string{
		m.TitleGroup.English,
		m.TitleGroup.Romaji,
		m.TitleGroup.Native,
	}
	for _, t := range append(ts, m.Synonyms...) {
		t = strings.TrimSpace(t)
		if t == title || t == "" {
			continue
		}
		titles = append(titles, t)
	}
	return titles
}

// Score is the community score for the manga.
//
// Accepted values are between 0.0 and 5.0.
func (m *Manga) Score() float32 {
	return float32(m.AverageScore) / 20
}

// Description is the description/summary for the manga.
func (m *Manga) Description() string {
	return m.Summary
}

// Cover is the cover image of the manga.
func (m *Manga) Cover() string {
	coverImage := m.CoverImage.ExtraLarge
	if coverImage == "" {
		coverImage = m.CoverImage.Large
	}
	if coverImage == "" {
		coverImage = m.CoverImage.Medium
	}
	return coverImage
}

// Banner is the banner image of the manga.
func (m *Manga) Banner() string {
	return m.BannerImage
}

// Tags is the list of tags associated with the manga.
func (m *Manga) Tags() []string {
	tags := make([]string, 0)
	for _, tag := range m.TagList {
		// TODO: make the rank threshold configurable
		if tag.Rank < 60 {
			continue
		}
		tags = append(tags, tag.Name)
	}
	return tags
}

// Genres is the list of genres associated with the manga.
func (m *Manga) Genres() []string {
	return m.GenreList
}

// Characters is the list of characters, in order of relevance.
func (m *Manga) Characters() []string {
	characters := make([]string, len(m.CharacterList.Nodes))
	for i, node := range m.CharacterList.Nodes {
		characters[i] = node.Name.Full
	}
	return characters
}

// Authors (or Writers) is the list of authors, in order of relevance.
// Must contain at least one author.
func (m *Manga) Authors() []string {
	var authors []string
	for _, edge := range m.Staff.Edges {
		role := strings.ToLower(edge.Role)
		name := edge.Node.Name.Full
		if strings.Contains(role, "story") {
			authors = append(authors, name)
		}
	}
	return authors
}

// Artists is the list of artists, in order of relevance.
func (m *Manga) Artists() []string {
	var artists []string
	for _, edge := range m.Staff.Edges {
		role := strings.ToLower(edge.Role)
		name := edge.Node.Name.Full
		if strings.Contains(role, "art") {
			artists = append(artists, name)
		}
	}
	return artists
}

// Translators is the list of translators, in order of relevance.
func (m *Manga) Translators() []string {
	var translators []string
	for _, edge := range m.Staff.Edges {
		role := strings.ToLower(edge.Role)
		name := edge.Node.Name.Full
		if strings.Contains(role, "translator") {
			translators = append(translators, name)
		}
	}
	return translators
}

// Letterers is the list of letterers, in order of relevance.
func (m *Manga) Letterers() []string {
	var letterers []string
	for _, edge := range m.Staff.Edges {
		role := strings.ToLower(edge.Role)
		name := edge.Node.Name.Full
		if strings.Contains(role, "lettering") {
			letterers = append(letterers, name)
		}
	}
	return letterers
}

// StartDate is the date the manga started publishing.
// Must be non-zero.
func (m *Manga) StartDate() metadata.Date {
	return m.DateStart
}

// EndDate is the date the manga ended publishing.
func (m *Manga) EndDate() metadata.Date {
	return m.DateEnd
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
	return m.PublicationStatus
}

// Format the original publication.
//
// For example: TBP, HC, Web, Digital, etc..
func (m *Manga) Format() string {
	return ""
}

// Country of origin of the manga. ISO 3166-1 alpha-2 country code.
func (m *Manga) Country() string {
	return m.CountryOfOrigin
}

// Chapter count until this point.
func (m *Manga) Chapters() int {
	return m.ChapterCount
}

// Extra notes to be added.
func (m *Manga) Notes() string {
	return ""
}

// URL is the source URL of the metadata.
func (m *Manga) URL() string {
	return m.SiteURL
}

// ID is the ID information of the metadata.
// Must be valid (ID.Validate).
func (m *Manga) ID() metadata.ID {
	return metadata.ID{
		Raw:    strconv.Itoa(m.IDProvider),
		Source: metadata.IDSourceAnilist,
		Code:   metadata.IDCodeAnilist,
	}
}

// ExtraIDs is a list of extra available IDs in the metadata provider.
// Each extra ID must be valid (ID.Validate).
func (m *Manga) ExtraIDs() []metadata.ID {
	if m.IDMal == 0 {
		return []metadata.ID{}
	}
	return []metadata.ID{
		{
			Raw:    strconv.Itoa(m.IDMal),
			Source: metadata.IDSourceMyAnimeList,
			Code:   metadata.IDCodeMyAnimeList,
		},
	}
}
