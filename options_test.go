package libmangal

import (
	"context"
	"testing"

	"github.com/luevano/libmangal/metadata"
	"github.com/luevano/libmangal/metadata/anilist"
	"github.com/luevano/libmangal/metadata/myanimelist"
)

func TestZeroOptionsReturnErrors(t *testing.T) {
	if _, err := NewClient(context.Background(), fakeLoader{&fakeProvider{}}, ClientOptions{}); err == nil {
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
	if _, err := c.DownloadChapter(context.Background(), newFakeChapter(validMeta(), 1), zero); err == nil {
		t.Error("DownloadChapter: got no error with zero options")
	}
	// a nil transformer is fine
	zero.Format = FormatCBZ
	if _, err := c.DownloadChapter(context.Background(), newFakeChapter(validMeta(), 1), zero); err != nil {
		t.Errorf("DownloadChapter: %v", err)
	}
}
