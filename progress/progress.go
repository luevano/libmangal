// Package progress carries a status reporter in a context, so long work (requests,
// rate-limit waits, pagination) can say what it's doing without printing anything.
package progress

import (
	"context"
	"fmt"
)

// Status is what some work is doing now.
type Status struct {
	// Source is who reports it, e.g. a provider id, may be empty.
	Source string
	// Message says what's being done, e.g. "AniList rate limited, retrying in 58s".
	Message string
	// Done and Total count the steps (e.g. request 3 of 7), Total is 0 when there's no count.
	Done, Total int
}

// String is the message, followed by the count when there's one.
func (s Status) String() string {
	if s.Total > 0 {
		return fmt.Sprintf("%s %d/%d", s.Message, s.Done, s.Total)
	}
	return s.Message
}

type reporterKey struct{}

// WithReporter returns a context whose Report calls go to report. It may be called from
// several goroutines at once.
func WithReporter(ctx context.Context, report func(Status)) context.Context {
	return context.WithValue(ctx, reporterKey{}, report)
}

// Report sends the status to the reporter of ctx, if any.
func Report(ctx context.Context, status Status) {
	if report, ok := ctx.Value(reporterKey{}).(func(Status)); ok && report != nil {
		report(status)
	}
}
