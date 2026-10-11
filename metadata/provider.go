package metadata

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/luevano/libmangal/logger"
	"github.com/philippgille/gokv"
	"golang.org/x/mod/semver"
)

// ProviderInfo is the passport of the metadata provider.
type ProviderInfo struct {
	// ID is the unique identifier of the provider.
	//
	// For a ProviderWithCache this is used as the cache DB name.
	ID string `json:"id"`

	// Code is the unique code of the provider.
	//
	// E.g. IDCodeAnilist, IDCodeMyAnimeList, etc.
	Code IDCode `json:"code"`

	// Source is the source of the metadata.
	//
	// E.g. IDSourceAnilist, IDSourceMyAnimeList, etc.
	Source IDSource `json:"source"`

	// Name is the non-empty name of the provider.
	Name string `json:"name"`

	// Version is a semantic version of the provider.
	//
	// "v" prefix is not permitted.
	// E.g. "0.1.0" is valid, but "v0.1.0" is not.
	//
	// See https://semver.org/
	Version string `json:"version"`

	// Description of the provider. May be empty.
	Description string `json:"description"`

	// Website of the provider. May be empty.
	Website string `json:"website"`
}

// Validate checks if the ProviderInfo is valid.
//
// This means that ID, Code and Name are non-empty and
// Version is a valid semver.
func (p ProviderInfo) Validate() error {
	if p.ID == "" {
		return errors.New("empty id")
	}

	if p.Code == "" {
		return errors.New("empty code")
	}

	if p.Name == "" {
		return errors.New("empty name")
	}

	// according to the semver specification,
	// versions should not have "v" prefix. E.g. v0.1.0 isn't a valid semver,
	// however, for some bizarre reason, Go semver package requires this prefix.
	if !semver.IsValid("v" + p.Version) {
		return fmt.Errorf("invalid semver %q", p.Version)
	}

	return nil
}

// Provider exposes methods for searching mangas, getting chapters, pages and images.
type Provider interface {
	fmt.Stringer

	// Info information about Provider.
	Info() ProviderInfo

	// SetLogger sets logger to use for this provider.
	SetLogger(*logger.Logger)

	// Logger returns the set logger.
	//
	// Always returns a non-nil logger.
	Logger() *logger.Logger

	// SearchByID for metadata with the given id.
	//
	// Implementation should only handle the request and marshaling.
	SearchByID(ctx context.Context, id int) (Metadata, bool, error)

	// Search for metadata with the given query.
	//
	// Implementation should only handle the request and marshaling.
	Search(ctx context.Context, query string) ([]Metadata, error)

	// SetMangaProgress sets the reading progress for a given manga metadata id.
	SetMangaProgress(ctx context.Context, id, chapterNumber int) error

	// Authenticated returns true if the Provider is
	// currently authenticated (user logged in).
	Authenticated() bool

	// User returns the currently authenticated user.
	//
	// nil User means non-authenticated.
	User() User

	// Login authorizes an user with the given access token.
	Login(ctx context.Context, token string) error

	// Logout de-authorizes the currently authorized user.
	Logout() error
}

// ProviderWithCache wraps a Provider with a cache and extra search behavior.
//
// Its methods that make requests come in pairs: Foo uses context.Background(),
// FooContext takes a context.
type ProviderWithCache struct {
	provider Provider
	store    store
	logger   *logger.Logger
}

// NewProviderWithCache constructs new Provider with cache given the Provider.
func NewProviderWithCache(options ProviderWithCacheOptions) (*ProviderWithCache, error) {
	if options.Provider == nil {
		return nil, errors.New("invalid options: nil Provider")
	}
	if options.CacheStore == nil {
		return nil, errors.New("invalid options: nil CacheStore")
	}

	s, err := openStore(func(bucketName string) (gokv.Store, error) {
		return options.CacheStore(string(options.Provider.Info().ID), bucketName)
	})
	if err != nil {
		return nil, fmt.Errorf("%s: cache: %w", options.Provider.Info().ID, err)
	}

	// ensure the logger is non-nil
	l := options.Provider.Logger()
	if l == nil {
		l = logger.NewLogger()
		options.Provider.SetLogger(l)
	}

	p := &ProviderWithCache{
		provider: options.Provider,
		store:    s,
		logger:   l,
	}

	return p, nil
}

// Close closes the cache stores.
func (p *ProviderWithCache) Close() error {
	return p.store.close()
}

func (p *ProviderWithCache) cacheError(err error) error {
	return fmt.Errorf("%s: cache: %w", p.Info().ID, err)
}

func (p *ProviderWithCache) String() string {
	return p.provider.String()
}

// Info information about Provider.
func (p *ProviderWithCache) Info() ProviderInfo {
	return p.provider.Info()
}

// SetLogger sets logger to use for this provider.
func (p *ProviderWithCache) SetLogger(logger *logger.Logger) {
	p.logger = logger
	p.provider.SetLogger(p.logger)
}

// Logger returns the set logger.
//
// Always returns a non-nil logger.
func (p *ProviderWithCache) Logger() *logger.Logger {
	return p.logger
}

// SearchByID for metadata with the given id.
//
// Implementation should only handle the request and marshaling.
func (p *ProviderWithCache) SearchByID(id int) (Metadata, bool, error) {
	return p.SearchByIDContext(context.Background(), id)
}

