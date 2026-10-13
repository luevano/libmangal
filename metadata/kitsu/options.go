package kitsu

import (
	"net/http"

	"github.com/luevano/libmangal/logger"
	"github.com/luevano/libmangal/netutil"
)

// DefaultUserAgent is sent when Options.UserAgent is empty, Cloudflare challenges
// some agents (curl's) on the login endpoint.
const DefaultUserAgent = netutil.LibraryUserAgent

// Options is options for Kitsu client.
type Options struct {
	// HTTPClient is a http client used for Kitsu API.
	HTTPClient *http.Client

	// UserAgent sent with every request, DefaultUserAgent if empty.
	UserAgent string

	// ClientID and ClientSecret of the OAuth application, optional:
	// the password grant works without them.
	ClientID     string
	ClientSecret string

	// Logger used for logs progress.
	//
	// If Logger is nil, a new one will be created.
	Logger *logger.Logger
}

// DefaultOptions constructs default Kitsu Options.
func DefaultOptions() Options {
	return Options{
		HTTPClient: netutil.NewClient(),
		UserAgent:  DefaultUserAgent,
		Logger:     logger.NewLogger(),
	}
}
