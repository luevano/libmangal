package libmangal

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/luevano/libmangal/netutil"
)

func TestFetchPageImageRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, _ := r.Cookie("session")
		if r.Header.Get("Referer") != "https://site.test/" || cookie == nil || cookie.Value != "s" {
			t.Errorf("headers %v", r.Header)
		}
		_, _ = w.Write([]byte("png"))
	}))
	defer server.Close()

	image, err := FetchPageImage(context.Background(), server.Client(), PageImageRequest{
		URL:     server.URL + "/1.png",
		Headers: map[string]string{"Referer": "https://site.test/"},
		Cookies: map[string]string{"session": "s"},
	}, FetchPageImageOptions{})
	if err != nil || string(image) != "png" {
		t.Errorf("got %q, %v", image, err)
	}
}

func TestFetchPageImageStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	page := PageImageRequest{URL: server.URL + "/1.png"}
	_, err := FetchPageImage(context.Background(), server.Client(), page, FetchPageImageOptions{})
	want := "GET " + server.URL + "/1.png: 404 not found"
	if err == nil || err.Error() != want {
		t.Errorf("wanted %q, got %v", want, err)
	}

	errStatus := errors.New("status")
	_, err = FetchPageImage(context.Background(), server.Client(), page, FetchPageImageOptions{
		StatusError: func(_ *http.Request, response *http.Response) error {
			if response.StatusCode != http.StatusNotFound {
				t.Errorf("status %d", response.StatusCode)
			}
			return errStatus
		},
	})
	if !errors.Is(err, errStatus) {
		t.Errorf("got %v, want StatusError's", err)
	}
}

func TestFetchPageImageMaxSize(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("12345"))
	}))
	defer server.Close()

	page := PageImageRequest{URL: server.URL + "/1.png"}
	if image, err := FetchPageImage(context.Background(), server.Client(), page, FetchPageImageOptions{MaxSize: 5}); err != nil || string(image) != "12345" {
		t.Errorf("at the cap: %q, %v", image, err)
	}
	if _, err := FetchPageImage(context.Background(), server.Client(), page, FetchPageImageOptions{MaxSize: 4}); err == nil {
		t.Error("over the cap: wanted an error")
	}
}

func TestFetchPageImageRetries(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch calls.Add(1) {
		case 1:
			w.WriteHeader(http.StatusTooManyRequests)
		case 2:
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
		default:
			_, _ = w.Write([]byte("png"))
		}
	}))
	defer server.Close()

	var waits []time.Duration
	options := FetchPageImageOptions{
		RetryWait: time.Millisecond,
		OnRetry:   func(retry int, wait time.Duration) { waits = append(waits, wait) },
	}
	start := time.Now()
	image, err := FetchPageImage(context.Background(), server.Client(), PageImageRequest{URL: server.URL + "/1.png"}, options)
	if err != nil || string(image) != "png" || calls.Load() != 3 {
		t.Fatalf("got %q, %v after %d calls", image, err, calls.Load())
	}
	if time.Since(start) < time.Second {
		t.Errorf("didn't wait the Retry-After")
	}
	if len(waits) != 2 || waits[0] != time.Millisecond || waits[1] != time.Second {
		t.Errorf("reported waits %v", waits)
	}

	// it gives up after the retries
	calls.Store(0)
	always := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer always.Close()
	page := PageImageRequest{URL: always.URL + "/1.png"}
	options.OnRetry = nil
	_, err = FetchPageImage(context.Background(), always.Client(), page, options)
	if !errors.Is(err, ErrRateLimited) || calls.Load() != DefaultPageRetries+1 {
		t.Errorf("always 429: %v after %d calls", err, calls.Load())
	}
	var rateLimit *netutil.RateLimitError
	if !errors.As(err, &rateLimit) || rateLimit.RetryAfterDuration() <= 0 {
		t.Errorf("the rate limit should say how long to wait: %v", err)
	}

	calls.Store(0)
	if _, err := FetchPageImage(context.Background(), always.Client(), page, FetchPageImageOptions{Retries: -1}); !errors.Is(err, ErrRateLimited) || calls.Load() != 1 {
		t.Errorf("no retries: %v after %d calls", err, calls.Load())
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := FetchPageImage(ctx, always.Client(), page, options); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled: %v", err)
	}
}
