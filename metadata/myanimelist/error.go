package myanimelist

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/luevano/libmangal/metadata"
)

// HTTPError: The API answered with a non-200 status code (other than 429, see RateLimitError).
type HTTPError struct {
	Method     string
	Path       string
	StatusCode int
	// Message: Details sent by the API, if any (e.g. "invalid_token").
	Message string
}

func (e *HTTPError) Error() string {
	msg := fmt.Sprintf("mal: %s %s: %d %s", e.Method, e.Path, e.StatusCode, strings.ToLower(http.StatusText(e.StatusCode)))
	if e.Message != "" {
		msg += ": " + e.Message
	}
	return msg
}

// ErrInvalidClientID: MyAnimeList doesn't know the API client id (Options.ClientID).
var ErrInvalidClientID = errors.New("mal: invalid client id")

func (e *HTTPError) Is(target error) bool {
	if target == ErrInvalidClientID {
		msg := strings.ToLower(e.Message)
		return e.StatusCode == http.StatusBadRequest && (strings.Contains(msg, "invalid client id") || strings.Contains(msg, "invalid_client"))
	}
	switch e.StatusCode {
	case http.StatusNotFound:
		return target == metadata.ErrNotFound
	case http.StatusUnauthorized:
		return target == metadata.ErrUnauthorized
	}
	return false
}

// RateLimitError: The API answered 429.
type RateLimitError struct {
	Method     string
	Path       string
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("mal: %s %s: 429 too many requests (retry after %s)", e.Method, e.Path, e.RetryAfter)
}

func (e *RateLimitError) Is(target error) bool {
	return target == metadata.ErrRateLimited
}

// RetryAfterDuration: How long to wait before retrying, for callers that don't import myanimelist.
func (e *RateLimitError) RetryAfterDuration() time.Duration {
	return e.RetryAfter
}

// errorResponse is the body of errored requests, e.g. {"error": "not_found", "message": ""}.
type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func (e errorResponse) String() string {
	switch {
	case e.Message == "":
		return e.Error
	case e.Error == "":
		return e.Message
	default:
		return e.Error + ": " + e.Message
	}
}

func retryAfter(header http.Header) time.Duration {
	if secs, err := strconv.Atoi(header.Get("Retry-After")); err == nil {
		return time.Duration(secs) * time.Second
	}
	// no documented limits
	return time.Minute
}
