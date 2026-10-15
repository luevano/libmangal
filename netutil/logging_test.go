package netutil

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestRedactURL(t *testing.T) {
	for in, want := range map[string]string{
		"https://host/a?b=1":                              "https://host/a?b=1",
		"https://host/a?access_token=t&b=1":               "https://host/a?access_token=REDACTED&b=1",
		"https://host/a?API_KEY=t":                        "https://host/a?API_KEY=REDACTED",
		"https://user:pass@host/a":                        "https://user:REDACTED@host/a",
		"https://user@host/a?token=x&client_secret=y&z=1": "https://user@host/a?client_secret=REDACTED&token=REDACTED&z=1",
	} {
		u, err := url.Parse(in)
		if err != nil {
			t.Fatal(err)
		}
		if got := RedactURL(u).String(); got != want {
			t.Errorf("%s: got %s, want %s", in, got, want)
		}
		if u.String() == want && in != want {
			t.Errorf("%s: the url itself was changed", in)
		}
	}
	u, _ := url.Parse("https://host/a?secret=s&title=1")
	if got := RedactURL(u, "secret").String(); got != "https://host/a?secret=REDACTED&title=1" {
		t.Errorf("custom params: got %s", got)
	}
}

func TestLoggingTransport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/missing" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	var lines []string
	logf := func(format string, a ...any) { lines = append(lines, fmt.Sprintf(format, a...)) }
	client := &http.Client{Transport: NewLoggingTransport(nil, logf, nil)}

	req, _ := http.NewRequest(http.MethodGet, server.URL+"/a?access_token=tok&b=1", nil)
	req.Header.Set("Authorization", "Bearer header-secret")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	resp, err = client.Get(server.URL + "/missing")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if len(lines) != 2 {
		t.Fatalf("got %d lines: %q", len(lines), lines)
	}
	want := "GET " + server.URL + "/a?access_token=REDACTED&b=1 200 "
	if !strings.HasPrefix(lines[0], want) || !strings.HasSuffix(lines[0], "s") {
		t.Errorf("got %q, want %q + duration", lines[0], want)
	}
	if !strings.HasPrefix(lines[1], "GET "+server.URL+"/missing 404 ") {
		t.Errorf("got %q", lines[1])
	}
	for _, line := range lines {
		if strings.Contains(line, "tok&") || strings.Contains(line, "header-secret") {
			t.Errorf("secret logged: %q", line)
		}
	}
}

func TestLoggingTransportError(t *testing.T) {
	var lines []string
	logf := func(format string, a ...any) { lines = append(lines, fmt.Sprintf(format, a...)) }
	failing := roundTripperFunc(func(*http.Request) (*http.Response, error) { return nil, errors.New("boom") })
	redact := func(u *url.URL) string { return "custom:" + u.Path }
	client := &http.Client{Transport: NewLoggingTransport(failing, logf, redact)}
	if _, err := client.Get("http://host/p?secret=s"); err == nil {
		t.Fatal("no error")
	}
	if len(lines) != 1 || !strings.HasPrefix(lines[0], "GET custom:/p: boom (") {
		t.Errorf("got %q", lines)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
