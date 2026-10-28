package mangaupdates

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/luevano/libmangal/metadata"
)

// HTTPError: The API answered with an error (other than 429, see RateLimitError).
type HTTPError struct {
	Method     string
	Path       string
	StatusCode int
	// Message: The reason sent by the API, if any.
	Message string
}

func (e *HTTPError) Error() string {
	msg := fmt.Sprintf("mangaupdates: %s %s: %d %s", e.Method, e.Path, e.StatusCode, strings.ToLower(http.StatusText(e.StatusCode)))
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
	return fmt.Sprintf("mangaupdates: %s %s: 429 too many requests (retry after %s)", e.Method, e.Path, e.RetryAfter)
}

func (e *RateLimitError) Is(target error) bool {
	return target == metadata.ErrRateLimited
}

// RetryAfterDuration: How long to wait before retrying, for callers that don't import mangaupdates.
func (e *RateLimitError) RetryAfterDuration() time.Duration {
	return e.RetryAfter
}

// errorResponse is the body of errored requests (ApiResponseV1).
type errorResponse struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}
