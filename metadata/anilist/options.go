package anilist

import (
	"net/http"

	"github.com/luevano/libmangal/logger"
	"github.com/luevano/libmangal/netutil"
)

// DefaultUserAgent is sent when Options.UserAgent is empty.
const DefaultUserAgent = netutil.LibraryUserAgent

// Options is options for Anilist client.
type Options struct {
	// HTTPClient is a http client used for Anilist API.
	HTTPClient *http.Client

	// UserAgent sent with every request, DefaultUserAgent if empty.
	UserAgent string

	// TagRankMin: Tags less relevant to the manga than this (AniList's rank, 1 to 100) are
	// left out of Tags, 1 keeps them all. 0 is DefaultTagRankMin.
	TagRankMin int

	// Logger used for logs progress.
	//
	// If Logger is nil, a new one will be created.
	Logger *logger.Logger
}

// DefaultOptions constructs default Anilist Options.
func DefaultOptions() Options {
	return Options{
		HTTPClient: netutil.NewClient(),
		UserAgent:  DefaultUserAgent,
		TagRankMin: DefaultTagRankMin,
		Logger:     logger.NewLogger(),
	}
}
