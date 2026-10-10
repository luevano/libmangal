package anilist

import (
	"context"
	"errors"
	"fmt"

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
	// apiURL is only changed in tests
	apiURL string
}

// NewAnilist constructs new Anilist client.
func NewAnilist(options Options) (*Anilist, error) {
	if options.HTTPClient == nil {
		return nil, errors.New("anilist: invalid options: nil HTTPClient")
	}
	// ensure the used logger is not nil
	l := options.Logger
	if l == nil {
		l = logger.NewLogger()
	}
	anilist := &Anilist{
		options: options,
		logger:  l,
		apiURL:  apiURL,
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
	if errors.Is(err, metadata.ErrNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("get manga %d: %w", id, err)
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
		return nil, fmt.Errorf("search %q: %w", query, err)
	}

	mangas := data.Page.Media.GetAsMetas()
	p.logger.Log("found %d manga(s) on Anilist", len(mangas))
	return mangas, nil
}

// SetMangaProgress sets the reading progress for a given manga metadata id.
func (p *Anilist) SetMangaProgress(ctx context.Context, id, chapterNumber int) error {
	if id == 0 {
		return errors.New("set progress: anilist: invalid manga id 0")
	}
	if !p.Authenticated() {
		return fmt.Errorf("set progress of manga %d: anilist: %w", id, metadata.ErrUnauthorized)
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
		return fmt.Errorf("set progress of manga %d: %w", id, err)
	}

	return nil
}

// User returns the currently authenticated user.
//
// nil User means non-authenticated.
func (p *Anilist) User() metadata.User {
	return p.user
}
