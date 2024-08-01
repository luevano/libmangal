package myanimelist

import (
	"net/http"

	"github.com/luevano/libmangal/logger"
)

type Options struct {
	// ClientID of the MyAnimeList API client. Required.
	ClientID string

	// NSFW if NSFW mangas should be included in the searches.
	NSFW bool

	// HTTPClient is a http client used for MyAnimeList API.
	HTTPClient *http.Client

	// Logger used for logs progress.
	//
	// If Logger is nil, a new one will be created.
	Logger *logger.Logger
}

// DefaultOptions constructs default MyAnimeList Options.
//
// Note: the ClientID still needs to be passed separately.
func DefaultOptions() Options {
	return Options{
		NSFW:       false,
		HTTPClient: &http.Client{},
		Logger:     logger.NewLogger(),
	}
}
