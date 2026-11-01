package libmangal

import (
	"archive/zip"
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"

	"github.com/gen2brain/vpx/webp"
	"github.com/luevano/libmangal/mangadata"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/spf13/afero"
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
	for i, want := range []string{"0001.webp", "0002.webp", "0003.jpg"} {
		if got, err := pageName(i, pages[i]); err != nil || got != want {
			t.Errorf("page %d name %q (%v), want %q", i+1, got, err, want)
		}
	}
	if string(pages[2].Image()) != "not an image" {
		t.Error("an undecodable page changed")
	}
}

func TestConvertToWebPKeepsOriginal(t *testing.T) {
	// over WebP's 16383 px limit
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
	if err := webp.Encode(&encoded, decoded, webp.EncodeOptions{Quality: 100}); err != nil {
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
	want := []string{"0001.png", "0002.jpg"}
	for i, data := range images {
		if !bytes.Equal(pages[i].Image(), data) {
			t.Errorf("page %d changed", i+1)
		}
		if got, err := pageName(i, pages[i]); err != nil || filepath.Ext(got) != filepath.Ext(want[i]) {
			t.Errorf("page %d name %q (%v), want %s", i+1, got, err, want[i])
		}
	}
}

// a site's WebP behind a .jpg url is named .webp without converting anything.
func TestDownloadNamesPagesByContent(t *testing.T) {
	var page bytes.Buffer
	if err := webp.Encode(&page, image.NewGray(image.Rect(0, 0, 5, 5))); err != nil {
		t.Fatal(err)
	}
	c, options := newDownloadTest(t)
	chapter := newFakeChapter(validMeta(), 1)
	chapter.image = page.Bytes()

	options.Format = FormatCBZ
	down, err := c.DownloadChapterContext(context.Background(), chapter, options)
	if err != nil {
		t.Fatal(err)
	}
	r, err := zip.OpenReader(filepath.Join(down.Directory, down.Filename))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if r.File[0].Name != "0001.webp" {
		t.Errorf("CBZ entry %q", r.File[0].Name)
	}

	options.Format = FormatImages
	if down, err = c.DownloadChapterContext(context.Background(), chapter, options); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(down.Directory, down.Filename, "0001.webp")); err != nil {
		t.Error(err)
	}

	options.Format = FormatEPUB
	if down, err = c.DownloadChapterContext(context.Background(), chapter, options); err != nil {
		t.Fatal(err)
	}
	e, err := zip.OpenReader(filepath.Join(down.Directory, down.Filename))
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	opf, err := e.Open("OEBPS/content.opf")
	if err != nil {
		t.Fatal(err)
	}
	defer opf.Close()
	data, err := io.ReadAll(opf)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte(`href="images/0001.webp" media-type="image/webp"`)) {
		t.Errorf("EPUB manifest:\n%s", data)
	}
}

func TestDownloadWebP(t *testing.T) {
	c, options := newDownloadTest(t)
	options.ImageFormat = ImageFormatWebP
	chapter := newFakeChapter(validMeta(), 2)
	chapter.ext = ".png"
	chapter.image = noisePNG(t, 64, 64)
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

// blackWebP is a lossy WebP page, all black.
func blackWebP(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for i := 3; i < len(img.Pix); i += 4 {
		img.Pix[i] = 255
	}
	var buf bytes.Buffer
	if err := webp.Encode(&buf, img, webp.EncodeOptions{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestDownloadWebPSource: a source answering WebP works for every format, under a wrong
// extension too.
func TestDownloadWebPSource(t *testing.T) {
	for _, ext := range []string{".webp", ".jpg"} {
		for _, format := range FormatValues() {
			c, options := newDownloadTest(t)
			options.Format = format
			chapter := newFakeChapter(validMeta(), 2)
			chapter.ext = ext
			chapter.image = blackWebP(t)
			if _, err := c.DownloadChapterContext(context.Background(), chapter, options); err != nil {
				t.Errorf("%s %s: %v", format, ext, err)
			}
		}
	}
}

// TestPDFWebPKeepsBlacks: pdfcpu alone gets black 16 out of a WebP page.
func TestPDFWebPKeepsBlacks(t *testing.T) {
	c, options := newDownloadTest(t)
	options.Format = FormatPDF
	chapter := newFakeChapter(validMeta(), 1)
	chapter.ext = ".webp"
	chapter.image = blackWebP(t)
	down, err := c.DownloadChapterContext(context.Background(), chapter, options)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	pdf := filepath.Join(dir, "chapter.pdf")
	data, err := afero.ReadFile(c.options.FS, filepath.Join(down.Directory, down.Filename))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pdf, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := api.ExtractImagesFile(context.Background(), pdf, dir, nil, nil); err != nil {
		t.Fatal(err)
	}
	found, err := filepath.Glob(filepath.Join(dir, "*.png"))
	if err != nil || len(found) != 1 {
		t.Fatalf("extracted %v, err %v", found, err)
	}
	extracted, err := os.Open(found[0])
	if err != nil {
		t.Fatal(err)
	}
	defer extracted.Close()
	img, err := png.Decode(extracted)
	if err != nil {
		t.Fatal(err)
	}
	if r, _, _, _ := img.At(1, 1).RGBA(); r>>8 > 2 {
		t.Errorf("black came out as %d", r>>8)
	}
}
