package myanimelist

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/luevano/libmangal/metadata/internal/ratelimit"
)

var mangaFields = strings.Join([]string{
	"id",
	"title",
	"main_picture",
	"alternative_titles",
	"start_date",
	"end_date",
	"synopsis",
	"mean",
	"rank",
	"popularity",
	"nsfw",
	"genres",
	"created_at",
	"updated_at",
	"media_type",
	"status",
	"num_volumes",
	"num_chapters",
	"authors{first_name,last_name}",
}, ",")

var userFields = strings.Join([]string{
	"id",
	"name",
	"picture",
	"gender",
	"birthday",
	"location",
	"joined_at",
	"time_zone",
	"is_supporter",
}, ",")

type mangasResponse struct {
	Data   mangaNodes `json:"data"`
	Paging struct {
		Previous string `json:"previous"`
		Next     string `json:"next"`
	} `json:"paging"`
}

type mangaNodes []mangaNode

type mangaNode struct {
	Node *Manga `json:"node"`
}

// maxErrorBody is how much of an error response is read.
const maxErrorBody = 1 << 16

// request sends a request to the API, retrying once when rate limited.
func (p *MyAnimeList) request(
	ctx context.Context,
	method string,
	path string,
	params url.Values,
	headers http.Header,
	body io.Reader,
	res any,
) error {
	var payload []byte
	if body != nil {
		var err error
		if payload, err = io.ReadAll(body); err != nil {
			return fmt.Errorf("mal: %s %s: read request body: %w", method, path, err)
		}
	}
	return ratelimit.Retry(ctx, p.logger, info.ID, "MyAnimeList", func() error {
		var body io.Reader
		if payload != nil {
			body = bytes.NewReader(payload)
		}
		return p.requestOnce(ctx, method, path, params, headers.Clone(), body, res)
	})
}

func (p *MyAnimeList) requestOnce(
	ctx context.Context,
	method string,
	path string,
	params url.Values,
	headers http.Header,
	body io.Reader,
	res any,
) error {
	u, err := url.Parse(p.apiURL)
	if err != nil {
		return fmt.Errorf("mal: %w", err)
	}
	u = u.JoinPath(path)
	u.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return fmt.Errorf("mal: %s %s: %w", method, u.Path, err)
	}

	if token := p.session.Token(); token != "" {
		headers.Set("Authorization", "Bearer "+token)
	} else {
		headers.Set("X-MAL-CLIENT-ID", p.options.ClientID)
	}
	req.Header = headers
	req.Header.Set("User-Agent", cmp.Or(p.options.UserAgent, DefaultUserAgent))

	resp, err := p.options.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("mal: %s %s: %w", method, u.Path, err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusTooManyRequests:
		// no documented limits
		return &RateLimitError{Method: method, Path: u.Path, RetryAfter: ratelimit.RetryAfter(resp.Header, time.Minute)}
	default:
		var er errorResponse
		_ = json.NewDecoder(io.LimitReader(resp.Body, maxErrorBody)).Decode(&er)
		return &HTTPError{Method: method, Path: u.Path, StatusCode: resp.StatusCode, Message: er.String()}
	}

	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return fmt.Errorf("mal: %s %s: decode response: %w", method, u.Path, err)
	}
	return nil
}

// commonMangaReqParams is a convenience method to get the common manga req params.
func (p *MyAnimeList) commonMangaReqParams() url.Values {
	params := url.Values{}
	params.Set("fields", mangaFields)
	if p.options.NSFW {
		params.Set("nsfw", "true")
	} else {
		params.Set("nsfw", "false")
	}
	return params
}

// commonMangaReqHeaders is a convenience method to get the common manga req headers.
func (p *MyAnimeList) commonMangaReqHeaders() http.Header {
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")
	return headers
}
