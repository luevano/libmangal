package metadata

import (
	"context"
	"testing"
)

type titledMeta struct {
	fakeMeta
	alt  []string
	year int
}

func (m *titledMeta) AlternateTitles() []string { return m.alt }
func (m *titledMeta) StartDate() Date           { return Date{Year: m.year} }

func TestScore(t *testing.T) {
	tests := []struct {
		name  string
		query Query
		meta  Metadata
		good  bool
	}{
		{"same title", Query{Titles: []string{"Berserk"}}, &titledMeta{fakeMeta: fakeMeta{title: "Berserk"}}, true},
		{"punctuation and case", Query{Titles: []string{"Kaguya-sama: Love Is War"}}, &titledMeta{fakeMeta: fakeMeta{title: "kaguya sama love is war"}}, true},
		{"full width", Query{Titles: []string{"ＢＥＲＳＥＲＫ"}}, &titledMeta{fakeMeta: fakeMeta{title: "Berserk"}}, true},
		{"alternate title", Query{Titles: []string{"Ore dake Level Up na Ken"}}, &titledMeta{fakeMeta: fakeMeta{title: "Solo Leveling"}, alt: []string{"Ore dake Level Up na Ken"}}, true},
		{"query alternate title", Query{Titles: []string{"Only I Level Up", "Solo Leveling"}}, &titledMeta{fakeMeta: fakeMeta{title: "Solo Leveling"}}, true},
		{"aside", Query{Titles: []string{"KAIJU NO.8 (Monster #8)"}}, &titledMeta{fakeMeta: fakeMeta{title: "Kaiju No. 8"}}, true},
		{"spin-off", Query{Titles: []string{"Kaiju No. 8"}}, &titledMeta{fakeMeta: fakeMeta{title: "Kaiju No. 8: B-Side"}}, false},
		{"other work", Query{Titles: []string{"Berserk"}}, &titledMeta{fakeMeta: fakeMeta{title: "Berserk of Gluttony"}}, false},
		{"years apart", Query{Titles: []string{"One Piece"}, Year: 2019}, &titledMeta{fakeMeta: fakeMeta{title: "ONE PIECE"}, year: 1997}, true},
		{"close year", Query{Titles: []string{"Hunter x Hunter"}, Year: 1999}, &titledMeta{fakeMeta: fakeMeta{title: "Hunter x Hunter"}, year: 1998}, true},
	}
	for _, tt := range tests {
		score := Score(tt.query, tt.meta)
		if good := score >= GoodMatchScore; good != tt.good {
			t.Errorf("%s: score %.2f, want good=%t", tt.name, score, tt.good)
		}
	}
}

func TestRankPrefersTheYear(t *testing.T) {
	remake := &titledMeta{fakeMeta: fakeMeta{id: 1, title: "Hunter x Hunter"}, year: 2011}
	original := &titledMeta{fakeMeta: fakeMeta{id: 2, title: "Hunter x Hunter"}, year: 1998}
	ranked := Rank(Query{Titles: []string{"Hunter x Hunter"}, Year: 1998}, []Metadata{remake, original})
	if ranked[0].Metadata != original || ranked[1].Bad() {
		t.Errorf("got %+v", ranked)
	}
}

func TestRankKeepsOrderOnTies(t *testing.T) {
	a := &titledMeta{fakeMeta: fakeMeta{id: 1, title: "Berserk"}}
	b := &titledMeta{fakeMeta: fakeMeta{id: 2, title: "Berserk"}}
	spinoff := &titledMeta{fakeMeta: fakeMeta{id: 3, title: "Berserk of Gluttony"}}
	ranked := Rank(Query{Titles: []string{"Berserk"}}, []Metadata{spinoff, a, b})
	if ranked[0].Metadata != a || ranked[1].Metadata != b || ranked[2].Metadata != spinoff {
		t.Errorf("got %v", ranked)
	}
}

func TestFindClosest(t *testing.T) {
	spinoff := &titledMeta{fakeMeta: fakeMeta{id: 3, title: "Kaiju No. 8: B-Side"}}
	kaiju := &titledMeta{fakeMeta: fakeMeta{id: 8, title: "Kaiju No. 8"}}

	t.Run("good match on an alternate title", func(t *testing.T) {
		fake := &fakeProvider{byQuery: map[string][]Metadata{
			"Kaiju Number Eight": {spinoff},
			"Kaiju No. 8":        {spinoff, kaiju},
		}}
		p := newTestProvider(t, fake)
		match, found, err := p.FindClosest(Query{Titles: []string{"Kaiju Number Eight", "Kaiju No. 8"}})
		if err != nil || !found || match.Metadata != kaiju || match.Bad() {
			t.Fatalf("got %+v found=%t err=%v, want %v", match, found, err, kaiju)
		}
		// remembered for the first title
		if id, ok, _ := p.store.getTitleID("Kaiju Number Eight"); !ok || id != 8 {
			t.Errorf("binding %d %t, want 8", id, ok)
		}
	})

	t.Run("bad match is returned but not remembered", func(t *testing.T) {
		fake := &fakeProvider{byQuery: map[string][]Metadata{"Kaiju No. 8": {spinoff}}}
		p := newTestProvider(t, fake)
		match, found, err := p.FindClosestContext(context.Background(), Query{Titles: []string{"Kaiju No. 8"}})
		if err != nil || !found || match.Metadata != spinoff || !match.Bad() {
			t.Fatalf("got %+v found=%t err=%v, want the spin-off as a bad match", match, found, err)
		}
		if _, ok, _ := p.store.getTitleID("Kaiju No. 8"); ok {
			t.Error("bad match was bound to the title")
		}
	})

	t.Run("searches the title without its aside", func(t *testing.T) {
		fake := &fakeProvider{byQuery: map[string][]Metadata{"KAIJU NO.8": {kaiju}}}
		p := newTestProvider(t, fake)
		match, found, err := p.FindClosest(Query{Titles: []string{"KAIJU NO.8 (Monster #8)"}})
		if err != nil || !found || match.Metadata != kaiju || match.Bad() {
			t.Fatalf("got %+v found=%t err=%v, searched %q", match, found, err, fake.queries)
		}
	})

	t.Run("searches at most maxMatchSearches titles", func(t *testing.T) {
		fake := &fakeProvider{}
		p := newTestProvider(t, fake)
		_, found, err := p.FindClosest(Query{Titles: []string{"a", "", "b", "c", "d"}})
		if err != nil || found {
			t.Fatalf("got found=%t err=%v", found, err)
		}
		if len(fake.queries) != maxMatchSearches {
			t.Errorf("searched %q", fake.queries)
		}
	})
}
