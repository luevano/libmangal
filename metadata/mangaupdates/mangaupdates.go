// Package mangaupdates is a metadata.Provider for MangaUpdates (Baka-Updates Manga), read
// only: search and series by id, no login or reading progress.
//
// MangaUpdates asks to be credited where its data is shown.
package mangaupdates

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"time"

	"github.com/luevano/libmangal/logger"
	"github.com/luevano/libmangal/metadata"
	"github.com/luevano/libmangal/metadata/internal/ratelimit"
	"golang.org/x/time/rate"
)

// Reference docs: https://api.mangaupdates.com/ (openapi.yaml)

const (
	apiURL = "https://api.mangaupdates.com/v1"
	// searchPerPage is how many search results are asked for, they all come in one request.
	searchPerPage = 25
	// requestInterval: the API has no documented limit, its acceptable use policy asks for
	// "reasonable spacing between requests" and caching (https://api.mangaupdates.com/,
	// 2026-10-28), so one request a second.
	requestInterval = time.Second
)

var info = metadata.ProviderInfo{
	ID:          "mangaupdates",
	Code:        metadata.IDCodeMangaUpdates,
	Source:      metadata.IDSourceMangaUpdates,
	Name:        "MangaUpdates",
	Version:     "0.1.0",
	Description: "Metadata from MangaUpdates (Baka-Updates Manga)",
	Website:     "https://www.mangaupdates.com/",
}

var _ metadata.Provider = (*MangaUpdates)(nil)

// MangaUpdates is a metadata.Provider implementation for MangaUpdates. Its ids are the
// API's series ids (see metadata.MangaUpdatesID), which need a 64-bit int.
type MangaUpdates struct {
	options Options
	logger  *logger.Logger
	limiter *rate.Limiter
	// apiURL is only changed in tests
	apiURL string
}

// NewMangaUpdates constructs new MangaUpdates client.
func NewMangaUpdates(options Options) (*MangaUpdates, error) {
	if options.HTTPClient == nil {
		return nil, errors.New("mangaupdates: invalid options: nil HTTPClient")
	}
	l := options.Logger
	if l == nil {
		l = logger.NewLogger()
	}
	return &MangaUpdates{
		options: options,
		logger:  l,
		limiter: rate.NewLimiter(rate.Every(requestInterval), 1),
		apiURL:  apiURL,
	}, nil
}

func (p *MangaUpdates) String() string {
	return info.Name
}

// Info information about Provider.
func (p *MangaUpdates) Info() metadata.ProviderInfo {
	return info
}

// SetLogger sets logger to use for this provider.
func (p *MangaUpdates) SetLogger(_logger *logger.Logger) {
	p.logger = _logger
}

// Logger returns the set logger.
//
// Always returns a non-nil logger.
func (p *MangaUpdates) Logger() *logger.Logger {
	return p.logger
}

// SearchByID for metadata with the given id.
//
// Implementation should only handle the request and marshaling.
func (p *MangaUpdates) SearchByID(ctx context.Context, id int) (metadata.Metadata, bool, error) {
	var m Manga
	err := p.request(ctx, http.MethodGet, "series/"+strconv.Itoa(id), nil, &m)
	if errors.Is(err, metadata.ErrNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("get manga %d: %w", id, err)
	}
	if m.SeriesID == 0 {
		return nil, false, nil
	}
	return &m, true, nil
}

// skippedTypes are left out of searches: not comics, or doujinshi (fan works), which
// otherwise crowd out the series (23 of the first 25 of an "attack on titan" search).
var skippedTypes = []string{"Novel", "Drama CD", "Artbook", "Doujinshi"}

// Search for metadata with the given query, one request. The results are Partial: search
// records have no authors, status, categories or associated titles.
//
// Implementation should only handle the request and marshaling.
func (p *MangaUpdates) Search(ctx context.Context, query string) ([]metadata.Metadata, error) {
	var res struct {
		Results []struct {
			Record   Manga  `json:"record"`
			HitTitle string `json:"hit_title"`
		} `json:"results"`
	}
	body := map[string]any{"search": query, "perpage": searchPerPage, "filter_types": skippedTypes}
	if err := p.request(ctx, http.MethodPost, "series/search", body, &res); err != nil {
		return nil, fmt.Errorf("search %q: %w", query, err)
	}

	var metas []metadata.Metadata
	for _, r := range res.Results {
		m := r.Record
		// ids that don't fit an int (32-bit platforms) can't be looked up
		if m.SeriesID <= 0 || m.SeriesID > math.MaxInt || slices.Contains(skippedTypes, m.Type) {
			continue
		}
		m.SearchRecord, m.HitTitle = true, r.HitTitle
		metas = append(metas, &m)
	}
	p.logger.Log("found %d manga(s) on MangaUpdates", len(metas))
	return metas, nil
}

// SetMangaProgress isn't supported: errors.ErrUnsupported.
func (p *MangaUpdates) SetMangaProgress(context.Context, int, int) error {
	return fmt.Errorf("set progress: mangaupdates: %w", errors.ErrUnsupported)
}

// Authenticated is always false, there's no login.
func (p *MangaUpdates) Authenticated() bool {
	return false
}

// User is always nil, there's no login.
func (p *MangaUpdates) User() metadata.User {
	return nil
}

// Login isn't supported: errors.ErrUnsupported.
func (p *MangaUpdates) Login(context.Context, string) error {
	return fmt.Errorf("login: mangaupdates: %w", errors.ErrUnsupported)
}

// Logout does nothing, there's no login.
func (p *MangaUpdates) Logout() error {
	return nil
}

// maxErrorBody is how much of an error response is read.
const maxErrorBody = 1 << 16

// request sends a request to the API (path relative to apiURL), body (if non-nil) as
// JSON, and decodes the response into res. A rate limited request is retried once.
func (p *MangaUpdates) request(ctx context.Context, method, path string, body, res any) error {
	u, err := url.Parse(p.apiURL)
	if err != nil {
		return fmt.Errorf("mangaupdates: %w", err)
	}
	u = u.JoinPath(path)
	var payload []byte
	if body != nil {
		if payload, err = json.Marshal(body); err != nil {
			return fmt.Errorf("mangaupdates: %s %s: encode request: %w", method, u.Path, err)
		}
	}
	return ratelimit.Retry(ctx, p.logger, info.ID, "MangaUpdates", func() error {
		if err := p.limiter.Wait(ctx); err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, method, u.String(), bytes.NewReader(payload))
		if err != nil {
			return fmt.Errorf("mangaupdates: %w", err)
		}
		return p.send(req, body != nil, res)
	})
}

func (p *MangaUpdates) send(req *http.Request, hasBody bool, res any) error {
	userAgent := p.options.UserAgent
	if userAgent == "" {
		userAgent = DefaultUserAgent
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")
	if hasBody {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := p.options.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("mangaupdates: %s %s: %w", req.Method, req.URL.Path, err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		return &RateLimitError{Method: req.Method, Path: req.URL.Path, RetryAfter: ratelimit.RetryAfter(resp.Header, time.Minute)}
	case resp.StatusCode < 200 || resp.StatusCode > 299:
		httpErr := &HTTPError{Method: req.Method, Path: req.URL.Path, StatusCode: resp.StatusCode}
		var er errorResponse
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
		if json.Unmarshal(raw, &er) == nil {
			httpErr.Message = er.Reason
		}
		return httpErr
	}
	if err := json.NewDecoder(resp.Body).Decode(res); err != nil {
		return fmt.Errorf("mangaupdates: %s %s: decode response: %w", req.Method, req.URL.Path, err)
	}
	return nil
}
