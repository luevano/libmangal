package anilist

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/luevano/libmangal/metadata"
	"github.com/luevano/libmangal/netutil"
)

// HTTPError: The API answered with an error (other than 429, see RateLimitError).
type HTTPError struct {
	StatusCode int
	// Message: Details sent by the API, if any.
	Message string
}

func (e *HTTPError) Error() string {
	msg := fmt.Sprintf("anilist: %d %s", e.StatusCode, strings.ToLower(http.StatusText(e.StatusCode)))
	if e.Message != "" {
		msg += ": " + e.Message
	}
	return msg
}

func (e *HTTPError) Is(target error) bool {
	switch e.StatusCode {
	case http.StatusNotFound:
		return target == metadata.ErrNotFound
	case http.StatusUnauthorized:
		return target == metadata.ErrUnauthorized
	case http.StatusBadRequest:
		// what AniList answers for an expired or revoked token
		return target == metadata.ErrUnauthorized && e.Message == "Invalid token"
	}
	return false
}

// RateLimitError: The API answered 429.
type RateLimitError struct {
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("anilist: 429 too many requests (retry after %s)", e.RetryAfter)
}

func (e *RateLimitError) Is(target error) bool {
	return target == metadata.ErrRateLimited
}

// RetryAfterDuration: How long to wait before retrying, for callers that don't import anilist.
func (e *RateLimitError) RetryAfterDuration() time.Duration {
	return e.RetryAfter
}

// retryAfter: How long to wait after a 429.
//
// https://docs.anilist.co/guide/rate-limiting
//
// AniList sometimes answers "Retry-After: 0" while still limiting: a hint that isn't in the
// future counts as none.
func retryAfter(header http.Header) time.Duration {
	if wait := netutil.RetryAfter(header.Get("Retry-After")); wait > 0 {
		return wait
	}
	if reset, err := strconv.ParseInt(header.Get("X-RateLimit-Reset"), 10, 64); err == nil {
		if wait := netutil.CapRetryAfter(time.Until(time.Unix(reset, 0)).Round(time.Second)); wait > 0 {
			return wait
		}
	}
	// no hint, the limit is per minute
	return time.Minute
}
