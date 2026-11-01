package libmangal

import (
	"archive/zip"
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"image/png"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"

	"github.com/gen2brain/webp"
	"github.com/luevano/libmangal/mangadata"
)

// noisePNG is a page that lossy WebP does shrink.
func noisePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, noiseImage(w, h)); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func noiseImage(w, h int) *image.Gray {
	img := image.NewGray(image.Rect(0, 0, w, h))
	random := rand.New(rand.NewPCG(1, 2))
	for i := range img.Pix {
		img.Pix[i] = byte(random.Uint32())
	}
	return img
}

func TestConvertToWebP(t *testing.T) {
	c := newTestClient(t)
	chapter := newFakeChapter(nil, 3)
	png := noisePNG(t, 30, 40)
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

func TestConvertToWebPKeepsOriginal(t *testing.T) {
	// over libwebp's 16383 px limit
	var tall bytes.Buffer
	if err := png.Encode(&tall, image.NewGray(image.Rect(0, 0, 2, 16384))); err != nil {
		t.Fatal(err)
	}
	// noise at JPEG quality 1 is small, WebP quality 100 has to spell out its artifacts
	var small bytes.Buffer
	if err := jpeg.Encode(&small, noiseImage(64, 64), &jpeg.Options{Quality: 1}); err != nil {
		t.Fatal(err)
	}
	decoded, err := jpeg.Decode(bytes.NewReader(small.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	var encoded bytes.Buffer
	if err := webp.Encode(&encoded, decoded, webp.Options{Quality: 100, Method: webp.DefaultMethod}); err != nil {
		t.Fatal(err)
	}
	if encoded.Len() < small.Len() {
		t.Fatalf("the WebP is smaller (%d < %d), no use for this test", encoded.Len(), small.Len())
	}

	c := newTestClient(t)
	chapter := newFakeChapter(nil, 2)
	images := [][]byte{tall.Bytes(), small.Bytes()}
	pages := make([]mangadata.PageWithImage, len(images))
	for i, data := range images {
		pages[i] = &pageWithImage{Page: &fakePage{chapter: chapter, n: i + 1}, image: data}
	}
	if err := c.convertToWebP(context.Background(), pages, 100); err != nil {
		t.Fatal(err)
	}
	for i, data := range images {
		if !bytes.Equal(pages[i].Image(), data) {
			t.Errorf("page %d changed", i+1)
		}
		if got := pages[i].Extension(); got != ".jpg" {
			t.Errorf("page %d extension %q", i+1, got)
		}
	}
}

func TestDownloadWebP(t *testing.T) {
	c, options := newDownloadTest(t)
	options.ImageFormat = ImageFormatWebP
	chapter := newFakeChapter(validMeta(), 2)
	chapter.ext = ".png"
	chapter.image = noisePNG(t, 20, 20)
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

	options.ImageQuality = 101
	if _, err := c.DownloadChapterContext(context.Background(), chapter, options); err == nil {
		t.Error("quality 101: no error")
	}
	options.ImageQuality = 0
	options.ImageFormat = ImageFormatWebP + 1
	if _, err := c.DownloadChapterContext(context.Background(), chapter, options); err == nil {
		t.Error("unknown image format: no error")
	}
}
