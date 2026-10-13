package kitsu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/luevano/libmangal/metadata/internal/ratelimit"
)

const contentType = "application/vnd.api+json"

// document is a JSON:API response, https://jsonapi.org/format/#document-structure.
type document struct {
	// Data is a resource or a list of them.
	Data     json.RawMessage `json:"data"`
	Included []resource      `json:"included"`
}

type resource struct {
	ID            string                  `json:"id"`
	Type          string                  `json:"type"`
	Attributes    json.RawMessage         `json:"attributes"`
	Relationships map[string]relationship `json:"relationships"`
}

type ref struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

type relationship struct {
	// Data is a ref, a list of refs or null, only present for included relationships.
	Data json.RawMessage `json:"data"`
}

func (r relationship) refs() []ref {
	data := bytes.TrimSpace(r.Data)
	switch {
	case len(data) == 0 || bytes.Equal(data, []byte("null")):
		return nil
	case data[0] == '[':
		var refs []ref
		_ = json.Unmarshal(data, &refs)
		return refs
	default:
		var one ref
		if json.Unmarshal(data, &one) != nil {
			return nil
		}
		return []ref{one}
	}
}

// resources decodes the document's data, a single resource or a list.
func (d document) resources() ([]resource, error) {
	data := bytes.TrimSpace(d.Data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		return nil, nil
	}
	if data[0] != '[' {
		data = append(append([]byte{'['}, data...), ']')
	}
	var res []resource
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, err
	}
	return res, nil
}

// includedIndex finds the document's included resources by type and id.
type includedIndex map[ref]resource

func (d document) index() includedIndex {
	idx := make(includedIndex, len(d.Included))
	for _, r := range d.Included {
		idx[ref{ID: r.ID, Type: r.Type}] = r
	}
	return idx
}

// related is the included resources of r's relationship.
func (idx includedIndex) related(r resource, name string) []resource {
	var out []resource
	for _, rf := range r.Relationships[name].refs() {
		if inc, ok := idx[rf]; ok {
			out = append(out, inc)
		}
	}
	return out
}

// request sends a request to the API (path relative to apiURL) and decodes
// the response into res, if non-nil. body, if non-nil, is sent as JSON.
func (p *Kitsu) request(ctx context.Context, method, path string, params url.Values, body, res any) error {
	u, err := url.Parse(p.apiURL)
	if err != nil {
		return fmt.Errorf("kitsu: %w", err)
	}
	u = u.JoinPath(path)
	u.RawQuery = params.Encode()

	var payload []byte
	if body != nil {
		if payload, err = json.Marshal(body); err != nil {
			return fmt.Errorf("kitsu: %s %s: encode request: %w", method, u.Path, err)
		}
	}

	return p.send(ctx, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, method, u.String(), bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", contentType)
		if body != nil {
			req.Header.Set("Content-Type", contentType)
		}
		if p.Authenticated() {
			req.Header.Set("Authorization", "Bearer "+p.token)
		}
		return req, nil
	}, res)
}

// send sends the request built by newReq, retrying once when rate limited.
func (p *Kitsu) send(ctx context.Context, newReq func() (*http.Request, error), res any) error {
	return ratelimit.Retry(ctx, p.logger, info.ID, "Kitsu", func() error {
		return p.sendOnce(newReq, res)
	})
}

func (p *Kitsu) sendOnce(newReq func() (*http.Request, error), res any) error {
	req, err := newReq()
	if err != nil {
		return fmt.Errorf("kitsu: %w", err)
	}
	userAgent := p.options.UserAgent
	if userAgent == "" {
		userAgent = DefaultUserAgent
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := p.options.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("kitsu: %s %s: %w", req.Method, req.URL.Path, err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		return &RateLimitError{Method: req.Method, Path: req.URL.Path, RetryAfter: retryAfter(resp.Header)}
	case resp.StatusCode < 200 || resp.StatusCode > 299:
		var er errorResponse
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		httpErr := &HTTPError{Method: req.Method, Path: req.URL.Path, StatusCode: resp.StatusCode}
		if json.Unmarshal(raw, &er) == nil {
			httpErr.Code, httpErr.Message = er.codeMessage()
		} else if strings.Contains(string(raw), "challenge-platform") {
			httpErr.Message = "blocked by a cloudflare challenge"
		}
		return httpErr
	}

	if res == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(res); err != nil {
		return fmt.Errorf("kitsu: %s %s: decode response: %w", req.Method, req.URL.Path, err)
	}
	return nil
}
