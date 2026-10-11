package kitsu

import (
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/luevano/libmangal/metadata"
)

const mangaURL = "https://kitsu.app/manga/"

var (
	_ metadata.Metadata = (*Manga)(nil)
	_ metadata.AgeRated = (*Manga)(nil)
)

// maxCharacters is how many main characters are kept, like AniList's.
const maxCharacters = 10

// Kitsu values of Manga.Subtype, Manga.KitsuStatus and Manga.AgeRatingCode.
const (
	SubtypeDoujin  = "doujin"
	SubtypeManga   = "manga"
	SubtypeManhua  = "manhua"
	SubtypeManhwa  = "manhwa"
	SubtypeNovel   = "novel"
	SubtypeOEL     = "oel"
	SubtypeOneshot = "oneshot"

	StatusCurrent    = "current"
	StatusFinished   = "finished"
	StatusTBA        = "tba"
	StatusUnreleased = "unreleased"
	StatusUpcoming   = "upcoming"

	AgeRatingG   = "G"
	AgeRatingPG  = "PG"
	AgeRatingR   = "R"
	AgeRatingR18 = "R18"
)

// Image is a Kitsu image in its available sizes.
type Image struct {
	Tiny     string `json:"tiny,omitempty"`
	Small    string `json:"small,omitempty"`
	Medium   string `json:"medium,omitempty"`
	Large    string `json:"large,omitempty"`
	Original string `json:"original,omitempty"`
}

// Largest is the largest available size, empty if none.
func (i Image) Largest() string {
	for _, s := range []string{i.Original, i.Large, i.Medium, i.Small, i.Tiny} {
		if s != "" {
			return s
		}
	}
	return ""
}

// Staff is a person that worked on the manga.
type Staff struct {
	Name string `json:"name"`
	// Role, e.g. "Story & Art", "Story", "Art".
	Role string `json:"role"`
}

// Mapping is the manga's id on another site.
type Mapping struct {
	// Site, e.g. "anilist/manga", "myanimelist/manga", "mangaupdates".
	Site string `json:"site"`
	ID   string `json:"id"`
}

// Manga is a metadata.Metadata implementation
// for Kitsu manga metadata.
//
// The JSON:API resource and its included relationships (categories, staff,
// main characters, mappings) are flattened into it. Note that Manga fields
// don't match the method names to avoid collisions with the interface.
type Manga struct {
	IDProvider int    `json:"id" jsonschema:"description=ID of the manga on Kitsu."`
	Slug       string `json:"slug" jsonschema:"description=Unique URL-safe identifier."`
	// CanonicalTitle is the most commonly used title, often the romanized one.
	CanonicalTitle string `json:"canonicalTitle" jsonschema:"description=The most commonly used title."`
	// TitleMap are the localised titles, keyed by locale (en, en_us, en_jp, ja_jp, ...).
	TitleMap          map[string]string `json:"titles" jsonschema:"description=Localised titles keyed by locale."`
	AbbreviatedTitles []string          `json:"abbreviatedTitles" jsonschema:"description=Abbreviations or other names the manga is known by."`
	Synopsis          string            `json:"synopsis" jsonschema:"description=Synopsis of the manga, plain text."`
	// AverageRating is the 0-100 bayesian average, 0 if unrated.
	AverageRating float32       `json:"averageRating" jsonschema:"description=Bayesian average rating, 0-100."`
	DateStart     metadata.Date `json:"startDate" jsonschema:"description=Date the manga started publishing."`
	DateEnd       metadata.Date `json:"endDate" jsonschema:"description=Date the manga ended publishing."`
	AgeRatingCode string        `json:"ageRating" jsonschema:"enum=G,enum=PG,enum=R,enum=R18"`
	Subtype       string        `json:"subtype" jsonschema:"enum=doujin,enum=manga,enum=manhua,enum=manhwa,enum=novel,enum=oel,enum=oneshot"`
	KitsuStatus   string        `json:"status" jsonschema:"enum=current,enum=finished,enum=tba,enum=unreleased,enum=upcoming"`
	PosterImage   Image         `json:"posterImage"`
	CoverImage    Image         `json:"coverImage"`
	ChapterCount  int           `json:"chapterCount"`
	VolumeCount   int           `json:"volumeCount"`
	Serialization string        `json:"serialization" jsonschema:"description=Magazine the manga was published in."`
	Categories    []string      `json:"categories" jsonschema:"description=Categories (genres and themes) of the manga."`
	Staff         []Staff       `json:"staff"`
	CharacterList []string      `json:"characters" jsonschema:"description=Main characters of the manga."`
	Mappings      []Mapping     `json:"mappings" jsonschema:"description=IDs of the manga on other sites."`
}

