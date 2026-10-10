package myanimelist

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
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

func (p *MyAnimeList) request(
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

	if p.Authenticated() {
		headers.Set("Authorization", "Bearer "+p.token)
	} else {
		headers.Set("X-MAL-CLIENT-ID", p.options.ClientID)
	}
	req.Header = headers

	resp, err := p.options.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("mal: %s %s: %w", method, u.Path, err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusTooManyRequests:
		return &RateLimitError{Method: method, Path: u.Path, RetryAfter: retryAfter(resp.Header)}
	default:
		var er errorResponse
		_ = json.NewDecoder(resp.Body).Decode(&er)
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
