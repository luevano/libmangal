package anilist

import (
	"net/http"

	"github.com/luevano/libmangal/logger"
	"github.com/philippgille/gokv"
	"github.com/philippgille/gokv/syncmap"
)

// Options is options for Anilist client
type Options struct {
	// HTTPClient is a http client used for Anilist API
	HTTPClient *http.Client

	// QueryToIDsStore maps manga query to multiple anilit ids.
	//
	// ["berserk" => [7, 42, 69], "death note" => [887, 3, 134]]
	QueryToIDsStore gokv.Store

	// TitleToIDStore maps manga title to anilist id.
	//
	// ["berserk" => 7, "death note" => 3]
	TitleToIDStore gokv.Store

	// IDToMangaStore maps anilist id to anilist manga.
	//
	// [7 => "{title: ..., image: ..., ...}"]
	IDToMangaStore gokv.Store

	// AccessTokenStore holds the authentication data.
	AccessTokenStore gokv.Store

	// LogWriter used for logs progress
	Logger *logger.Logger
}

// DefaultOptions constructs default AnilistOptions
func DefaultOptions() Options {
	return Options{
		Logger: logger.NewLogger(),

		HTTPClient: &http.Client{},

		QueryToIDsStore:  syncmap.NewStore(syncmap.DefaultOptions),
		TitleToIDStore:   syncmap.NewStore(syncmap.DefaultOptions),
		IDToMangaStore:   syncmap.NewStore(syncmap.DefaultOptions),
		AccessTokenStore: syncmap.NewStore(syncmap.DefaultOptions),
	}
}
