package libmangal

import "testing"

func TestFilterOptionsString(t *testing.T) {
	f := FilterOptions{NSFW: true, Language: "en", ShowUnavailableChapters: true}
	if got, want := f.String(), "filters[true&en&false&true]"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
