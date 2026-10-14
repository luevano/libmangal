package libmangal

import "testing"

func TestFilterOptionsString(t *testing.T) {
	f := FilterOptions{NSFW: true, Language: "en", ShowUnavailableChapters: true}
	if got, want := f.String(), "filters[true&en&false&true]"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDefaultFilterOptions(t *testing.T) {
	f := DefaultFilterOptions()
	if got, want := f.String(), "filters[true&&false&true]"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
