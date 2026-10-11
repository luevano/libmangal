package libmangal

import (
	"regexp"
	"runtime"
	"strings"
	"unicode/utf8"
)

// maxNameBytes keeps names under the usual 255 bytes limit with room for
// the extension (".tar.gz") and the temporary suffix (".part").
const maxNameBytes = 240

var underscores = regexp.MustCompile(`_+`)

// windowsReserved are names Windows doesn't allow for files, with or without an extension.
var windowsReserved = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true, "COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true, "LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
}

func sanitizePath(path string) string {
	return SanitizeName(path, invalidPathChars, runtime.GOOS == "windows")
}

// SanitizeName makes name usable as a single file or directory name: control chars and
// the invalid ones become underscores (repeated ones collapse), it's capped to 240 bytes,
// and it's never empty, "." or "..". With windows, trailing dots and spaces go and
// reserved names (CON, NUL, ...) get a "_" prefix.
func SanitizeName(name, invalid string, windows bool) string {
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || strings.ContainsRune(invalid, r) {
			return '_'
		}
		return r
	}, name)

	// replace two or more consecutive underscores with one underscore
	name = underscores.ReplaceAllString(name, "_")

	if len(name) > maxNameBytes {
		name = name[:maxNameBytes]
		// don't leave half a character
		for !utf8.ValidString(name) {
			name = name[:len(name)-1]
		}
	}

	if windows {
		// Windows drops trailing dots and spaces
		name = strings.TrimRight(name, ". ")
		base, _, _ := strings.Cut(name, ".")
		if windowsReserved[strings.ToUpper(strings.TrimSpace(base))] {
			name = "_" + name
		}
	}

	switch name {
	case "", ".", "..":
		return "_"
	}
	return name
}