// SearchByIDContext is SearchByID with a context.
func (p *ProviderWithCache) SearchByIDContext(ctx context.Context, id int) (Metadata, bool, error) {
	p.logger.Log("searching manga metadata with id %d on %q", id, p.Info().Name)
	meta, found, err := p.store.getMeta(id)
	if err != nil {
		return nil, false, p.cacheError(err)
	}
	if found {
		return meta, true, nil
	}

	meta, ok, err := p.provider.SearchByID(ctx, id)
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}

	if err := p.store.setMeta(id, meta); err != nil {
		return nil, false, p.cacheError(err)
	}

	return meta, true, nil
}

// Search for metadata with the given query.
//
// Implementation should only handle the request and marshaling.
func (p *ProviderWithCache) Search(query string) ([]Metadata, error) {
	return p.SearchContext(context.Background(), query)
}

// SearchContext is Search with a context.
func (p *ProviderWithCache) SearchContext(ctx context.Context, query string) ([]Metadata, error) {
	p.logger.Log("searching manga metadata with query %q on %q", query, p.Info().Name)
	ids, found, err := p.store.getQueryIDs(query)
	if err != nil {
		return nil, p.cacheError(err)
	}
	if found {
		var metas []Metadata
		for _, id := range ids {
			meta, ok, err := p.SearchByIDContext(ctx, id)
			if err != nil {
				return nil, err
			}
			if ok {
				metas = append(metas, meta)
			}
		}
		return metas, nil
	}

	metas, err := p.provider.Search(ctx, query)
	if err != nil {
		return nil, err
	}

	ids = make([]int, len(metas))
	for i, meta := range metas {
		id, err := strconv.Atoi(meta.ID().Raw)
		if err != nil {
			return nil, fmt.Errorf("%s: search %q: result %q: id %q isn't an integer", p.Info().ID, query, meta, meta.ID().Raw)
		}
		if err := p.store.setMeta(id, meta); err != nil {
			return nil, p.cacheError(err)
		}

		ids[i] = id
	}

	if err := p.store.setQueryIDs(query, ids); err != nil {
		return nil, p.cacheError(err)
	}

	return metas, nil
}

// maxMatchSearches is how many of the query's titles are searched at most,
// metadata APIs are rate limited (AniList: 30 requests a minute).
const maxMatchSearches = 3

// FindClosest finds the metadata that best matches the query.
//
// A title bound with BindTitleWithID (the query's first title) wins. Otherwise the
// query's titles are searched in order until a good match, and every result is
// scored (see Rank). found is false only when there were no results at all; the best
// match is returned even if it's bad (Match.Bad), for the user to judge, but only
// good matches are remembered for the title.
func (p *ProviderWithCache) FindClosest(query Query) (match Match, found bool, err error) {
	return p.FindClosestContext(context.Background(), query)
}

// FindClosestContext is FindClosest with a context.
func (p *ProviderWithCache) FindClosestContext(ctx context.Context, query Query) (match Match, found bool, err error) {
	p.logger.Log("finding closest manga metadata to %q on %q", query.Titles, p.Info().Name)
	matches, founds, err := p.FindClosestManyContext(ctx, []Query{query})
	if err != nil {
		return Match{}, false, err
	}
	return matches[0], founds[0], nil
}

// BindTitleWithID sets a given id to a title, so on each title search
// the same manga metadata with that id is obtained.
func (p *ProviderWithCache) BindTitleWithID(title string, id int) error {
	p.logger.Log("binding manga title %q to manga metadata id %d on %q", title, id, p.Info().Name)
	if err := p.store.setTitleID(title, id); err != nil {
		return p.cacheError(err)
	}
	return nil
}

// SetMangaProgress sets the reading progress for a given manga metadata id.
//
// For ProviderWithCache this is only a wrapper around the actual provider's method.
func (p *ProviderWithCache) SetMangaProgress(id, chapterNumber int) error {
	return p.SetMangaProgressContext(context.Background(), id, chapterNumber)
}

// SetMangaProgressContext is SetMangaProgress with a context.
func (p *ProviderWithCache) SetMangaProgressContext(ctx context.Context, id, chapterNumber int) error {
	p.logger.Log("setting manga chapter progress (%d) for manga id %d on %q", chapterNumber, id, p.Info().Name)
	return p.provider.SetMangaProgress(ctx, id, chapterNumber)
}

// Authenticated returns true if the Provider is
// currently authenticated (user logged in).
func (p *ProviderWithCache) Authenticated() bool {
	return p.provider.Authenticated()
}

// User returns the currently authenticated user.
//
// nil User means non-authenticated.
func (p *ProviderWithCache) User() User {
	return p.provider.User()
}

// Login authorizes an user with the given access token.
func (p *ProviderWithCache) Login(token string) error {
	return p.LoginContext(context.Background(), token)
}

// LoginContext is Login with a context.
func (p *ProviderWithCache) LoginContext(ctx context.Context, token string) error {
	p.logger.Log("logging in to %q", p.Info().Name)
	return p.provider.Login(ctx, token)
}

// Logout de-authorizes the currently authorized user.
func (p *ProviderWithCache) Logout() error {
	p.logger.Log("logging out of %q", p.Info().Name)
	return p.provider.Logout()
}

// Batcher is a Provider that can search several queries or ids at once,
// ProviderWithCache uses it when available.
type Batcher interface {
	// SearchMany searches several queries, the results are in the order of the queries.
	SearchMany(ctx context.Context, queries []string) ([][]Metadata, error)

	// SearchByIDs gets the metadata of several ids, unknown ids are left out.
	SearchByIDs(ctx context.Context, ids []int) (map[int]Metadata, error)
}
