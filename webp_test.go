package libmangal

import (
	"archive/zip"
	"bytes"
	"context"
	"image"
	"os"
	"path/filepath"
	"testing"

	"github.com/gen2brain/webp"
	"github.com/luevano/libmangal/mangadata"
)

func TestConvertToWebP(t *testing.T) {
	c := newTestClient(t)
	chapter := newFakeChapter(nil, 3)
	png := testPNG(t, 30, 40)
	var already bytes.Buffer
	if err := webp.Encode(&already, image.NewGray(image.Rect(0, 0, 5, 5))); err != nil {
		t.Fatal(err)
	}
	images := [][]byte{png, already.Bytes(), []byte("not an image")}
	pages := make([]mangadata.PageWithImage, len(images))
	for i, data := range images {
		pages[i] = &pageWithImage{Page: &fakePage{chapter: chapter, n: i + 1}, image: data}
	}

	if err := c.convertToWebP(context.Background(), pages, 0); err != nil {
		t.Fatal(err)
	}
	img, err := webp.Decode(bytes.NewReader(pages[0].Image()))
	if err != nil {
		t.Fatalf("decode the converted page: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 30 || b.Dy() != 40 {
		t.Errorf("converted page is %v, want 30x40", b)
	}
	if !bytes.Equal(pages[1].Image(), already.Bytes()) {
		t.Error("a WebP page was encoded again")
	}
	for i, want := range []string{".webp", ".webp", ".jpg"} {
		if got := pages[i].Extension(); got != want {
			t.Errorf("page %d extension %q, want %q", i+1, got, want)
		}
	}
	if string(pages[2].Image()) != "not an image" {
		t.Error("an undecodable page changed")
	}
}

func TestDownloadWebP(t *testing.T) {
	c, options := newDownloadTest(t)
	options.WebP = true
	chapter := newFakeChapter(validMeta(), 2)
	chapter.ext = ".png"
	chapter.image = testPNG(t, 20, 20)
	down, err := c.DownloadChapterContext(context.Background(), chapter, options)
	if err != nil {
		t.Fatal(err)
	}
	r, err := zip.OpenReader(filepath.Join(down.Directory, down.Filename))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	var names []string
	for _, f := range r.File {
		names = append(names, f.Name)
	}
	if len(names) != 2 || names[0] != "0001.webp" || names[1] != "0002.webp" {
		t.Errorf("CBZ entries %v", names)
	}

	// EPUB declares them as WebP
	options.Format = FormatEPUB
	if down, err = c.DownloadChapterContext(context.Background(), chapter, options); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(down.Directory, down.Filename))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("OEBPS/images/0001.webp")) {
		t.Error("EPUB has no WebP page")
	}

	options.WebPQuality = 101
	if _, err := c.DownloadChapterContext(context.Background(), chapter, options); err == nil {
		t.Error("quality 101: no error")
	}
}
