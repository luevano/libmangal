package myanimelist

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/luevano/libmangal/logger"
	"github.com/luevano/libmangal/metadata"
)

// Reference docs:
// https://myanimelist.net/apiconfig/references/api/v2

const apiURL = "https://api.myanimelist.net/v2"

var info = metadata.ProviderInfo{
	ID:      "myanimelist",
	Code:    metadata.IDCodeMyAnimeList,
	Source:  metadata.IDSourceMyAnimeList,
	Name:    "MyAnimeList",
	Version: "0.1.0",
	Website: "https://myanimelist.net/",
}

var _ metadata.Provider = (*MyAnimeList)(nil)

// MyAnimeList is a metadata.Provider implementation for MyAnimeList.
type MyAnimeList struct {
	// authenticated user info
	user  metadata.User
	token string

	options Options
	logger  *logger.Logger
}

// NewMAL constructs new MyAnimeList client.
func NewMAL(options Options) (*MyAnimeList, error) {
	if options.ClientID == "" {
		return nil, Error("invalid options: empty ClientID")
	}
	if options.HTTPClient == nil {
		return nil, Error("invalid options: nil HTTPClient")
	}

	// ensure the used logger is not nil
	l := options.Logger
	if l == nil {
		l = logger.NewLogger()
	}
	mal := &MyAnimeList{
		options: options,
		logger:  l,
	}

	return mal, nil
}

func (p *MyAnimeList) String() string {
	return info.Name
}

// Info information about Provider.
func (p *MyAnimeList) Info() metadata.ProviderInfo {
	return info
}

// SetLogger sets logger to use for this provider.
func (p *MyAnimeList) SetLogger(_logger *logger.Logger) {
	p.logger = _logger
}

// Logger returns the set logger.
//
// Always returns a non-nil logger.
func (p *MyAnimeList) Logger() *logger.Logger {
	return p.logger
}

// SearchByID for metadata with the given id.
// Implementation should only handle the request and and marshaling.
func (p *MyAnimeList) SearchByID(ctx context.Context, id int) (metadata.Metadata, bool, error) {
	params := p.commonMangaReqParams()
	params.Set("manga_id", strconv.Itoa(id))

	var manga *Manga
	err := p.request(ctx, http.MethodGet, "manga/"+strconv.Itoa(id), params, p.commonMangaReqHeaders(), nil, &manga)
	if err != nil {
		return nil, false, err
	}

	if manga == nil {
		return nil, false, nil
	}

	return manga, true, nil
}

// Search for metadata with the given query.
//
// Implementation should only handle the request and and marshaling.
func (p *MyAnimeList) Search(ctx context.Context, query string) ([]metadata.Metadata, error) {
	params := p.commonMangaReqParams()
	params.Set("q", query)
	params.Set("offset", "0")
	params.Set("limit", "30")

	var res mangasResponse
	err := p.request(ctx, http.MethodGet, "manga", params, p.commonMangaReqHeaders(), nil, &res)
	if err != nil {
		return nil, err
	}

	mangas := res.Data.GetAsMetas()
	p.logger.Log("found %d manga(s) on MyAnimeList", len(mangas))
	return mangas, nil
}

// SetMangaProgress sets the reading progress for a given manga metadata id.
func (p *MyAnimeList) SetMangaProgress(ctx context.Context, id, chapterNumber int) error {
	if id == 0 {
		return Error("MyAnimeList ID not valid (0)")
	}
	if !p.Authenticated() {
		return Error("not authorized")
	}

	path := fmt.Sprintf("manga/%d/my_list_status", id)

	headers := http.Header{}
	headers.Set("Content-Type", "application/x-www-form-urlencoded")

	params := url.Values{}
	params.Set("num_chapters_read", strconv.Itoa(chapterNumber))
	body := strings.NewReader(params.Encode())

	// currently the read status is not used/needed
	var readStatus *ReadStatus
	err := p.request(ctx, http.MethodPatch, path, url.Values{}, headers, body, &readStatus)
	if err != nil {
		return err
	}

	return nil
}

// User returns the currently authenticated user.
//
// nil User means non-authenticated.
func (p *MyAnimeList) User() metadata.User {
	return p.user
}
