package kitsu

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/luevano/libmangal/logger"
	"github.com/luevano/libmangal/metadata"
	"github.com/luevano/libmangal/metadata/internal/session"
)

// Reference docs:
// https://kitsu.docs.apiary.io/
// https://hummingbird-me.github.io/api-docs/

const (
	apiURL = "https://kitsu.app/api/edge"
	// pageLimit is the API's maximum page size.
	pageLimit = 20
)

var info = metadata.ProviderInfo{
	ID:      "kitsu",
	Code:    metadata.IDCodeKitsu,
	Source:  metadata.IDSourceKitsu,
	Name:    "Kitsu",
	Version: "0.1.0",
	Website: "https://kitsu.app/",
}

var _ metadata.Provider = (*Kitsu)(nil)

// Kitsu is a metadata.Provider implementation for Kitsu.
type Kitsu struct {
	session session.Session[*User]

	options Options
	logger  *logger.Logger
	// apiURL and tokenURL are only changed in tests
	apiURL   string
	tokenURL string
}

// NewKitsu constructs new Kitsu client.
func NewKitsu(options Options) (*Kitsu, error) {
	if options.HTTPClient == nil {
		return nil, errors.New("kitsu: invalid options: nil HTTPClient")
	}
	// ensure the used logger is not nil
	l := options.Logger
	if l == nil {
		l = logger.NewLogger()
	}
	return &Kitsu{
		options:  options,
		logger:   l,
		apiURL:   apiURL,
		tokenURL: OAuthTokenURL,
	}, nil
}

func (p *Kitsu) String() string {
	return info.Name
}

// Info information about Provider.
func (p *Kitsu) Info() metadata.ProviderInfo {
	return info
}

// SetLogger sets logger to use for this provider.
func (p *Kitsu) SetLogger(_logger *logger.Logger) {
	p.logger = _logger
}

// Logger returns the set logger.
//
// Always returns a non-nil logger.
func (p *Kitsu) Logger() *logger.Logger {
	return p.logger
}

// SearchByID for metadata with the given id.
//
// Implementation should only handle the request and marshaling.
func (p *Kitsu) SearchByID(ctx context.Context, id int) (metadata.Metadata, bool, error) {
	var doc document
	err := p.request(ctx, http.MethodGet, "manga/"+strconv.Itoa(id), mangaParams(), nil, &doc)
	if errors.Is(err, metadata.ErrNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("get manga %d: %w", id, err)
	}

	mangas, err := doc.mangas()
	if err != nil {
		return nil, false, fmt.Errorf("get manga %d: kitsu: decode response: %w", id, err)
	}
	if len(mangas) == 0 {
		return nil, false, nil
	}
	return mangas[0], true, nil
}

// Search for metadata with the given query.
//
// Implementation should only handle the request and marshaling.
func (p *Kitsu) Search(ctx context.Context, query string) ([]metadata.Metadata, error) {
	params := mangaParams()
	params.Set("filter[text]", query)
	params.Set("page[limit]", strconv.Itoa(pageLimit))

	var doc document
	if err := p.request(ctx, http.MethodGet, "manga", params, nil, &doc); err != nil {
		return nil, fmt.Errorf("search %q: %w", query, err)
	}
	mangas, err := doc.mangas()
	if err != nil {
		return nil, fmt.Errorf("search %q: kitsu: decode response: %w", query, err)
	}

	var metas []metadata.Metadata
	for _, m := range mangas {
		// novels share the manga search
		if m.Subtype == SubtypeNovel {
			continue
		}
		metas = append(metas, m)
	}
	p.logger.Log("found %d manga(s) on Kitsu", len(metas))
	return metas, nil
}

// User returns the currently authenticated user.
//
// nil User means non-authenticated.
func (p *Kitsu) User() metadata.User {
	_, user := p.session.Get()
	if user == nil {
		return nil
	}
	return user
}
