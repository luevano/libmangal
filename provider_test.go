package libmangal

import (
	"context"
	"testing"
)

type infoLoader struct{ info ProviderInfo }

func (l infoLoader) String() string                         { return l.info.Name }
func (l infoLoader) Info() ProviderInfo                     { return l.info }
func (l infoLoader) Load(context.Context) (Provider, error) { return nil, nil }

func TestFindLoader(t *testing.T) {
	weebCentral := infoLoader{ProviderInfo{
		ID: "mango-weebcentral", Name: "Weeb Central", Version: "0.1.0",
		History: []ProviderAlias{{ID: "mango-mangasee", Name: "MangaSee", Website: "https://mangasee123.com/"}},
	}}
	// a later provider reusing the old id wins over the alias
	reused := infoLoader{ProviderInfo{ID: "mango-reused", Name: "Reused", Version: "0.1.0", History: []ProviderAlias{{ID: "mango-old"}}}}
	old := infoLoader{ProviderInfo{ID: "mango-old", Name: "Old", Version: "0.1.0"}}
	loaders := []ProviderLoader{weebCentral, reused, old}

	tests := []struct {
		id        string
		wantName  string
		wantAlias string
	}{
		{"mango-weebcentral", "Weeb Central", ""},
		{"mango-mangasee", "Weeb Central", "MangaSee"},
		{"mango-old", "Old", ""},
	}
	for _, tt := range tests {
		loader, alias, ok := FindLoader(loaders, tt.id)
		if !ok || loader.Info().Name != tt.wantName {
			t.Errorf("%s: got %v, %t, want %s", tt.id, loader, ok, tt.wantName)
			continue
		}
		gotAlias := ""
		if alias != nil {
			gotAlias = alias.Name
		}
		if gotAlias != tt.wantAlias {
			t.Errorf("%s: alias %q, want %q", tt.id, gotAlias, tt.wantAlias)
		}
	}
	if _, _, ok := FindLoader(loaders, "nope"); ok {
		t.Error("found an unknown id")
	}
}

func TestProviderInfoMatches(t *testing.T) {
	// same id, only the name changed
	info := ProviderInfo{ID: "mango-mangasee", Name: "Weeb Central", Version: "0.1.0", History: []ProviderAlias{{ID: "mango-mangasee", Name: "MangaSee"}}}
	if !info.Matches("mango-mangasee") {
		t.Error("doesn't match its own id")
	}
	if _, ok := info.Alias("mango-mangasee"); ok {
		t.Error("the current id is reported as an alias")
	}
	if err := (ProviderInfo{ID: "x", Name: "X", Version: "0.1.0", History: []ProviderAlias{{Name: "Y"}}}).Validate(); err == nil {
		t.Error("history entry without an id is valid")
	}
}

func TestProviderInfoValidateMirrors(t *testing.T) {
	info := ProviderInfo{ID: "x", Name: "X", Version: "0.1.0", Mirrors: []string{"https://mirror.example/"}}
	if err := info.Validate(); err != nil {
		t.Errorf("valid mirror: %v", err)
	}
	for _, bad := range []string{"", "mirror.example", "ftp://mirror.example/", "https://"} {
		info.Mirrors = []string{bad}
		if err := info.Validate(); err == nil {
			t.Errorf("mirror %q: no error", bad)
		}
	}
}

func TestProviderInfoValidateIcon(t *testing.T) {
	info := ProviderInfo{ID: "x", Name: "X", Version: "0.1.0"}
	for _, good := range []string{"", "https://site.example/icon.png"} {
		info.Icon = good
		if err := info.Validate(); err != nil {
			t.Errorf("icon %q: %v", good, err)
		}
	}
	for _, bad := range []string{"icon.png", "ftp://site.example/icon.png", "https://"} {
		info.Icon = bad
		if err := info.Validate(); err == nil {
			t.Errorf("icon %q: no error", bad)
		}
	}
}
