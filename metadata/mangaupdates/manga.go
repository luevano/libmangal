package mangaupdates

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"github.com/luevano/libmangal/metadata"
)

// maxTags is how many of the most voted categories are the tags (popular series have
// hundreds).
const maxTags = 20

var _ metadata.Metadata = (*Manga)(nil)

// Category is a user-voted tag of a series.
type Category struct {
	Category string `json:"category"`
	Votes    int    `json:"votes"`
}

// Manga is a MangaUpdates series, as the API answers it (the fields used).
type Manga struct {
	SeriesID    int64  `json:"series_id"`
	SeriesTitle string `json:"title"`
	SeriesURL   string `json:"url"`
	Associated  []struct {
		Title string `json:"title"`
	} `json:"associated"`
	SeriesDescription string `json:"description"`
	Image             struct {
		URL struct {
			Original string `json:"original"`
		} `json:"url"`
	} `json:"image"`
	// Type is Manga, Manhwa, Manhua, Doujinshi, OEL, Novel, ...
	Type           string  `json:"type"`
	Year           string  `json:"year"`
	BayesianRating float32 `json:"bayesian_rating"`
	GenreList      []struct {
		Genre string `json:"genre"`
	} `json:"genres"`
	Categories    []Category `json:"categories"`
	LatestChapter int        `json:"latest_chapter"`
	// StatusText is free text, e.g. "115 Volumes (Ongoing)".
	StatusText string `json:"status"`
	Completed  bool   `json:"completed"`
	AuthorList []struct {
		Name string `json:"name"`
		// Type is Author or Artist.
		Type string `json:"type"`
	} `json:"authors"`
	Publishers []struct {
		Name string `json:"publisher_name"`
		// Type is Original or English.
		Type string `json:"type"`
	} `json:"publishers"`

	// SearchRecord: this is a search result (not an API field), see Partial.
	SearchRecord bool `json:"search_record,omitempty"`
	// HitTitle is the title the search matched, of a search result: often not the main one
	// ("Attack on Titan" hits "Shingeki no Kyojin").
	HitTitle string `json:"hit_title,omitempty"`
}

// Partial is whether this is a search result, without authors, status, categories or
// associated titles (see metadata.Partial).
func (m *Manga) Partial() bool {
	return m.SearchRecord
}

// String is the short representation of the manga.
// Must be non-empty.
//
// At the minimum it should return "`Title` (`Year`)", else
// "`Title` (`Year`) [`IDCode`id-`ID`]" if available.
func (m *Manga) String() string {
	base := m.Title() + " (" + strconv.Itoa(m.StartDate().Year)
	if m.SeriesID == 0 {
		return base + ")"
	}
	return base + ") [" + string(metadata.IDCodeMangaUpdates) + "id-" + strconv.FormatInt(m.SeriesID, 10) + "]"
}

// Title is the English title of the manga.
// Must be non-empty.
//
// MangaUpdates' main title is mostly the English or romanized one.
func (m *Manga) Title() string {
	return m.SeriesTitle
}

// AlternateTitles is a list of alternative titles in order of relevance.
func (m *Manga) AlternateTitles() []string {
	titles := make([]string, 0, len(m.Associated)+1)
	if m.HitTitle != "" && m.HitTitle != m.SeriesTitle {
		titles = append(titles, m.HitTitle)
	}
	for _, a := range m.Associated {
		if a.Title != "" {
			titles = append(titles, a.Title)
		}
	}
	return titles
}

// Score is the community score for the manga, the bayesian rating (0 to 10) halved.
//
// Accepted values are between 0.0 and 5.0.
func (m *Manga) Score() float32 {
	return min(max(m.BayesianRating/2, 0), 5)
}

// Description is the description/summary for the manga.
func (m *Manga) Description() string {
	return m.SeriesDescription
}

// Cover is the cover image of the manga.
func (m *Manga) Cover() string {
	return m.Image.URL.Original
}

// Banner is the banner image of the manga, MangaUpdates has none.
func (m *Manga) Banner() string {
	return ""
}

// Tags are the most voted categories (maxTags at most).
func (m *Manga) Tags() []string {
	categories := slices.Clone(m.Categories)
	slices.SortStableFunc(categories, func(a, b Category) int { return cmp.Compare(b.Votes, a.Votes) })
	var tags []string
	for _, c := range categories {
		if len(tags) == maxTags {
			break
		}
		if c.Votes > 0 && c.Category != "" {
			tags = append(tags, c.Category)
		}
	}
	return tags
}