// mangaFields are the manga attributes (and relationships) requested.
var mangaFields = strings.Join([]string{
	"slug", "titles", "canonicalTitle", "abbreviatedTitles", "synopsis", "averageRating",
	"startDate", "endDate", "ageRating", "subtype", "status", "posterImage", "coverImage",
	"chapterCount", "volumeCount", "serialization",
	"categories", "staff", "characters", "mappings",
}, ",")

// mangaParams are the query parameters to get mangas with their relationships.
func mangaParams() url.Values {
	return url.Values{
		"include":                 {"categories,staff.person,characters.character,mappings"},
		"fields[manga]":           {mangaFields},
		"fields[categories]":      {"title"},
		"fields[mediaStaff]":      {"role,person"},
		"fields[people]":          {"name"},
		"fields[mediaCharacters]": {"role,character"},
		"fields[characters]":      {"canonicalName"},
		"fields[mappings]":        {"externalSite,externalId"},
	}
}

type mangaAttributes struct {
	Slug              string            `json:"slug"`
	Titles            map[string]string `json:"titles"`
	CanonicalTitle    string            `json:"canonicalTitle"`
	AbbreviatedTitles []string          `json:"abbreviatedTitles"`
	Synopsis          string            `json:"synopsis"`
	AverageRating     *string           `json:"averageRating"`
	StartDate         *string           `json:"startDate"`
	EndDate           *string           `json:"endDate"`
	AgeRating         *string           `json:"ageRating"`
	Subtype           string            `json:"subtype"`
	Status            string            `json:"status"`
	PosterImage       *Image            `json:"posterImage"`
	CoverImage        *Image            `json:"coverImage"`
	ChapterCount      *int              `json:"chapterCount"`
	VolumeCount       *int              `json:"volumeCount"`
	Serialization     *string           `json:"serialization"`
}

// mangas builds the mangas of a document.
func (d document) mangas() ([]*Manga, error) {
	resources, err := d.resources()
	if err != nil {
		return nil, err
	}
	idx := d.index()
	mangas := make([]*Manga, 0, len(resources))
	for _, r := range resources {
		m, err := newManga(r, idx)
		if err != nil {
			return nil, err
		}
		mangas = append(mangas, m)
	}
	return mangas, nil
}

func newManga(r resource, idx includedIndex) (*Manga, error) {
	id, err := strconv.Atoi(r.ID)
	if err != nil {
		return nil, fmt.Errorf("manga id %q isn't an integer", r.ID)
	}
	var a mangaAttributes
	if err := json.Unmarshal(r.Attributes, &a); err != nil {
		return nil, fmt.Errorf("manga %d: %w", id, err)
	}

	m := &Manga{
		IDProvider:        id,
		Slug:              a.Slug,
		CanonicalTitle:    a.CanonicalTitle,
		TitleMap:          a.Titles,
		AbbreviatedTitles: a.AbbreviatedTitles,
		Synopsis:          a.Synopsis,
		DateStart:         parseDate(deref(a.StartDate)),
		DateEnd:           parseDate(deref(a.EndDate)),
		AgeRatingCode:     deref(a.AgeRating),
		Subtype:           a.Subtype,
		KitsuStatus:       a.Status,
		ChapterCount:      deref(a.ChapterCount),
		VolumeCount:       deref(a.VolumeCount),
		Serialization:     deref(a.Serialization),
	}
	if a.AverageRating != nil {
		rating, _ := strconv.ParseFloat(*a.AverageRating, 32)
		m.AverageRating = float32(rating)
	}
	if a.PosterImage != nil {
		m.PosterImage = *a.PosterImage
	}
	if a.CoverImage != nil {
		m.CoverImage = *a.CoverImage
	}

	for _, c := range idx.related(r, "categories") {
		var attrs struct {
			Title string `json:"title"`
		}
		if json.Unmarshal(c.Attributes, &attrs) == nil && attrs.Title != "" {
			m.Categories = append(m.Categories, attrs.Title)
		}
	}
	for _, s := range idx.related(r, "staff") {
		var attrs struct {
			Role string `json:"role"`
		}
		_ = json.Unmarshal(s.Attributes, &attrs)
		for _, person := range idx.related(s, "person") {
			var p struct {
				Name string `json:"name"`
			}
			if json.Unmarshal(person.Attributes, &p) == nil && p.Name != "" {
				m.Staff = append(m.Staff, Staff{Name: p.Name, Role: attrs.Role})
			}
		}
	}
	for _, c := range idx.related(r, "characters") {
		var attrs struct {
			Role string `json:"role"`
		}
		_ = json.Unmarshal(c.Attributes, &attrs)
		if attrs.Role != "main" || len(m.CharacterList) == maxCharacters {
			continue
		}
		for _, character := range idx.related(c, "character") {
			var ch struct {
				CanonicalName string `json:"canonicalName"`
			}
			if json.Unmarshal(character.Attributes, &ch) == nil && ch.CanonicalName != "" {
				m.CharacterList = append(m.CharacterList, ch.CanonicalName)
			}
		}
	}
	for _, mp := range idx.related(r, "mappings") {
		var attrs struct {
			ExternalSite string `json:"externalSite"`
			ExternalID   string `json:"externalId"`
		}
		if json.Unmarshal(mp.Attributes, &attrs) == nil && attrs.ExternalSite != "" {
			m.Mappings = append(m.Mappings, Mapping{Site: attrs.ExternalSite, ID: attrs.ExternalID})
		}
	}
	return m, nil
}

