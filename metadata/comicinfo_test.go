package metadata

import (
	"strings"
	"testing"
)

type comicMeta struct {
	fakeMeta
}

func (m *comicMeta) Authors() []string { return []string{"Kentarou Miura"} }
func (m *comicMeta) Genres() []string  { return []string{"Action", "Drama"} }
func (m *comicMeta) StartDate() Date   { return Date{1989, 8, 25} }

func TestComicInfoXML(t *testing.T) {
	ci := ToComicInfoXML(&comicMeta{fakeMeta{id: 7, title: "Berserk"}}, Chapter{
		Title:           "The Black Swordsman",
		Number:          0,
		ScanlationGroup: "Some Group",
		Pages:           24,
	})
	out, err := ci.Marshal(DefaultComicInfoOptions())
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	for _, want := range []string{
		"<Title>The Black Swordsman</Title>",
		"<Series>Berserk</Series>",
		"<Number>0</Number>", // chapter 0 is kept
		"<Genre>Action,Drama</Genre>",
		"<Writer>Kentarou Miura</Writer>",
		"<Translator>Some Group</Translator>",
		"<PageCount>24</PageCount>",
		"<Year>1989</Year>",
		"<Manga>YesAndRightToLeft</Manga>",
		"<Notes>Downloaded with libmangal&#xA;",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in:\n%s", want, got)
		}
	}

	options := DefaultComicInfoOptions()
	options.AddDate = false
	out, _ = ci.Marshal(options)
	if strings.Contains(string(out), "<Year>") {
		t.Error("date written with AddDate false")
	}
}
