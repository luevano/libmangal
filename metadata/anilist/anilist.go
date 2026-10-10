package anilist

import (
	"context"

	"github.com/luevano/libmangal/logger"
	"github.com/luevano/libmangal/metadata"
)

// Reference docs:
// https://anilist.github.io/ApiV2-GraphQL-Docs/

const apiURL = "https://graphql.anilist.co"

var info = metadata.ProviderInfo{
	ID:      "anilist",
	Code:    metadata.IDCodeAnilist,
	Source:  metadata.IDSourceAnilist,
	Name:    "Anilist",
	Version: "0.1.0",
	Website: "https://anilist.co/",
}

var _ metadata.Provider = (*Anilist)(nil)

// Anilist is the Anilist client.
type Anilist struct {
	// authenticated user info
	user  metadata.User
	token string

	options Options
	logger  *logger.Logger
}

// NewAnilist constructs new Anilist client.
func NewAnilist(options Options) (*Anilist, error) {
	if options.HTTPClient == nil {
		return nil, Error("invalid options: nil HTTPClient")
	}
	// ensure the used logger is not nil
	l := options.Logger
	if l == nil {
		l = logger.NewLogger()
	}
	anilist := &Anilist{
		options: options,
		logger:  l,
	}

	return anilist, nil
}

func (p *Anilist) String() string {
	return info.Name
}

// Info information about Provider.
func (p *Anilist) Info() metadata.ProviderInfo {
	return info
}

// SetLogger sets logger to use for this provider.
func (p *Anilist) SetLogger(_logger *logger.Logger) {
	p.logger = _logger
}

// Logger returns the set logger.
//
// Always returns a non-nil logger.
func (p *Anilist) Logger() *logger.Logger {
	return p.logger
}

// SearchByID for metadata with the given id.
//
// Implementation should only handle the request and and marshaling.
func (p *Anilist) SearchByID(ctx context.Context, id int) (metadata.Metadata, bool, error) {
	body := apiRequestBody{
		Query: querySearchByID,
		Variables: map[string]any{
			"id": id,
		},
	}
	data, err := sendRequest[byIDData](ctx, p, body)
	if err != nil {
		return nil, false, err
	}

	manga := data.Media
	if manga == nil {
		return nil, false, nil
	}

	return manga, true, nil
}

// Search for metadata with the given query.
//
// Implementation should only handle the request and and marshaling.
func (p *Anilist) Search(ctx context.Context, query string) ([]metadata.Metadata, error) {
	body := apiRequestBody{
		Query: querySearchByName,
		Variables: map[string]any{
			"query": query,
		},
	}
	data, err := sendRequest[mangasData](ctx, p, body)
	if err != nil {
		return nil, err
	}

	mangas := data.Page.Media.GetAsMetas()
	p.logger.Log("found %d manga(s) on Anilist", len(mangas))
	return mangas, nil
}

// SetMangaProgress sets the reading progress for a given manga metadata id.
func (p *Anilist) SetMangaProgress(ctx context.Context, id, chapterNumber int) error {
	if id == 0 {
		return Error("Anilist ID not valid (0)")
	}
	if !p.Authenticated() {
		return Error("not authorized")
	}

	body := apiRequestBody{
		Query: mutationSaveProgress,
		Variables: map[string]any{
			"id":       id,
			"progress": chapterNumber,
		},
	}
	_, err := sendRequest[setProgressData](ctx, p, body)
	if err != nil {
		return Error(err.Error())
	}

	return nil
}

// User returns the currently authenticated user.
//
// nil User means non-authenticated.
func (p *Anilist) User() metadata.User {
	return p.user
}
