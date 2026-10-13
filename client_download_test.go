package libmangal

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/luevano/libmangal/mangadata"
	"github.com/luevano/libmangal/metadata"
)

func newDownloadTest(t *testing.T) (*Client, DownloadOptions) {
	t.Helper()
	c := newTestClient(t)
	options := DefaultDownloadOptions()
	options.Directory = t.TempDir()
	options.SearchMetadata = false
	return c, options
}

func TestDownloadFailedCoverIsNotWritten(t *testing.T) {
	c, options := newDownloadTest(t)
	options.Strict = false
	options.DownloadMangaCover = true
	// no cover url anywhere, so the cover download fails
	chapter := newFakeChapter(validMeta(), 2)

	for range 2 {
		down, err := c.DownloadChapterContext(context.Background(), chapter, options)
		if err != nil {
			t.Fatal(err)
		}
		if down.CoverStatus != metadata.DownloadStatusFailed {
			t.Errorf("cover status %q, want %q", down.CoverStatus, metadata.DownloadStatusFailed)
		}
	}
	cover := filepath.Join(options.Directory, "Fake Manga", metadata.FilenameCoverJPG)
	if _, err := os.Stat(cover); !os.IsNotExist(err) {
		t.Errorf("%s exists after a failed download (err %v)", cover, err)
	}
}

func TestDownloadChapterStatus(t *testing.T) {
	c, options := newDownloadTest(t)
	options.SkipIfExists = false
	chapter := newFakeChapter(validMeta(), 2)

	for _, want := range []metadata.DownloadStatus{metadata.DownloadStatusNew, metadata.DownloadStatusOverwritten} {
		down, err := c.DownloadChapterContext(context.Background(), chapter, options)
		if err != nil {
			t.Fatal(err)
		}
		if down.ChapterStatus != want {
			t.Errorf("chapter status %q, want %q", down.ChapterStatus, want)
		}
		if down.Pages != 2 {
			t.Errorf("%d pages, want 2", down.Pages)
		}
	}

	options.SkipIfExists = true
	down, err := c.DownloadChapterContext(context.Background(), chapter, options)
	if err != nil {
		t.Fatal(err)
	}
	if down.ChapterStatus != metadata.DownloadStatusExists || down.Pages != 0 {
		t.Errorf("chapter status %q, %d pages, want %q and 0 pages", down.ChapterStatus, down.Pages, metadata.DownloadStatusExists)
	}
}

