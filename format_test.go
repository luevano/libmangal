package libmangal

import "testing"

func TestFormatOf(t *testing.T) {
	for _, format := range FormatValues() {
		if format == FormatImages {
			continue
		}
		if got, ok := FormatOf("Chapter 1" + format.Extension()); !ok || got != format {
			t.Errorf("%s: got %s, %v", format, got, ok)
		}
	}
	for name, want := range map[string]Format{
		"[001] ch.TAR.GZ": FormatTARGZ,
		"a.tar":           FormatTAR,
		"b.Cbz":           FormatCBZ,
	} {
		if got, ok := FormatOf(name); !ok || got != want {
			t.Errorf("%q: got %s, %v", name, got, ok)
		}
	}
	for _, name := range []string{"Chapter 1", "cover.jpg", "a.gz", ""} {
		if got, ok := FormatOf(name); ok {
			t.Errorf("%q: got %s", name, got)
		}
	}
}
