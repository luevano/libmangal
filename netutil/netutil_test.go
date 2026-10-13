package netutil

import (
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
