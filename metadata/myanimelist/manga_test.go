package myanimelist

import (
	"testing"

	"github.com/luevano/libmangal/metadata"
)

func TestDateToMetadataDate(t *testing.T) {
	for d, want := range map[date]metadata.Date{
		"":           {},
		"2011":       {Year: 2011},
		"2011-03":    {Year: 2011, Month: 3},
		"2011-03-09": {Year: 2011, Month: 3, Day: 9},
	} {
		if got := d.toMetadataDate(); got != want {
			t.Errorf("%q: got %+v, want %+v", d, got, want)
		}
	}
}

func TestAuthorName(t *testing.T) {
	for a, want := range map[authorListNode]string{
		{FirstName: "Kentarou", LastName: "Miura"}: "Kentarou Miura",
		{LastName: "ONE"}:                          "ONE",
		{FirstName: "Boichi"}:                      "Boichi",
	} {
		if got := a.name(); got != want {
			t.Errorf("%+v: got %q, want %q", a, got, want)
		}
	}
}

func TestStatus(t *testing.T) {
	for s, want := range map[Status]metadata.Status{
		StatusOnHiatus:     metadata.StatusHiatus,
		StatusDiscontinued: metadata.StatusCancelled,
	} {
		if got := (&Manga{PublicationStatus: s}).Status(); got != want {
			t.Errorf("%q: got %q, want %q", s, got, want)
		}
	}
}

func TestCountryAndAgeRating(t *testing.T) {
	m := &Manga{MediaType: "manhwa", NSFW: "black"}
	if m.Country() != "KR" || m.AgeRating() != metadata.AgeRatingAdultsOnly {
		t.Errorf("got %q, %q", m.Country(), m.AgeRating())
	}
	if (&Manga{MediaType: "one_shot", NSFW: "white"}).AgeRating() != "" {
		t.Error("white is rated")
	}
}
