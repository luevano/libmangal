// Package ratelimit has the rate limit handling shared by the metadata providers.
package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/luevano/libmangal/logger"
	"github.com/luevano/libmangal/netutil"
	"github.com/luevano/libmangal/progress"
)

// RetryAfter is the header's Retry-After, or fallback when there's none. A hint that isn't in
// the future counts as none.
func RetryAfter(header http.Header, fallback time.Duration) time.Duration {
	if wait := netutil.RetryAfter(header.Get("Retry-After")); wait > 0 {
		return wait
	}
	return fallback
}

// Retries is how many times a rate limited request is retried (after waiting).
const Retries = 1

// Retry calls send, and again after the wait it asks for while it fails with a
// netutil.RateLimited (a bare ErrRateLimited isn't retried), up to Retries times. The wait is logged and reported as source's progress (site is
// the name in the message, e.g. "AniList").
func Retry(ctx context.Context, l *logger.Logger, source, site string, send func() error) error {
	for retry := 0; ; retry++ {
		err := send()
		var rateLimit netutil.RateLimited
		if retry == Retries || !errors.As(err, &rateLimit) {
			return err
		}

		wait := rateLimit.RetryAfterDuration()
		msg := fmt.Sprintf("%s rate limited, retrying in %s", site, wait)
		l.Log("%s", msg)
		progress.Report(ctx, progress.Status{Source: source, Message: msg})
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
