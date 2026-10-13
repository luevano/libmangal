package libmangal

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/luevano/libmangal/mangadata"
	"github.com/luevano/libmangal/metadata"
	"github.com/pdfcpu/pdfcpu/pkg/api"
)

func testPages(n, size int) []mangadata.PageWithImage {
	chapter := newFakeChapter(nil, n)
	pages := make([]mangadata.PageWithImage, n)
	for i := range pages {
		pages[i] = &pageWithImage{Page: &fakePage{chapter: chapter, n: i + 1}, image: bytes.Repeat([]byte{'x'}, size)}
	}
	return pages
}

func TestSaveArchivesReturnWriteErrors(t *testing.T) {
	c := &Client{options: DefaultClientOptions(), logger: newTestLogger()}
	// bigger than the archive writers' buffers so the page write itself fails
	pages := testPages(2, 64<<10)

	if _, err := c.saveCBZ(pages, errWriter{}, nil, metadata.DefaultComicInfoOptions()); !errors.Is(err, errWrite) {
		t.Errorf("saveCBZ: got %v, want %v", err, errWrite)
	}
	if err := c.saveZIP(pages, errWriter{}); !errors.Is(err, errWrite) {
		t.Errorf("saveZIP: got %v, want %v", err, errWrite)
	}
	if err := c.saveTAR(pages, errWriter{}); !errors.Is(err, errWrite) {
		t.Errorf("saveTAR: got %v, want %v", err, errWrite)
	}
	if err := c.saveTARGZ(pages, errWriter{}); !errors.Is(err, errWrite) {
		t.Errorf("saveTARGZ: got %v, want %v", err, errWrite)
	}
}

func TestSaveArchivesReturnCloseErrors(t *testing.T) {
	c := &Client{options: DefaultClientOptions(), logger: newTestLogger()}
	// small enough to stay buffered until the writers are closed
	pages := testPages(1, 10)

	if _, err := c.saveCBZ(pages, errWriter{}, nil, metadata.DefaultComicInfoOptions()); !errors.Is(err, errWrite) {
		t.Errorf("saveCBZ: got %v, want %v", err, errWrite)
	}
	if err := c.saveZIP(pages, errWriter{}); !errors.Is(err, errWrite) {
		t.Errorf("saveZIP: got %v, want %v", err, errWrite)
	}
	if err := c.saveTARGZ(pages, errWriter{}); !errors.Is(err, errWrite) {
		t.Errorf("saveTARGZ: got %v, want %v", err, errWrite)
	}
}

func TestMangaImageReferer(t *testing.T) {
	var referer string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		referer = r.Header.Get("Referer")
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("image"))
	}))
	defer server.Close()
	c := newTestClient(t)

	// cover from the metadata (e.g. AniList): the manga page isn't its referer
	meta := validMeta()
	meta.CoverImage = server.URL + "/meta.jpg"
	manga := newFakeChapter(meta, 1).volume.manga
	if err := c.downloadMangaImage(context.Background(), manga, mangaImageCover, io.Discard); err != nil {
		t.Fatal(err)
	}
	if referer != "" {
		t.Errorf("metadata cover sent referer %q, want none", referer)
	}

	// cover from the provider
	manga.info.Cover = server.URL + "/provider.jpg"
	if err := c.downloadMangaImage(context.Background(), manga, mangaImageCover, io.Discard); err != nil {
		t.Fatal(err)
	}
	// only the origin, like a browser (2xstorage.com refuses the manga page as referer)
	manga.info.URL = "https://site.example/manga/berserk?x=1"
	if err := c.downloadMangaImage(context.Background(), manga, mangaImageCover, io.Discard); err != nil {
		t.Fatal(err)
	}
	if want := "https://site.example/"; referer != want {
		t.Errorf("provider cover sent referer %q, want %q", referer, want)
	}
}

func TestSavePDF(t *testing.T) {
	var img bytes.Buffer
	if err := png.Encode(&img, image.NewGray(image.Rect(0, 0, 20, 30))); err != nil {
		t.Fatal(err)
	}
	pages := testPages(3, 0)
	for _, p := range pages {
		p.SetImage(img.Bytes())
	}
	c := &Client{options: DefaultClientOptions(), logger: newTestLogger()}

	var out bytes.Buffer
	if err := c.savePDF(context.Background(), pages, &out); err != nil {
		t.Fatal(err)
	}
	n, err := api.PageCount(context.Background(), bytes.NewReader(out.Bytes()), nil)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("%d pages, want 3", n)
	}
}

func TestMangaImageChecks(t *testing.T) {
	var img bytes.Buffer
	if err := png.Encode(&img, image.NewGray(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name, contentType string
		body              []byte
		ok                bool
	}{
		{"png", "image/png", img.Bytes(), true},
		{"octet-stream png", "application/octet-stream", img.Bytes(), true},
		{"html", "text/html", []byte("<html><body>blocked</body></html>"), false},
		{"html as octet-stream", "application/octet-stream", []byte("<html><body>blocked</body></html>"), false},
		{"too big", "image/png", make([]byte, maxMangaImageSize+1), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tt.contentType)
				_, _ = w.Write(tt.body)
			}))
			defer server.Close()
			meta := validMeta()
			meta.CoverImage = server.URL + "/cover"
			var out bytes.Buffer
			err := newTestClient(t).downloadMangaImage(context.Background(), newFakeChapter(meta, 1).volume.manga, mangaImageCover, &out)
			if tt.ok != (err == nil) || (err != nil && out.Len() != 0) {
				t.Errorf("err %v, wrote %d bytes", err, out.Len())
			}
		})
	}
}

func TestMangaImageURL(t *testing.T) {
	meta := validMeta()
	meta.CoverImage, meta.BannerImage = "meta-cover", "meta-banner"
	manga := newFakeChapter(meta, 1).volume.manga
	for _, tt := range []struct {
		image        mangaImage
		provider     string
		want         string
		fromProvider bool
	}{
		{mangaImageCover, "", "meta-cover", false},
		{mangaImageCover, "provider", "provider", true},
		{mangaImageBanner, "", "meta-banner", false},
		{mangaImageBanner, "provider", "provider", true},
	} {
		manga.info.Cover, manga.info.Banner = "", ""
		if tt.image == mangaImageCover {
			manga.info.Cover = tt.provider
		} else {
			manga.info.Banner = tt.provider
		}
		got, fromProvider, err := mangaImageURL(manga, tt.image)
		if err != nil || got != tt.want || fromProvider != tt.fromProvider {
			t.Errorf("%s: got %q, %t, %v; want %q, %t", tt.image, got, fromProvider, err, tt.want, tt.fromProvider)
		}
	}
	manga.meta, manga.info.Banner = nil, ""
	if got, _, err := mangaImageURL(manga, mangaImageBanner); got != "" || err != nil {
		t.Errorf("no metadata: got %q, %v", got, err)
	}
	if _, _, err := mangaImageURL(manga, "icon"); err == nil {
		t.Error("unknown image: no error")
	}
}
