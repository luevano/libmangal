package metadata

import "testing"

func TestMangaUpdatesID(t *testing.T) {
	for _, tt := range []struct {
		urlID string
		raw   string
		ok    bool
	}{
		{"pb8uwds", "55099564912", true}, // One Piece, https://www.mangaupdates.com/series/pb8uwds
		{"PB8UWDS", "55099564912", true},
		{"33", "", false},     // the old series.html?id=33, only the website knows it
		{"153558", "", false}, // old
		{"", "", false},
		{"pb8-uwds", "", false},
		{"zzzzzzzzzzzzzzz", "", false}, // overflows
	} {
		id, ok := MangaUpdatesID(tt.urlID)
		if ok != tt.ok || id.Raw != tt.raw {
			t.Errorf("%q: got %+v, %t", tt.urlID, id, ok)
			continue
		}
		if ok {
			if err := id.validate(); err != nil || id.Source != IDSourceMangaUpdates || id.Code != IDCodeMangaUpdates {
				t.Errorf("%q: %+v, %v", tt.urlID, id, err)
			}
		}
	}
}