func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}

// parseDate parses a YYYY-MM-DD date, or the leading parts of it.
func parseDate(s string) metadata.Date {
	var d metadata.Date
	parts := strings.SplitN(s, "-", 3)
	for i, dst := range []*int{&d.Year, &d.Month, &d.Day} {
		if i >= len(parts) {
			break
		}
		n, err := strconv.Atoi(parts[i])
		if err != nil {
			break
		}
		*dst = n
	}
	return d
}

// String is the short representation of the manga.
// Must be non-empty.
//
// At the minimum it should return "`Title` (`Year`)", else
// "`Title` (`Year`) [`IDCode`id-`ID`]" if available.
func (m *Manga) String() string {
	base := m.Title() + " (" + strconv.Itoa(m.StartDate().Year)
	if m.IDProvider == 0 {
		return base + ")"
	}
	return base + ") [" + string(metadata.IDCodeKitsu) + "id-" + strconv.Itoa(m.IDProvider) + "]"
}

// englishLocales are the English title locales, in order of preference.
var englishLocales = []string{"en", "en_us", "en_gb"}

// romanizedLocales and nativeLocales are the original titles, in order of preference.
var (
	romanizedLocales = []string{"en_jp", "en_kr", "en_cn", "en_tw"}
	nativeLocales    = []string{"ja_jp", "ko_kr", "zh_cn", "zh_tw", "zh_hk"}
)

func (m *Manga) title(locales []string) string {
	for _, l := range locales {
		if t := strings.TrimSpace(m.TitleMap[l]); t != "" {
			return t
		}
	}
	return ""
}

// Title is the English title of the manga.
// Must be non-empty.
//
// If English is not available, then in order of availability:
// the canonical title (usually romanized), romanized or native.
func (m *Manga) Title() string {
	for _, t := range []string{
		m.title(englishLocales),
		strings.TrimSpace(m.CanonicalTitle),
		m.title(romanizedLocales),
		m.title(nativeLocales),
	} {
		if t != "" {
			return t
		}
	}
	return ""
}

// AlternateTitles is a list of alternative titles in order of relevance.
func (m *Manga) AlternateTitles() []string {
	ordered := []string{m.CanonicalTitle}
	for _, group := range [][]string{englishLocales, romanizedLocales, nativeLocales} {
		for _, l := range group {
			ordered = append(ordered, m.TitleMap[l])
		}
	}
	// the other locales, sorted so the order is stable
	var others []string
	for l := range m.TitleMap {
		if !slices.Contains(englishLocales, l) && !slices.Contains(romanizedLocales, l) && !slices.Contains(nativeLocales, l) {
			others = append(others, l)
		}
	}
	sort.Strings(others)
	for _, l := range others {
		ordered = append(ordered, m.TitleMap[l])
	}
	ordered = append(ordered, m.AbbreviatedTitles...)

	title := m.Title()
	seen := map[string]bool{title: true}
	var titles []string
	for _, t := range ordered {
		t = strings.TrimSpace(t)
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		titles = append(titles, t)
	}
	return titles
}

// Score is the community score for the manga.
//
// Accepted values are between 0.0 and 5.0.
func (m *Manga) Score() float32 {
	return m.AverageRating / 20
}

// Description is the description/summary for the manga.
func (m *Manga) Description() string {
	return m.Synopsis
}

// Cover is the cover image of the manga.
func (m *Manga) Cover() string {
	return m.PosterImage.Largest()
}

// Banner is the banner image of the manga.
func (m *Manga) Banner() string {
	// what Kitsu calls the cover is the banner
	return m.CoverImage.Largest()
}

