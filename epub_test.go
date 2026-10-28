package libmangal

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewGray(image.Rect(0, 0, w, h))
	for x := range w {
		img.SetGray(x, x%h, color.Gray{Y: 200})
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func readZIPEntry(t *testing.T, f *zip.File) []byte {
	t.Helper()
	r, err := f.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

type opfPackage struct {
	Metadata struct {
		Identifier string   `xml:"identifier"`
		Title      string   `xml:"title"`
		Language   string   `xml:"language"`
		Creators   []string `xml:"creator"`
		Metas      []struct {
			Property string `xml:"property,attr"`
			Value    string `xml:",chardata"`
		} `xml:"meta"`
	} `xml:"metadata"`
	Items []struct {
		ID         string `xml:"id,attr"`
		Href       string `xml:"href,attr"`
		MediaType  string `xml:"media-type,attr"`
		Properties string `xml:"properties,attr"`
	} `xml:"manifest>item"`
	Spine struct {
		Direction string `xml:"page-progression-direction,attr"`
		Refs      []struct {
			IDRef string `xml:"idref,attr"`
		} `xml:"itemref"`
	} `xml:"spine"`
}

func TestDownloadEPUB(t *testing.T) {
	c, options := newDownloadTest(t)
	options.Format = FormatEPUB
	chapter := newFakeChapter(validMeta(), 3)
	chapter.ext = ".png"
	chapter.image = testPNG(t, 30, 40)

	down, err := c.DownloadChapterContext(context.Background(), chapter, options)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(down.Filename, ".epub") {
		t.Errorf("filename %q", down.Filename)
	}
	data, err := os.ReadFile(filepath.Join(down.Directory, down.Filename))
	if err != nil {
		t.Fatal(err)
	}
	// OCF: the mimetype is the first local header, stored, no extra field: name at 30, content at 38
	if got := string(data[30:58]); got != "mimetypeapplication/epub+zip" {
		t.Fatalf("bytes 30-58 %q, want the mimetype entry", got)
	}
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if first := r.File[0]; first.Name != "mimetype" || first.Method != zip.Store || len(first.Extra) != 0 || first.Flags&0x8 != 0 {
		t.Fatalf("first entry %q method %d extra %d flags %x", first.Name, first.Method, len(first.Extra), first.Flags)
	}
	files := map[string]*zip.File{}
	for _, f := range r.File {
		files[f.Name] = f
	}

	var container struct {
		Rootfiles []struct {
			FullPath string `xml:"full-path,attr"`
		} `xml:"rootfiles>rootfile"`
	}
	if err := xml.Unmarshal(readZIPEntry(t, files["META-INF/container.xml"]), &container); err != nil {
		t.Fatal(err)
	}
	if len(container.Rootfiles) != 1 || files[container.Rootfiles[0].FullPath] == nil {
		t.Fatalf("container rootfiles %+v", container.Rootfiles)
	}
	opfPath := container.Rootfiles[0].FullPath
	var opf opfPackage
	if err := xml.Unmarshal(readZIPEntry(t, files[opfPath]), &opf); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(opf.Metadata.Identifier, "urn:uuid:") || opf.Metadata.Title != "Chapter 1" || opf.Metadata.Language != "und" {
		t.Errorf("metadata %+v", opf.Metadata)
	}
	if len(opf.Metadata.Creators) != 1 || opf.Metadata.Creators[0] != "Someone" {
		t.Errorf("creators %v", opf.Metadata.Creators)
	}
	metas := map[string]string{}
	for _, m := range opf.Metadata.Metas {
		metas[m.Property] = m.Value
	}
	for property, want := range map[string]string{
		"rendition:layout": "pre-paginated", "belongs-to-collection": "Fake Manga", "group-position": "1",
	} {
		if metas[property] != want {
			t.Errorf("meta %s = %q, want %q", property, metas[property], want)
		}
	}
	if metas["dcterms:modified"] == "" {
		t.Error("no dcterms:modified")
	}
	// no country: read like ComicInfo.xml's Manga, right to left
	if opf.Spine.Direction != "rtl" || len(opf.Spine.Refs) != 3 {
		t.Errorf("spine %+v", opf.Spine)
	}

	dir := filepath.Dir(opfPath)
	items := map[string]string{}
	nav := false
	for _, item := range opf.Items {
		items[item.ID] = item.Href
		if files[dir+"/"+item.Href] == nil {
			t.Errorf("manifest item %s not in the zip", item.Href)
		}
		nav = nav || item.Properties == "nav"
		if item.ID == "i0001" && (item.Properties != "cover-image" || item.MediaType != "image/png") {
			t.Errorf("first image %+v", item)
		}
	}
	if !nav {
		t.Error("no nav item")
	}
	page := readZIPEntry(t, files[dir+"/"+items[opf.Spine.Refs[0].IDRef]])
	if err := xml.Unmarshal(page, new(struct{})); err != nil {
		t.Errorf("page xhtml: %v", err)
	}
	if !bytes.Contains(page, []byte(`content="width=30, height=40"`)) {
		t.Errorf("page has no viewport of the image size:\n%s", page)
	}
}

func TestDownloadEPUBWithoutMetadata(t *testing.T) {
	c, options := newDownloadTest(t)
	options.Format = FormatEPUB
	options.Strict = false
	chapter := newFakeChapter(nil, 1)
	chapter.info.Title = ""
	chapter.image = testPNG(t, 10, 10)
	down, err := c.DownloadChapterContext(context.Background(), chapter, options)
	if err != nil {
		t.Fatal(err)
	}
	r, err := zip.OpenReader(filepath.Join(down.Directory, down.Filename))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	for _, f := range r.File {
		if f.Name != "OEBPS/content.opf" {
			continue
		}
		var opf opfPackage
		if err := xml.Unmarshal(readZIPEntry(t, f), &opf); err != nil {
			t.Fatal(err)
		}
		if opf.Metadata.Title != "Chapter 1" || opf.Spine.Direction != "" {
			t.Errorf("title %q, direction %q", opf.Metadata.Title, opf.Spine.Direction)
		}
	}
}

func TestDownloadEPUBRejectsNonImages(t *testing.T) {
	c, options := newDownloadTest(t)
	options.Format = FormatEPUB
	_, err := c.DownloadChapterContext(context.Background(), newFakeChapter(validMeta(), 1), options)
	if err == nil || !strings.Contains(err.Error(), "write EPUB: page 1") {
		t.Fatalf("err %v", err)
	}
}
