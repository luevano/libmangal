package kitsu

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/luevano/libmangal/metadata"
)

// HTTPError: The API answered with an error (other than 429, see RateLimitError).
type HTTPError struct {
	Method     string
	Path       string
	StatusCode int
	// Code: The JSON:API error code, or the OAuth error (e.g. "invalid_grant").
	Code string
	// Message: Details sent by the API, if any.
	Message string
}

func (e *HTTPError) Error() string {
	msg := fmt.Sprintf("kitsu: %s %s: %d %s", e.Method, e.Path, e.StatusCode, strings.ToLower(http.StatusText(e.StatusCode)))
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
		// what the token endpoint answers for a wrong password or refresh token
		return target == metadata.ErrUnauthorized && e.Code == "invalid_grant"
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
	return fmt.Sprintf("kitsu: %s %s: 429 too many requests (retry after %s)", e.Method, e.Path, e.RetryAfter)
}

func (e *RateLimitError) Is(target error) bool {
	return target == metadata.ErrRateLimited
}

// RetryAfterDuration: How long to wait before retrying, for callers that don't import kitsu.
func (e *RateLimitError) RetryAfterDuration() time.Duration {
	return e.RetryAfter
}

// errorResponse is the body of errored requests: JSON:API errors
// ({"errors": [{"title": "Record not found", "detail": "...", "code": "404"}]})
// or OAuth ones ({"error": "invalid_grant", "error_description": "..."}).
type errorResponse struct {
	Errors []struct {
		Title  string `json:"title"`
		Detail string `json:"detail"`
		Code   string `json:"code"`
	} `json:"errors"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func (e errorResponse) codeMessage() (code, message string) {
	if e.Error != "" {
		return e.Error, e.Error + ": " + e.ErrorDescription
	}
	if len(e.Errors) == 0 {
		return "", ""
	}
	first := e.Errors[0]
	message = first.Detail
	if message == "" || message == first.Title {
		message = first.Title
	}
	return first.Code, message
}

func retryAfter(header http.Header) time.Duration {
	if secs, err := strconv.Atoi(header.Get("Retry-After")); err == nil {
		return time.Duration(secs) * time.Second
	}
	// no documented limits
	return time.Minute
}
