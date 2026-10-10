package libmangal

import (
	"context"
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
		down, err := c.DownloadChapter(context.Background(), chapter, options)
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
