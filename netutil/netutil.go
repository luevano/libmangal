// Package netutil has the HTTP basics shared by libmangal, its metadata providers and
// the manga providers: default timeout and user agents, Retry-After parsing, request
// logging with redaction (LoggingTransport) and the error sentinels every provider error
// should match.
//
// It's a package of its own (libmangal re-exports the sentinels) so the metadata
// providers don't depend on libmangal.
package netutil

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Errors to check with errors.Is, libmangal.ErrNotFound and metadata.ErrNotFound are the
// same values (and so for ErrRateLimited). Provider errors should match them in their Is.
var (
	// ErrNotFound: The site doesn't have it (a 404, an unknown id).
	ErrNotFound = errors.New("not found")

	// ErrRateLimited: The site answered 429 (or its equivalent), see RetryAfter.
	ErrRateLimited = errors.New("rate limited")
)

const (
	// DefaultTimeout of the http clients libmangal creates (NewClient).
	DefaultTimeout = time.Minute

	// BrowserUserAgent is a current Firefox, for sites made for browsers.
	BrowserUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:158.0) Gecko/20100101 Firefox/158.0"

	// LibraryUserAgent is an honest user agent, for APIs (AniList, MyAnimeList, Kitsu).
	LibraryUserAgent = "libmangal (https://github.com/luevano/libmangal)"

	// MaxRetryAfter caps the waits asked by Retry-After (and similar) headers.
	MaxRetryAfter = 5 * time.Minute
)

// NewClient returns an http client with DefaultTimeout.
func NewClient() *http.Client {
	return &http.Client{Timeout: DefaultTimeout}
}

// RetryAfter parses a Retry-After header value, in seconds or an http-date, capped at
// MaxRetryAfter. It's zero for an empty or invalid value, or one that isn't in the future.
func RetryAfter(header string) time.Duration {
	header = strings.TrimSpace(header)
	if header == "" {
		return 0
	}
	var wait time.Duration
	if secs, err := strconv.ParseInt(header, 10, 64); err == nil {
		if secs > int64(MaxRetryAfter/time.Second) {
			return MaxRetryAfter
		}
		wait = time.Duration(secs) * time.Second
	} else if date, err := http.ParseTime(header); err == nil {
		wait = time.Until(date).Round(time.Second)
	}
	return CapRetryAfter(wait)
}

// CapRetryAfter keeps wait between zero and MaxRetryAfter.
func CapRetryAfter(wait time.Duration) time.Duration {
	return min(max(wait, 0), MaxRetryAfter)
}

// RateLimited is implemented by rate limit errors that say how long to wait
// (RateLimitError, the metadata providers' and the manga providers' RateLimitError).
type RateLimited interface {
	RetryAfterDuration() time.Duration
}

// RetryAfterOf is whether err (or one it wraps) is a rate limit (a RateLimited, or
// ErrRateLimited) and how long to wait: zero when the site didn't say.
func RetryAfterOf(err error) (time.Duration, bool) {
	var rateLimit RateLimited
	if errors.As(err, &rateLimit) {
		return rateLimit.RetryAfterDuration(), true
	}
	return 0, errors.Is(err, ErrRateLimited)
}

// RateLimitError: The site answered 429 too many requests. It matches ErrRateLimited.
type RateLimitError struct {
	// RetryAfter: How long the site asked to wait, 0 if it didn't say.
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("429 too many requests (retry after %s)", e.RetryAfter)
	}
	return "429 too many requests"
}

func (e *RateLimitError) Is(target error) bool {
	return target == ErrRateLimited
}

// RetryAfterDuration: How long to wait before trying again, for callers that don't import
// netutil.
func (e *RateLimitError) RetryAfterDuration() time.Duration {
	return e.RetryAfter
}
