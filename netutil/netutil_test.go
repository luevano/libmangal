package netutil

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestRetryAfter(t *testing.T) {
	for header, want := range map[string]time.Duration{
		"":                              0,
		"x":                             0,
		"0":                             0,
		"-5":                            0,
		"30":                            30 * time.Second,
		" 30 ":                          30 * time.Second,
		"3600":                          MaxRetryAfter,
		"99999999999999999999":          0, // overflows, invalid
		"9999999999999":                 MaxRetryAfter,
		"Wed, 21 Oct 2015 07:28:00 GMT": 0,
		time.Now().Add(90 * time.Second).UTC().Format(http.TimeFormat): 90 * time.Second,
		time.Now().Add(time.Hour).UTC().Format(http.TimeFormat):        MaxRetryAfter,
	} {
		got := RetryAfter(header)
		if d := got - want; d < -time.Second || d > time.Second {
			t.Errorf("%q: got %s, want %s", header, got, want)
		}
	}
}

func TestNewClient(t *testing.T) {
	if c := NewClient(); c.Timeout != DefaultTimeout {
		t.Errorf("timeout %s", c.Timeout)
	}
}

type rateLimited struct{ wait time.Duration }

func (e rateLimited) Error() string                     { return "429" }
func (e rateLimited) RetryAfterDuration() time.Duration { return e.wait }

func TestRetryAfterOf(t *testing.T) {
	err := fmt.Errorf("download chapter: %w", fmt.Errorf("list chapters: %w", rateLimited{30 * time.Second}))
	if wait, ok := RetryAfterOf(err); !ok || wait != 30*time.Second {
		t.Errorf("wanted 30s through the wrapping, got %s, %v", wait, ok)
	}
	if wait, ok := RetryAfterOf(&RateLimitError{RetryAfter: time.Minute}); !ok || wait != time.Minute {
		t.Errorf("RateLimitError: %s, %v", wait, ok)
	}
	if _, ok := RetryAfterOf(errors.New("some other error")); ok {
		t.Error("other errors aren't rate limits")
	}
	if _, ok := RetryAfterOf(nil); ok {
		t.Error("nil isn't a rate limit")
	}
	if wait, ok := RetryAfterOf(fmt.Errorf("get pages: %w", ErrRateLimited)); !ok || wait != 0 {
		t.Errorf("the sentinel: %s, %v", wait, ok)
	}
}
