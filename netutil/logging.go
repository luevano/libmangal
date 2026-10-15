package netutil

import (
	"net/http"
	"net/url"
	"strings"
	"time"
)

// SensitiveParams are the query params RedactURL hides by default (any case).
var SensitiveParams = []string{
	"access_token", "refresh_token", "id_token", "token", "client_secret",
	"api_key", "apikey", "password",
}

// RedactURL returns a copy of u that can be logged: the user info's password and the
// values of params (SensitiveParams if none) are "REDACTED".
func RedactURL(u *url.URL, params ...string) *url.URL {
	if len(params) == 0 {
		params = SensitiveParams
	}
	redacted := *u
	if _, ok := u.User.Password(); ok {
		redacted.User = url.UserPassword(u.User.Username(), "REDACTED")
	}
	query := u.Query()
	changed := false
	for key := range query {
		for _, param := range params {
			if strings.EqualFold(key, param) {
				query[key] = []string{"REDACTED"}
				changed = true
			}
		}
	}
	if changed {
		redacted.RawQuery = query.Encode()
	}
	return &redacted
}

// LoggingTransport logs every request sent through Next: method, url (as Redact shows
// it), then the status or the error, and how long it took to get the headers. Headers and
// bodies are never logged (they carry tokens and cookies).
type LoggingTransport struct {
	// Next sends the requests, http.DefaultTransport if nil.
	Next http.RoundTripper
	// Logf logs a line.
	Logf func(format string, a ...any)
	// Redact shows a url, RedactURL with SensitiveParams if nil.
	Redact func(*url.URL) string
}

// NewLoggingTransport is a LoggingTransport, see its fields.
func NewLoggingTransport(next http.RoundTripper, logf func(format string, a ...any), redact func(*url.URL) string) *LoggingTransport {
	return &LoggingTransport{Next: next, Logf: logf, Redact: redact}
}

func (t *LoggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	next := t.Next
	if next == nil {
		next = http.DefaultTransport
	}
	start := time.Now()
	resp, err := next.RoundTrip(req)
	if t.Logf == nil {
		return resp, err
	}
	took := time.Since(start).Round(time.Millisecond)
	target := t.redact(req.URL)
	if err != nil {
		t.Logf("%s %s: %s (%s)", req.Method, target, err, took)
	} else {
		t.Logf("%s %s %d %s", req.Method, target, resp.StatusCode, took)
	}
	return resp, err
}

func (t *LoggingTransport) redact(u *url.URL) string {
	if t.Redact != nil {
		return t.Redact(u)
	}
	return RedactURL(u).String()
}
