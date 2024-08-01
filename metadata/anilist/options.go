package anilist

import (
	"net/http"

	"github.com/luevano/libmangal/logger"
)

// Options is options for Anilist client.
type Options struct {
	// HTTPClient is a http client used for Anilist API.
	HTTPClient *http.Client

	// Logger used for logs progress.
	//
	// If Logger is nil, a new one will be created.
	Logger *logger.Logger
}

// DefaultOptions constructs default Anilist Options.
func DefaultOptions() Options {
	return Options{
		HTTPClient: &http.Client{},
		Logger:     logger.NewLogger(),
	}
}