// Tags is the list of tags associated with the manga.
//
// Kitsu doesn't tell genres and tags apart, all its categories are in Genres.
func (m *Manga) Tags() []string {
	return []string{}
}

// Genres is the list of genres associated with the manga.
func (m *Manga) Genres() []string {
	return m.Categories
}

// Characters is the list of characters, in order of relevance.
func (m *Manga) Characters() []string {
	return m.CharacterList
}

func (m *Manga) staffWith(roles ...string) []string {
	var names []string
	for _, s := range m.Staff {
		role := strings.ToLower(s.Role)
		for _, r := range roles {
			if strings.Contains(role, r) {
				names = append(names, s.Name)
				break
			}
		}
	}
	return names
}

// Authors (or Writers) is the list of authors, in order of relevance.
// Must contain at least one author.
func (m *Manga) Authors() []string {
	return m.staffWith("story", "author", "original creator")
}

// Artists is the list of artists, in order of relevance.
func (m *Manga) Artists() []string {
	return m.staffWith("art")
}

// Translators is the list of translators, in order of relevance.
func (m *Manga) Translators() []string {
	return m.staffWith("translat")
}

// Letterers is the list of letterers, in order of relevance.
func (m *Manga) Letterers() []string {
	return m.staffWith("letter")
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
//
// Kitsu only has the magazine (Serialization), not the publisher.
func (m *Manga) Publisher() string {
	return ""
}

// Current status of the manga.
// Must be non-empty.
//
// One of: FINISHED, RELEASING, NOT_YET_RELEASED, CANCELLED, HIATUS
func (m *Manga) Status() metadata.Status {
	switch m.KitsuStatus {
	case StatusCurrent:
		return metadata.StatusReleasing
	case StatusFinished:
		return metadata.StatusFinished
	case StatusTBA, StatusUnreleased, StatusUpcoming:
		return metadata.StatusNotYetReleased
	}
	return ""
}

// Format the original publication.
//
// For example: TBP, HC, Web, Digital, etc..
func (m *Manga) Format() string {
	return ""
}

// Country of origin of the manga. ISO 3166-1 alpha-2 country code.
//
// Kitsu doesn't have it, it's guessed from the subtype and the titles.
func (m *Manga) Country() string {
	switch m.Subtype {
	case SubtypeManga, SubtypeDoujin:
		return "JP"
	case SubtypeManhwa:
		return "KR"
	case SubtypeManhua:
		return "CN"
	}
	for _, guess := range []struct{ locales, country string }{
		{"ja_jp", "JP"}, {"en_jp", "JP"}, {"ko_kr", "KR"}, {"en_kr", "KR"}, {"zh_cn", "CN"}, {"en_cn", "CN"}, {"zh_tw", "TW"}, {"en_tw", "TW"},
	} {
		if m.TitleMap[guess.locales] != "" {
			return guess.country
		}
	}
	return ""
}

// AgeRating is the ComicInfo.xml age rating, only known for adult mangas.
func (m *Manga) AgeRating() string {
	if m.AgeRatingCode == AgeRatingR18 {
		return metadata.AgeRatingAdultsOnly
	}
	return ""
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
	if m.Slug == "" {
		return mangaURL + strconv.Itoa(m.IDProvider)
	}
	return mangaURL + m.Slug
}

// ID is the ID information of the metadata.
// Must be valid (ID.Validate).
func (m *Manga) ID() metadata.ID {
	return metadata.ID{
		Raw:    strconv.Itoa(m.IDProvider),
		Source: metadata.IDSourceKitsu,
		Code:   metadata.IDCodeKitsu,
	}
}

// ExtraIDs is a list of extra available IDs in the metadata provider.
// Each extra ID must be valid (ID.Validate).
//
// The AniList and MyAnimeList mappings, MangaUpdates ones are left out (Kitsu
// mixes its old numeric ids and the new base36 ones).
func (m *Manga) ExtraIDs() []metadata.ID {
	ids := []metadata.ID{}
	seen := map[metadata.IDSource]bool{}
	for _, mp := range m.Mappings {
		var source metadata.IDSource
		var code metadata.IDCode
		switch mp.Site {
		case "anilist/manga":
			source, code = metadata.IDSourceAnilist, metadata.IDCodeAnilist
		case "myanimelist/manga":
			source, code = metadata.IDSourceMyAnimeList, metadata.IDCodeMyAnimeList
		default:
			continue
		}
		if n, err := strconv.Atoi(mp.ID); err != nil || n < 1 || seen[source] {
			continue
		}
		seen[source] = true
		ids = append(ids, metadata.ID{Raw: mp.ID, Source: source, Code: code})
	}
	return ids
}
