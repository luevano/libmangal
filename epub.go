package libmangal

import (
	"archive/zip"
	"bytes"
	"crypto/sha1"
	"encoding/xml"
	"fmt"
	"hash/crc32"
	"image"
	"io"
	"strconv"
	"strings"
	"time"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	_ "golang.org/x/image/webp"

	"github.com/luevano/libmangal/mangadata"
	"github.com/luevano/libmangal/metadata"
)

// epubMediaTypes are the EPUB 3.3 core image media types, by image.DecodeConfig's format name.
var epubMediaTypes = map[string]string{
	"jpeg": "image/jpeg",
	"png":  "image/png",
	"gif":  "image/gif",
	"webp": "image/webp",
}

// epubBook is what saveEPUB needs besides the pages.
type epubBook struct {
	// id is the unique identifier (a urn).
	id string
	// title of the book, required.
	title string
	// comicInfo is the metadata, nil if there's none.
	comicInfo *metadata.ComicInfoXML
	options   metadata.ComicInfoXMLOptions
	modified  time.Time
}

// saveEPUB saves pages in FormatEPUB: a fixed-layout EPUB 3, one page per image.
func (c *Client) saveEPUB(pages []mangadata.PageWithImage, out io.Writer, book epubBook) error {
	c.logger.Log("saving %d pages as EPUB", len(pages))

	type epubPage struct {
		name, mediaType string
		width, height   int
	}
	epubPages := make([]epubPage, len(pages))
	for i, page := range pages {
		name, err := pageName(i, page)
		if err != nil {
			return err
		}
		cfg, format, err := image.DecodeConfig(bytes.NewReader(page.Image()))
		if err != nil {
			return fmt.Errorf("page %d: %w", i+1, err)
		}
		mediaType, ok := epubMediaTypes[format]
		if !ok {
			return fmt.Errorf("page %d: %s images aren't supported by EPUB", i+1, format)
		}
		epubPages[i] = epubPage{name: name, mediaType: mediaType, width: cfg.Width, height: cfg.Height}
	}

	zipWriter := zip.NewWriter(out)
	// the mimetype must be first, stored, without extra fields or data descriptor
	if err := writeZIPRaw(zipWriter, "mimetype", []byte("application/epub+zip")); err != nil {
		return err
	}
	if err := writeZIPDeflated(zipWriter, "META-INF/container.xml", []byte(epubContainerXML)); err != nil {
		return err
	}

	var opf bytes.Buffer
	opf.WriteString(xml.Header)
	opf.WriteString(`<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="uid" xml:lang="` + xmlEscape(book.language()) + `">` + "\n")
	opf.WriteString(`  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">` + "\n")
	book.writeMetadata(&opf)
	opf.WriteString("    <meta property=\"rendition:layout\">pre-paginated</meta>\n")
	opf.WriteString("    <meta property=\"rendition:spread\">landscape</meta>\n")
	opf.WriteString("    <meta property=\"rendition:orientation\">auto</meta>\n")
	opf.WriteString("  </metadata>\n  <manifest>\n")
	opf.WriteString(`    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>` + "\n")
	for i, page := range epubPages {
		cover := ""
		if i == 0 {
			cover = ` properties="cover-image"`
		}
		fmt.Fprintf(&opf, `    <item id="p%04d" href="pages/%04d.xhtml" media-type="application/xhtml+xml"/>`+"\n", i+1, i+1)
		fmt.Fprintf(&opf, `    <item id="i%04d" href="images/%s" media-type="%s"%s/>`+"\n", i+1, page.name, page.mediaType, cover)
	}
	opf.WriteString("  </manifest>\n")
	if book.rightToLeft() {
		opf.WriteString(`  <spine page-progression-direction="rtl">` + "\n")
	} else {
		opf.WriteString("  <spine>\n")
	}
	for i := range epubPages {
		fmt.Fprintf(&opf, `    <itemref idref="p%04d"/>`+"\n", i+1)
	}
	opf.WriteString("  </spine>\n</package>\n")
	if err := writeZIPDeflated(zipWriter, "OEBPS/content.opf", opf.Bytes()); err != nil {
		return err
	}

	nav := fmt.Sprintf(xml.Header+`<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">
<head><title>%[1]s</title></head>
<body><nav epub:type="toc"><ol><li><a href="pages/0001.xhtml">%[1]s</a></li></ol></nav></body>
</html>
`, xmlEscape(book.title))
	if err := writeZIPDeflated(zipWriter, "OEBPS/nav.xhtml", []byte(nav)); err != nil {
		return err
	}

	for i, page := range epubPages {
		xhtml := fmt.Sprintf(xml.Header+`<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml">
<head>
<title>%d</title>
<meta name="viewport" content="width=%d, height=%d"/>
<style>html,body{margin:0;padding:0}img{display:block;width:100%%;height:100%%}</style>
</head>
<body><img src="../images/%s" alt=""/></body>
</html>
`, i+1, page.width, page.height, page.name)
		if err := writeZIPDeflated(zipWriter, fmt.Sprintf("OEBPS/pages/%04d.xhtml", i+1), []byte(xhtml)); err != nil {
			return err
		}
	}
	for i, page := range pages {
		if err := writeZIPFile(zipWriter, "OEBPS/images/"+epubPages[i].name, page.Image()); err != nil {
			return err
		}
	}
	return zipWriter.Close()
}

