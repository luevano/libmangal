package ratelimit

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/luevano/libmangal/logger"
)

type limited struct{ wait time.Duration }

func (e limited) Error() string                     { return "429" }
func (e limited) RetryAfterDuration() time.Duration { return e.wait }

func TestRetry(t *testing.T) {
	calls := 0
	err := Retry(context.Background(), logger.NewLogger(), "x", "X", func() error {
		calls++
		return limited{time.Millisecond}
	})
	if !errors.As(err, new(limited)) || calls != Retries+1 {
		t.Errorf("got %v after %d calls", err, calls)
	}

	calls = 0
	other := errors.New("500")
	if err := Retry(context.Background(), logger.NewLogger(), "x", "X", func() error { calls++; return other }); err != other || calls != 1 {
		t.Errorf("got %v after %d calls, want no retry", err, calls)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Retry(ctx, logger.NewLogger(), "x", "X", func() error { return limited{time.Hour} }); !errors.Is(err, context.Canceled) {
		t.Errorf("got %v, want canceled", err)
	}
}

func TestRetryAfter(t *testing.T) {
	if got := RetryAfter(http.Header{"Retry-After": {"0"}}, time.Minute); got != time.Minute {
		t.Errorf("Retry-After 0: %s, want the fallback", got)
	}
	if got := RetryAfter(http.Header{"Retry-After": {"5"}}, time.Minute); got != 5*time.Second {
		t.Errorf("Retry-After 5: %s, want 5s", got)
	}
}
