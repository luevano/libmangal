package libmangal

import (
	"bytes"
	"errors"
	"testing"

	"github.com/luevano/libmangal/mangadata"
	"github.com/luevano/libmangal/metadata"
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