// Genres is the list of genres associated with the manga.
func (m *Manga) Genres() []string {
	genres := make([]string, 0, len(m.GenreList))
	for _, g := range m.GenreList {
		if g.Genre != "" {
			genres = append(genres, g.Genre)
		}
	}
	return genres
}

// Characters is the list of characters, MangaUpdates has none.
func (m *Manga) Characters() []string {
	return nil
}

func (m *Manga) authorsOfType(t string) []string {
	var names []string
	for _, a := range m.AuthorList {
		if a.Type == t && a.Name != "" && !slices.Contains(names, a.Name) {
			names = append(names, a.Name)
		}
	}
	return names
}

// Authors (or Writers) is the list of authors, in order of relevance.
// Must contain at least one author.
func (m *Manga) Authors() []string {
	return m.authorsOfType("Author")
}

// Artists is the list of artists, in order of relevance.
func (m *Manga) Artists() []string {
	return m.authorsOfType("Artist")
}

// Translators is the list of translators, MangaUpdates has none.
func (m *Manga) Translators() []string {
	return nil
}

// Letterers is the list of letterers, MangaUpdates has none.
func (m *Manga) Letterers() []string {
	return nil
}

// StartDate is the date the manga started publishing, only its year.
// Must be non-zero.
func (m *Manga) StartDate() metadata.Date {
	year, _ := strconv.Atoi(strings.TrimSpace(m.Year))
	return metadata.Date{Year: year}
}

// EndDate is the date the manga ended publishing, MangaUpdates doesn't say.
func (m *Manga) EndDate() metadata.Date {
	return metadata.Date{}
}

// Publisher of the manga, the original one.
func (m *Manga) Publisher() string {
	for _, p := range m.Publishers {
		if p.Type == "Original" {
			return p.Name
		}
	}
	return ""
}

// Current status of the manga.
// Must be non-empty.
//
// One of: FINISHED, RELEASING, NOT_YET_RELEASED, CANCELLED, HIATUS
//
// From the status text ("115 Volumes (Ongoing)"), else whether it's completed. A search
// result doesn't say: empty.
func (m *Manga) Status() metadata.Status {
	if m.SearchRecord {
		return ""
	}
	status := strings.ToLower(m.StatusText)
	switch {
	case strings.Contains(status, "hiatus"):
		return metadata.StatusHiatus
	case strings.Contains(status, "cancel"), strings.Contains(status, "discontinued"), strings.Contains(status, "axed"):
		return metadata.StatusCancelled
	case strings.Contains(status, "ongoing"):
		return metadata.StatusReleasing
	case m.Completed, strings.Contains(status, "complete"):
		return metadata.StatusFinished
	default:
		return metadata.StatusReleasing
	}
}

// Format the original publication: the series type (Manga, Manhwa, ...).
func (m *Manga) Format() string {
	return m.Type
}

// countries of the series types that tell it.
var countries = map[string]string{
	"Manga":      "JP",
	"Doujinshi":  "JP",
	"Manhwa":     "KR",
	"Manhua":     "CN",
	"Thai":       "TH",
	"Indonesian": "ID",
	"Vietnamese": "VN",
	"Filipino":   "PH",
	"Malaysian":  "MY",
	"French":     "FR",
	"Spanish":    "ES",
	"German":     "DE",
	"Russian":    "RU",
}

// Country of origin of the manga. ISO 3166-1 alpha-2 country code.
//
// Guessed from the series type, empty when it doesn't tell (OEL, Nordic, ...).
func (m *Manga) Country() string {
	return countries[m.Type]
}

// Chapters is the latest chapter.
func (m *Manga) Chapters() int {
	return m.LatestChapter
}

// Notes is empty.
func (m *Manga) Notes() string {
	return ""
}

// URL is the source URL of the metadata.
func (m *Manga) URL() string {
	return m.SeriesURL
}

// ID is the ID information of the metadata: the series id.
func (m *Manga) ID() metadata.ID {
	return metadata.ID{
		Raw:    strconv.FormatInt(m.SeriesID, 10),
		Source: metadata.IDSourceMangaUpdates,
		Code:   metadata.IDCodeMangaUpdates,
	}
}

// ExtraIDs is empty, MangaUpdates doesn't link other sites.
func (m *Manga) ExtraIDs() []metadata.ID {
	return nil
}
