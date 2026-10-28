package mangaupdates

import (
	"net/http"

	"github.com/luevano/libmangal/logger"
	"github.com/luevano/libmangal/netutil"
)

// DefaultUserAgent is sent when Options.UserAgent is empty.
const DefaultUserAgent = netutil.LibraryUserAgent

// Options is options for the MangaUpdates client.
type Options struct {
	// HTTPClient is a http client used for the MangaUpdates API.
	HTTPClient *http.Client

	// UserAgent sent with every request, DefaultUserAgent if empty.
	UserAgent string

	// Logger used for logs progress.
	//
	// If Logger is nil, a new one will be created.
	Logger *logger.Logger
}

// DefaultOptions constructs default MangaUpdates Options.
func DefaultOptions() Options {
	return Options{
		HTTPClient: netutil.NewClient(),
		UserAgent:  DefaultUserAgent,
		Logger:     logger.NewLogger(),
	}
}
