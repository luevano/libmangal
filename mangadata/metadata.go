package mangadata

import (
	"strconv"
	"strings"

	"github.com/luevano/libmangal/metadata"
)

var _ metadata.Metadata = (*Metadata)(nil)

// Metadata is a metadata.Metadata implementation
// for a generic Provider Metadata, usable by mangadata implementations.
type Metadata struct {
	EnglishTitle      string          `json:"english_title"`
	RomajiTitle       string          `json:"romaji_title"`
	NativeTitle       string          `json:"native_title"`
	Synonyms          []string        `json:"synonyms"`
	CommunityScore    float32         `json:"community_score"`
	Summary           string          `json:"summary"`
	CoverImage        string          `json:"cover_image"`
	BannerImage       string          `json:"banner_image"`
	TagList           []string        `json:"tag_list"`
	GenreList         []string        `json:"genre_list"`
	CharacterList     []string        `json:"character_list"`
	AuthorList        []string        `json:"author_list"`
	ArtistList        []string        `json:"artist_list"`
	TranslatorList    []string        `json:"translator_list"`
	LettererList      []string        `json:"letterer_list"`
	DateStart         metadata.Date   `json:"date_start"`
	DateEnd           metadata.Date   `json:"date_end"`
	ProviderPublisher string          `json:"provider_publisher"`
	PublicationStatus metadata.Status `json:"publication_status"`
	PublicationFormat string          `json:"publication_format"`
	CountryOfOrigin   string          `json:"country_of_origin"`
	ChapterCount      int             `json:"chapter_count"`
	ExtraNotes        string          `json:"extra_notes"`
	SourceURL         string          `json:"source_url"`
	ProviderID        string          `json:"provider_id"`
	ProviderIDCode    string          `json:"provider_id_code"`
	OtherIDs          []metadata.ID   `json:"other_ids"`
}

// String is the short representation of the manga.
// Must be non-empty.
//
// At the minimum it should return "`Title` (`Year`)", else
// "`Title` (`Year`) [`IDCode`id-`ID`]" if available.
func (m *Metadata) String() string {
	base := m.Title() + " (" + strconv.Itoa(m.StartDate().Year)
	if m.ID().Value() == 0 || m.ID().Code == "" {
		return base + ")"
	}
	return base + ") [" + string(m.ID().Code) + "id-" + strconv.Itoa(m.ID().Value()) + "]"
}

// Title is the English title of the manga.
// Must be non-empty.
//
// If English is not available, then in order of availability:
// Romaji (the romanized title) or Native (usually Kanji).
func (m *Metadata) Title() string {
	t := strings.TrimSpace(m.EnglishTitle)
	if t == "" {
		t = strings.TrimSpace(m.RomajiTitle)
	}
	if t == "" {
		t = strings.TrimSpace(m.NativeTitle)
	}
	return t
}

// AlternateTitles is a list of alternative titles in order of relevance.
func (m *Metadata) AlternateTitles() []string {
	title := m.Title()

	var titles []string
	ts := []string{
		m.EnglishTitle,
		m.RomajiTitle,
		m.NativeTitle,
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
func (m *Metadata) Score() float32 {
	return m.CommunityScore
}

// Description is the description/summary for the manga.
func (m *Metadata) Description() string {
	return m.Summary
}

// CoverImage is the cover image of the manga.
func (m *Metadata) Cover() string {
	return m.CoverImage
}

// BannerImage is the banner image of the manga.
func (m *Metadata) Banner() string {
	return m.BannerImage
}

// Tags is the list of tags associated with the manga.
func (m *Metadata) Tags() []string {
	return m.TagList
}

// Genres is the list of genres associated with the manga.
func (m *Metadata) Genres() []string {
	return m.GenreList
}

// Characters is the list of characters, in order of relevance.
func (m *Metadata) Characters() []string {
	return m.CharacterList
}

// Authors (or Writers) is the list of authors, in order of relevance.
// Must contain at least one author.
func (m *Metadata) Authors() []string {
	return m.AuthorList
}

// Artists is the list of artists, in order of relevance.
func (m *Metadata) Artists() []string {
	return m.ArtistList
}

// Translators is the list of translators, in order of relevance.
func (m *Metadata) Translators() []string {
	return m.TranslatorList
}

// Letterers is the list of letterers, in order of relevance.
func (m *Metadata) Letterers() []string {
	return m.LettererList
}

// StartDate is the date the manga started publishing.
// Must be non-zero.
func (m *Metadata) StartDate() metadata.Date {
	return m.DateStart
}

// EndDate is the date the manga ended publishing.
func (m *Metadata) EndDate() metadata.Date {
	return m.DateEnd
}

// Publisher of the manga.
func (m *Metadata) Publisher() string {
	return m.ProviderPublisher
}

// Current status of the manga.
// Must be non-empty.
//
// One of: FINISHED, RELEASING, NOT_YET_RELEASED, CANCELLED, HIATUS
func (m *Metadata) Status() metadata.Status {
	return m.PublicationStatus
}

// Format the original publication.
//
// For example: TBP, HC, Web, Digital, etc..
func (m *Metadata) Format() string {
	return m.PublicationFormat
}

// Country of origin of the manga. ISO 3166-1 alpha-2 country code.
func (m *Metadata) Country() string {
	return m.CountryOfOrigin
}

// Chapter count until this point.
func (m *Metadata) Chapters() int {
	return m.ChapterCount
}

// Extra notes to be added.
func (m *Metadata) Notes() string {
	return m.ExtraNotes
}

// URL is the source URL of the metadata.
func (m *Metadata) URL() string {
	return m.SourceURL
}

// ID is the ID information of the metadata.
// Must be valid (ID.Validate).
func (m *Metadata) ID() metadata.ID {
	return metadata.ID{
		Raw:    m.ProviderID,
		Source: metadata.IDSourceProvider,
		Code:   metadata.IDCode(m.ProviderIDCode),
	}
}

// ExtraIDs is a list of extra available IDs in the metadata provider.
// Each extra ID must be valid (ID.Validate).
func (m *Metadata) ExtraIDs() []metadata.ID {
	return m.OtherIDs
}
