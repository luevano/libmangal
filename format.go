package libmangal

import "strings"

//go:generate enumer -type=Format -trimprefix=Format -json -yaml -text

// Format is the format for saving chapters
type Format uint8

const (
	// FormatPDF saves chapter as a PDF document
	FormatPDF Format = iota + 1

	// FormatImages saves chapter as a directory of plain images
	FormatImages

	// FormatCBZ saves chapter as CBZ archive.
	// CBZ stands for Comic Book Zip format.
	// Common among comic readers
	FormatCBZ

	// FormatTAR saves chapter images as tar archive
	FormatTAR

	// FormatTARGZ saves chapter images tar.gz archive
	FormatTARGZ

	// FormatZIP save chapter images as zip archive
	FormatZIP
)

// Extension returns extension of the format with the leading dot.
func (f Format) Extension() string {
	switch f {
	case FormatPDF:
		return ".pdf"
	case FormatCBZ:
		return ".cbz"
	case FormatTAR:
		return ".tar"
	case FormatTARGZ:
		return ".tar.gz"
	case FormatZIP:
		return ".zip"
	default:
		return ""
	}
}

// FormatOf is the format of a chapter file by its name's extension (any case), the
// inverse of Format.Extension. FormatImages (a directory) has none, so it's never found.
func FormatOf(name string) (Format, bool) {
	lower := strings.ToLower(name)
	// .tar.gz before .tar
	for _, format := range []Format{FormatTARGZ, FormatCBZ, FormatPDF, FormatZIP, FormatTAR} {
		if strings.HasSuffix(lower, format.Extension()) {
			return format, true
		}
	}
	return 0, false
}