func TestDownloadProgress(t *testing.T) {
	c, options := newDownloadTest(t)
	var got []string
	options.OnProgress = func(p DownloadProgress) {
		got = append(got, fmt.Sprintf("%s %d/%d", p.Stage, p.Done, p.Total))
	}
	if _, err := c.DownloadChapterContext(context.Background(), newFakeChapter(validMeta(), 3), options); err != nil {
		t.Fatal(err)
	}
	want := []string{"pages 0/0", "images 0/3", "images 1/3", "images 2/3", "images 3/3", "saving 3/3"}
	if !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDownloadPagesConcurrencyLimit(t *testing.T) {
	provider := &fakeProvider{}
	options := DefaultClientOptions()
	options.ConcurrentPages = 3
	c, err := NewClientContext(context.Background(), fakeLoader{provider}, options)
	if err != nil {
		t.Fatal(err)
	}
	chapter := newFakeChapter(nil, 50)
	pages, _ := provider.ChapterPages(context.Background(), chapter)

	downloaded, err := c.DownloadPagesInBatchContext(context.Background(), pages)
	if err != nil {
		t.Fatal(err)
	}
	if len(downloaded) != 50 {
		t.Fatalf("got %d pages, want 50", len(downloaded))
	}
	if got := provider.maxInFlight.Load(); got > 3 {
		t.Errorf("%d pages downloaded at once, want at most 3", got)
	}
}

func TestDownloadErrorsWrap(t *testing.T) {
	errImage := errors.New("fake: image of page: 403 forbidden")
	c, err := NewClientContext(context.Background(), fakeLoader{&fakeProvider{imageErr: errImage}}, DefaultClientOptions())
	if err != nil {
		t.Fatal(err)
	}
	options := DefaultDownloadOptions()
	options.Directory = t.TempDir()
	options.SearchMetadata = false

	_, err = c.DownloadChapterContext(context.Background(), newFakeChapter(validMeta(), 1), options)
	if !errors.Is(err, errImage) {
		t.Fatalf("got %v, want it to wrap %v", err, errImage)
	}
	if want := "page 1: " + errImage.Error(); err.Error() != want {
		t.Errorf("got %q, want %q", err, want)
	}

	chapter := newFakeChapter(validMeta(), 1)
	chapter.info.Unavailable = "external"
	if _, err := c.DownloadChapterContext(context.Background(), chapter, options); !errors.Is(err, ErrUnavailable) {
		t.Errorf("got %v, want ErrUnavailable", err)
	}
}

type fakeChapterIndex struct {
	path, provider string
}

func (f fakeChapterIndex) FindChapter(string, mangadata.Chapter, Format) (string, string, bool, error) {
	return f.path, f.provider, f.path != "", nil
}

func TestChapterIndexSkips(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "Fake Manga", "Chapter 1 from elsewhere.cbz")
	if err := os.MkdirAll(filepath.Dir(existing), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(existing, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	newClient := func(index ChapterIndex) *Client {
		options := DefaultClientOptions()
		options.ChapterIndex = index
		c, err := NewClientContext(context.Background(), fakeLoader{&fakeProvider{}}, options)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	options := DefaultDownloadOptions()
	options.Directory = dir
	options.SearchMetadata = false
	chapter := newFakeChapter(validMeta(), 1)

	down, err := newClient(fakeChapterIndex{path: existing, provider: "other"}).DownloadChapterContext(context.Background(), chapter, options)
	if err != nil {
		t.Fatal(err)
	}
	if down.ChapterStatus != metadata.DownloadStatusExists || down.Path() != existing || !strings.Contains(down.Skipped, `from other as "Chapter 1 from elsewhere.cbz"`) {
		t.Errorf("got %+v", down)
	}

	// a stale entry (the file is gone) doesn't stop the download
	down, err = newClient(fakeChapterIndex{path: filepath.Join(dir, "gone.cbz"), provider: "other"}).DownloadChapterContext(context.Background(), chapter, options)
	if err != nil {
		t.Fatal(err)
	}
	if down.ChapterStatus != metadata.DownloadStatusNew || down.Skipped != "" {
		t.Errorf("got %+v", down)
	}

	// again, the same file
	down, _ = newClient(nil).DownloadChapterContext(context.Background(), chapter, options)
	if down.Skipped != "already downloaded" {
		t.Errorf("skipped %q", down.Skipped)
	}
}

type accountProvider struct{ *fakeProvider }

func (accountProvider) Account(context.Context) (Account, bool, error) {
	return Account{Service: "Fake", Status: "deluxe"}, true, nil
}

func TestAccount(t *testing.T) {
	c, _ := NewClientContext(context.Background(), fakeLoader{accountProvider{&fakeProvider{}}}, DefaultClientOptions())
	if account, ok, err := c.Account(); err != nil || !ok || account.Status != "deluxe" {
		t.Errorf("got %+v %t %v", account, ok, err)
	}
	c, _ = NewClientContext(context.Background(), fakeLoader{&fakeProvider{}}, DefaultClientOptions())
	if _, ok, _ := c.Account(); ok {
		t.Error("account without an AccountProvider")
	}
	ani := &fakeMetaProvider{source: metadata.IDSourceAnilist}
	if account := metadata.AccountOf(ani); account.Status != "not logged in" {
		// the fake is authenticated but has no user
		t.Errorf("got %+v", account)
	}
}
