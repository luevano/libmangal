package libmangal

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

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
