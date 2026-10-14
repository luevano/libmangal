package libmangal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/luevano/libmangal/netutil"
)

// PageImageRequest is what FetchPageImage gets: a page image URL with the headers and
// cookies the site wants.
type PageImageRequest struct {
	URL     string
	Headers map[string]string
	Cookies map[string]string
}

// Defaults of FetchPageImageOptions.
const (
	// DefaultPageRetries: How many times a page is tried again after a 429.
	DefaultPageRetries = 3
	// DefaultPageRetryWait: The first wait after a 429 without Retry-After, doubled each time.
	DefaultPageRetryWait = 5 * time.Second
	// DefaultMaxPageRetryWait caps each wait, Retry-After included.
	DefaultMaxPageRetryWait = time.Minute
	// DefaultMaxPageImageSize: Larger images are an error.
	DefaultMaxPageImageSize int64 = 100 << 20
)

// FetchPageImageOptions of FetchPageImage, the zero value takes the defaults.
type FetchPageImageOptions struct {
	// Retries after a 429, DefaultPageRetries if 0, none if negative.
	Retries int

	// RetryWait is the first wait after a 429 without Retry-After, doubled each time,
	// DefaultPageRetryWait if 0.
	RetryWait time.Duration

	// MaxRetryWait caps each wait, DefaultMaxPageRetryWait if 0.
	MaxRetryWait time.Duration

	// MaxSize of the image, DefaultMaxPageImageSize if 0.
	MaxSize int64

	// OnRetry is called before each wait after a 429 (retry counts from 1).
	OnRetry func(retry int, wait time.Duration)

	// StatusError makes the error of a response other than 200 and 429 (its body is closed
	// after). Nil gives "GET <url>: 404 not found".
	StatusError func(request *http.Request, response *http.Response) error
}

func (o FetchPageImageOptions) withDefaults() FetchPageImageOptions {
	if o.Retries == 0 {
		o.Retries = DefaultPageRetries
	}
	o.Retries = max(o.Retries, 0)
	if o.RetryWait <= 0 {
		o.RetryWait = DefaultPageRetryWait
	}
	if o.MaxRetryWait <= 0 {
		o.MaxRetryWait = DefaultMaxPageRetryWait
	}
	if o.MaxSize <= 0 {
		o.MaxSize = DefaultMaxPageImageSize
	}
	return o
}

// FetchPageImage gets a page image with a GET, sending the request's headers and cookies.
// A 429 is tried again (FetchPageImageOptions.Retries), waiting Retry-After or a doubling
// wait; the last one is a *netutil.RateLimitError (matches ErrRateLimited) saying how long
// to wait. Other statuses than 200 are errors, so is an image over MaxSize.
func FetchPageImage(ctx context.Context, client *http.Client, page PageImageRequest, options FetchPageImageOptions) ([]byte, error) {
	options = options.withDefaults()
	wait := options.RetryWait
	for retry := 0; ; retry++ {
		image, after, err := fetchPageImage(ctx, client, page, options)
		if after < 0 {
			return image, err
		}
		if retry == options.Retries {
			// the caller may wait longer, as long as the next try here would have
			var rateLimit *netutil.RateLimitError
			if errors.As(err, &rateLimit) && rateLimit.RetryAfter == 0 {
				rateLimit.RetryAfter = min(wait, options.MaxRetryWait)
			}
			return image, err
		}
		if after == 0 {
			after = wait
			wait *= 2
		}
		after = min(after, options.MaxRetryWait)
		if options.OnRetry != nil {
			options.OnRetry(retry+1, after)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(after):
		}
	}
}

// fetchPageImage gets the page once; after is how long to wait before trying again after a
// 429 (0 when the site didn't say), -1 for any other outcome.
func fetchPageImage(ctx context.Context, client *http.Client, page PageImageRequest, options FetchPageImageOptions) (image []byte, after time.Duration, err error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, page.URL, nil)
	if err != nil {
		return nil, -1, err
	}
	for key, value := range page.Headers {
		request.Header.Set(key, value)
	}
	for key, value := range page.Cookies {
		request.AddCookie(&http.Cookie{Name: key, Value: value})
	}

	response, err := client.Do(request)
	if err != nil {
		return nil, -1, err
	}
	defer response.Body.Close()

	switch response.StatusCode {
	case http.StatusOK:
	case http.StatusTooManyRequests:
		after := netutil.RetryAfter(response.Header.Get("Retry-After"))
		return nil, after, fmt.Errorf("GET %s: %w", page.URL, &netutil.RateLimitError{RetryAfter: after})
	default:
		if options.StatusError != nil {
			return nil, -1, options.StatusError(request, response)
		}
		return nil, -1, fmt.Errorf("GET %s: %d %s", page.URL, response.StatusCode, strings.ToLower(http.StatusText(response.StatusCode)))
	}

	image, err = io.ReadAll(io.LimitReader(response.Body, options.MaxSize+1))
	if err != nil {
		return nil, -1, err
	}
	if int64(len(image)) > options.MaxSize {
		return nil, -1, fmt.Errorf("response over %d bytes", options.MaxSize)
	}
	return image, -1, nil
}
