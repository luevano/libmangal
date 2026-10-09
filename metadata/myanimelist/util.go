package myanimelist

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/luevano/libmangal/metadata"
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
	"anime_statistics",
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

func (n mangaNodes) GetAsMetas() []metadata.Metadata {
	mangas := make([]metadata.Metadata, len(n))
	for i, n := range n {
		mangas[i] = n.Node
	}
	return mangas
}

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
	u, _ := url.Parse(apiURL)
	u = u.JoinPath(path)
	u.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return err
	}

	if p.Authenticated() {
		headers.Set("Authorization", "Bearer "+p.token)
	} else {
		headers.Set("X-MAL-CLIENT-ID", p.options.ClientID)
	}
	req.Header = headers

	resp, err := p.options.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New(resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(&res)
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
