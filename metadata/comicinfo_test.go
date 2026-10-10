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

type koreanMeta struct {
	comicMeta
}

func (m *koreanMeta) Country() string     { return "KR" }
func (m *koreanMeta) AgeRating() string   { return AgeRatingAdultsOnly }
func (m *koreanMeta) Description() string { return "Line one.<br><br>\nLine <i>two</i> &amp; more." }

func TestComicInfoXMLFields(t *testing.T) {
	ci := ToComicInfoXML(&koreanMeta{comicMeta{fakeMeta{id: 1, title: "Solo Leveling"}}}, Chapter{Number: 3, Volume: 2, ScanlationGroup: "Group"})
	options := DefaultComicInfoOptions()
	options.LanguageISO = "en"
	options.AddVolume = true
	out, err := ci.Marshal(options)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	for _, want := range []string{
		"<Manga>Yes</Manga>",
		"<AgeRating>Adults Only 18+</AgeRating>",
		"<ScanInformation>Group</ScanInformation>",
		"<LanguageISO>en</LanguageISO>",
		"<Volume>2</Volume>",
		"<Summary>Line one.&#xA;&#xA;Line two &amp; more.</Summary>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in:\n%s", want, got)
		}
	}

	// the xsd sequence order
	order := []string{"<Title", "<Series>", "<Number>", "<Volume>", "<Summary>", "<Notes>", "<Year>", "<Writer>", "<Translator>", "<Genre>", "<PageCount", "<LanguageISO>", "<Manga>", "<ScanInformation>", "<AgeRating>"}
	last := -1
	for _, tag := range order {
		i := strings.Index(got, tag)
		if i == -1 {
			continue
		}
		if i < last {
			t.Errorf("%s out of order", tag)
		}
		last = i
	}

	// volume is opt in
	out, _ = ci.Marshal(DefaultComicInfoOptions())
	if strings.Contains(string(out), "<Volume>") {
		t.Error("volume written without AddVolume")
	}
}
