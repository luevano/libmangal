package libmangal

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/luevano/libmangal/metadata"
	"github.com/luevano/libmangal/metadata/anilist"
	"github.com/luevano/libmangal/metadata/kitsu"
	"github.com/luevano/libmangal/metadata/myanimelist"
	"github.com/luevano/libmangal/netutil"
)

func TestZeroOptionsReturnErrors(t *testing.T) {
	if _, err := NewClientContext(context.Background(), fakeLoader{&fakeProvider{}}, ClientOptions{}); err == nil {
		t.Error("NewClient: got no error with zero options")
	}
	if _, err := metadata.NewProviderWithCache(metadata.ProviderWithCacheOptions{Provider: &fakeMetaProvider{}}); err == nil {
		t.Error("NewProviderWithCache: got no error without a CacheStore")
	}
	if _, err := anilist.NewAnilist(anilist.Options{}); err == nil {
		t.Error("NewAnilist: got no error without an HTTPClient")
	}
	if _, err := myanimelist.NewMAL(myanimelist.Options{ClientID: "x"}); err == nil {
		t.Error("NewMAL: got no error without an HTTPClient")
	}

	c, options := newDownloadTest(t)
	zero := DownloadOptions{Directory: options.Directory}
	if _, err := c.DownloadChapterContext(context.Background(), newFakeChapter(validMeta(), 1), zero); err == nil {
		t.Error("DownloadChapter: got no error with zero options")
	}
	// a nil transformer is fine
	zero.Format = FormatCBZ
	if _, err := c.DownloadChapterContext(context.Background(), newFakeChapter(validMeta(), 1), zero); err != nil {
		t.Errorf("DownloadChapter: %v", err)
	}
}

func TestDefaultHTTPClients(t *testing.T) {
	for name, client := range map[string]*http.Client{
		"libmangal":   DefaultClientOptions().HTTPClient,
		"anilist":     anilist.DefaultOptions().HTTPClient,
		"myanimelist": myanimelist.DefaultOptions().HTTPClient,
		"kitsu":       kitsu.DefaultOptions().HTTPClient,
	} {
		if client.Timeout != netutil.DefaultTimeout {
			t.Errorf("%s: timeout %s, want %s", name, client.Timeout, netutil.DefaultTimeout)
		}
	}
}

func TestSentinelsAreShared(t *testing.T) {
	if !errors.Is(&anilist.HTTPError{StatusCode: http.StatusNotFound}, ErrNotFound) {
		t.Error("an AniList 404 doesn't match ErrNotFound")
	}
	if !errors.Is(&kitsu.RateLimitError{}, ErrRateLimited) {
		t.Error("a Kitsu 429 doesn't match ErrRateLimited")
	}
}
