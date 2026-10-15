package metadata

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestReadSeriesJSON(t *testing.T) {
	want := ToSeriesJSON(&comicMeta{fakeMeta{id: 7, title: "Berserk"}})
	want.ComicImage = "https://s4.anilist.co/file/cover.jpg"
	data, err := want.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	got, err := ReadSeriesJSON(bytes.NewReader(data))
	if err != nil || got != want {
		t.Errorf("got %+v, %v\nwant %+v", got, err, want)
	}

	// another tool's file with a field of another type
	got, err = ReadSeriesJSON(strings.NewReader(`{"metadata": {"name": "Berserk", "year": "1989", "comicid": 7}}`))
	if err != nil || got.Name != "Berserk" || got.ComicID != 7 || got.Year != 0 {
		t.Errorf("lenient: got %+v, %v", got, err)
	}

	if _, err := ReadSeriesJSON(strings.NewReader(`{"metadata": `)); err == nil {
		t.Error("broken json read")
	}
}

func TestReadComicInfoXML(t *testing.T) {
	want := ToComicInfoXML(&koreanMeta{comicMeta{fakeMeta{id: 7, title: "Berserk"}}}, Chapter{
		Title:           "The Black Swordsman",
		URL:             "https://example.com/1",
		Number:          1.5,
		ScanlationGroup: "Some Group",
		Pages:           24,
		Volume:          2,
	})
	want.Notes = "some notes"
	options := DefaultComicInfoOptions()
	options.AddVolume = true
	options.AddDate = true
	data, err := want.Marshal(options)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ReadComicInfoXML(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	// the file only says the reading direction
	want.Country = ""
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}

	// chapter 0 is a number
	zero := ComicInfoXML{Series: "Berserk"}
	data, _ = zero.Marshal(DefaultComicInfoOptions())
	if got, err := ReadComicInfoXML(bytes.NewReader(data)); err != nil || got.NoNumber || got.Number != 0 {
		t.Errorf("chapter 0: got %+v, %v", got, err)
	}

	for doc, want := range map[string]ComicInfoXML{
		`<ComicInfo><Title>t</Title></ComicInfo>`:                                 {Title: "t", NoNumber: true},
		`<ComicInfo><Number> 12 </Number><Genre>A, B,</Genre></ComicInfo>`:        {Number: 12, Genres: []string{"A", "B"}},
		`<ComicInfo><Number>x</Number><Year>soon</Year><Web>u</Web></ComicInfo>`:  {Web: "u", NoNumber: true},
		`<ComicInfo><Number>3</Number><Notes>by another tool</Notes></ComicInfo>`: {Number: 3, Notes: "by another tool"},
	} {
		if got, err := ReadComicInfoXML(strings.NewReader(doc)); err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %+v, %v", doc, got, err)
		}
	}
	if _, err := ReadComicInfoXML(strings.NewReader(`<ComicInfo><Title>`)); err == nil {
		t.Error("broken xml read")
	}
}
