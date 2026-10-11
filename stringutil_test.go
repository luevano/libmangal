package libmangal

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		name    string
		windows bool
		want    string
	}{
		{"Chainsaw Man", false, "Chainsaw Man"},
		{"Fate/Zero", false, "Fate_Zero"},
		{"a//b", false, "a_b"},
		{"line\nbreak\ttab", false, "line_break_tab"},
		{"", false, "_"},
		{".", false, "_"},
		{"..", false, "_"},
		{"Kaguya-sama...", false, "Kaguya-sama..."},
		{"Kaguya-sama...", true, "Kaguya-sama"},
		{"CON", true, "_CON"},
		{"con.cbz", true, "_con.cbz"},
		{"Console", true, "Console"},
	}
	for _, tt := range tests {
		if got := SanitizeName(tt.name, "/", tt.windows); got != tt.want {
			t.Errorf("SanitizeName(%q, %t): got %q, want %q", tt.name, tt.windows, got, tt.want)
		}
	}
}

func TestSanitizeNameLength(t *testing.T) {
	long := strings.Repeat("ベ", 200) // 600 bytes
	got := SanitizeName(long, "/", false)
	if len(got) > maxNameBytes || !utf8.ValidString(got) {
		t.Errorf("got %d bytes, valid utf-8 %t", len(got), utf8.ValidString(got))
	}
}