const epubContainerXML = xml.Header + `<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>
`

func (b epubBook) language() string {
	switch {
	case b.options.LanguageISO != "":
		return b.options.LanguageISO
	case b.comicInfo != nil && b.comicInfo.LanguageISO != "":
		return b.comicInfo.LanguageISO
	default:
		return "und"
	}
}

// rightToLeft is the reading direction, only known with metadata (like ComicInfo.xml's Manga).
func (b epubBook) rightToLeft() bool {
	return b.comicInfo != nil && metadata.RightToLeft(b.comicInfo.Country)
}

func (b epubBook) writeMetadata(w *bytes.Buffer) {
	element := func(name, value string) {
		if value != "" {
			fmt.Fprintf(w, "    <%s>%s</%s>\n", name, xmlEscape(value), name)
		}
	}
	fmt.Fprintf(w, "    <dc:identifier id=\"uid\">%s</dc:identifier>\n", xmlEscape(b.id))
	element("dc:title", b.title)
	element("dc:language", b.language())
	fmt.Fprintf(w, "    <meta property=\"dcterms:modified\">%s</meta>\n", b.modified.UTC().Format("2006-01-02T15:04:05Z"))

	ci := b.comicInfo
	if ci == nil {
		return
	}
	seen := map[string]bool{}
	for _, creator := range append(append([]string{}, ci.Writers...), ci.Pencillers...) {
		if !seen[creator] {
			seen[creator] = true
			element("dc:creator", creator)
		}
	}
	for _, translator := range ci.Translators {
		element("dc:contributor", translator)
	}
	element("dc:publisher", ci.Publisher)
	element("dc:description", ci.Summary)
	for _, genre := range ci.Genres {
		element("dc:subject", genre)
	}
	element("dc:source", ci.Web)
	element("dc:date", b.date())
	if ci.Series != "" {
		fmt.Fprintf(w, "    <meta property=\"belongs-to-collection\" id=\"series\">%s</meta>\n", xmlEscape(ci.Series))
		w.WriteString("    <meta refines=\"#series\" property=\"collection-type\">series</meta>\n")
		if ci.Number > 0 {
			fmt.Fprintf(w, "    <meta refines=\"#series\" property=\"group-position\">%s</meta>\n", strconv.FormatFloat(float64(ci.Number), 'f', -1, 32))
		}
	}
}

// date of the chapter as YYYY[-MM[-DD]], following ComicInfoXMLOptions' AddDate/AlternativeDate.
func (b epubBook) date() string {
	if !b.options.AddDate {
		return ""
	}
	year, month, day := b.comicInfo.Year, b.comicInfo.Month, b.comicInfo.Day
	if alt := b.options.AlternativeDate; alt != nil {
		year, month, day = alt.Year, alt.Month, alt.Day
	}
	switch {
	case year <= 0:
		return ""
	case month <= 0:
		return fmt.Sprintf("%04d", year)
	case day <= 0:
		return fmt.Sprintf("%04d-%02d", year, month)
	default:
		return fmt.Sprintf("%04d-%02d-%02d", year, month, day)
	}
}

// epubID is a name-based urn:uuid (RFC 9562 v5 layout) of the chapter, the same on every download.
func epubID(provider ProviderInfo, chapter mangadata.Chapter) string {
	manga := chapter.Volume().Manga().Info()
	info := chapter.Info()
	sum := sha1.Sum([]byte(strings.Join([]string{provider.ID, manga.ID, info.URL, strconv.FormatFloat(float64(info.Number), 'f', -1, 32), info.Title}, "\x00")))
	sum[6] = sum[6]&0x0f | 0x50
	sum[8] = sum[8]&0x3f | 0x80
	return fmt.Sprintf("urn:uuid:%x-%x-%x-%x-%x", sum[0:4], sum[4:6], sum[6:8], sum[8:10], sum[10:16])
}

func xmlEscape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

// writeZIPRaw writes a stored entry with its sizes and CRC in the local header (no data
// descriptor) and no extra fields.
func writeZIPRaw(zipWriter *zip.Writer, name string, data []byte) error {
	writer, err := zipWriter.CreateRaw(&zip.FileHeader{
		Name:               name,
		Method:             zip.Store,
		CRC32:              crc32.ChecksumIEEE(data),
		CompressedSize64:   uint64(len(data)),
		UncompressedSize64: uint64(len(data)),
	})
	if err != nil {
		return err
	}
	_, err = writer.Write(data)
	return err
}

func writeZIPDeflated(zipWriter *zip.Writer, name string, data []byte) error {
	writer, err := zipWriter.CreateHeader(&zip.FileHeader{
		Name:     name,
		Method:   zip.Deflate,
		Modified: time.Now(),
	})
	if err != nil {
		return err
	}
	_, err = writer.Write(data)
	return err
}
