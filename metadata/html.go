package metadata

import (
	"html"
	"regexp"
	"strings"
)

var (
	htmlBreaks = regexp.MustCompile(`(?i)<br\s*/?>`)
	htmlTags   = regexp.MustCompile(`<[^>]*>`)
	blankLines = regexp.MustCompile(`\n{3,}`)
)

// StripHTML turns a description with html (e.g. AniList's <br> and <i>) into plain text.
func StripHTML(s string) string {
	s = htmlBreaks.ReplaceAllString(s, "\n")
	s = htmlTags.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	return strings.TrimSpace(blankLines.ReplaceAllString(s, "\n\n"))
}
