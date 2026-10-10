package metadata

import (
	"slices"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// GoodMatchScore is the lowest Match.Score used automatically.
const GoodMatchScore = 0.85

// yearPenalty is taken from the score when the start years differ, small enough to
// never make a good match bad.
const yearPenalty = 0.01

// Query describes the manga a metadata search is for.
type Query struct {
	// Titles to search with and compare to, most relevant first
	// (e.g. the provider's title, then its alternate titles).
	Titles []string

	// Year the manga started publishing, 0 if unknown.
	Year int
}

// Match is a metadata search result scored against a Query.
type Match struct {
	Metadata Metadata

	// Score from 0 (nothing in common) to 1 (same title).
	Score float64

	// Bound is true when the result comes from a title bound by the user
	// (ProviderWithCache.BindTitleWithID) or an id given by the provider.
	Bound bool
}

// Bad reports whether the match is too unlike the query to be used automatically.
// It can still be shown and picked by the user.
func (m Match) Bad() bool {
	return !m.Bound && m.Score < GoodMatchScore
}

// Rank scores the metadatas against the query, best first. Ties keep the given order
// (the provider's search relevance).
func Rank(q Query, metas []Metadata) []Match {
	normalized := normalizeAll(q.Titles)
	matches := make([]Match, len(metas))
	for i, m := range metas {
		matches[i] = Match{Metadata: m, Score: score(normalized, q.Year, m)}
	}
	slices.SortStableFunc(matches, func(a, b Match) int {
		switch {
		case a.Score > b.Score:
			return -1
		case a.Score < b.Score:
			return 1
		}
		return 0
	})
	return matches
}

// Score is how well the metadata matches the query, from 0 to 1.
func Score(q Query, m Metadata) float64 {
	return score(normalizeAll(q.Titles), q.Year, m)
}

func score(queryTitles []string, year int, m Metadata) float64 {
	best := 0.0
	for _, title := range normalizeAll(append([]string{m.Title()}, m.AlternateTitles()...)) {
		for _, query := range queryTitles {
			best = max(best, similarity(query, title))
		}
	}
	// only a tie breaker between same titles: providers' years aren't always the
	// start year (MangaPlus' One Piece isn't 1997)
	if start := m.StartDate().Year; year != 0 && start != 0 && (year-start > 1 || start-year > 1) {
		best -= yearPenalty
	}
	return max(best, 0)
}

func normalizeAll(titles []string) []string {
	var normalized []string
	for _, t := range titles {
		if n := normalize(t); n != "" {
			normalized = append(normalized, n)
		}
	}
	return normalized
}

// normalize keeps only letters and digits, lowercased and in their compatibility form
// (full-width to half-width), so "Kaguya-sama: Love Is War" equals "kaguya sama love is war".
func normalize(title string) string {
	var sb strings.Builder
	for _, r := range norm.NFKC.String(title) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			sb.WriteRune(unicode.ToLower(r))
		}
	}
	return sb.String()
}

// similarity is 1 minus the edit distance relative to the longer string.
func similarity(a, b string) float64 {
	ra, rb := []rune(a), []rune(b)
	longest := max(len(ra), len(rb))
	if longest == 0 {
		return 0
	}
	return 1 - float64(levenshtein(ra, rb))/float64(longest)
}

func levenshtein(a, b []rune) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}
