package libmangal

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/luevano/libmangal/logger"
	"github.com/luevano/libmangal/mangadata"
	"golang.org/x/mod/semver"
)

// ProviderInfo is the passport of the provider.
type ProviderInfo struct {
	// ID is the unique identifier of the provider.
	ID string `json:"id"`

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

	// History of previous ids, names and websites, newest first. May be empty.
	//
	// Sites move or get renamed; the current ID, Name and Website are the ones to use,
	// every entry here is deprecated. An entry may keep the current ID when only the
	// name or website changed.
	History []ProviderAlias `json:"history,omitempty"`
}

// ProviderAlias is a previous identity of a provider.
type ProviderAlias struct {
	// ID the provider had. Non-empty.
	ID string `json:"id"`

	// Name the provider had. May be empty if only the ID changed.
	Name string `json:"name,omitempty"`

	// Website the provider had. May be empty.
	Website string `json:"website,omitempty"`
}

// Matches reports whether id is the provider's current ID or one in its History.
func (p ProviderInfo) Matches(id string) bool {
	if id == p.ID {
		return true
	}
	_, ok := p.Alias(id)
	return ok
}

// Alias returns the newest History entry with the given old id, false if the id
// is the current one or unknown.
func (p ProviderInfo) Alias(id string) (ProviderAlias, bool) {
	if id == p.ID {
		return ProviderAlias{}, false
	}
	for _, alias := range p.History {
		if alias.ID == id {
			return alias, true
		}
	}
	return ProviderAlias{}, false
}

// FindLoader finds the loader of the provider with the given id, current or old.
//
// Current ids win over old ones (a new provider may reuse an old id). If the id is an
// old one, the matching History entry is returned too, so callers can warn about it.
func FindLoader(loaders []ProviderLoader, id string) (ProviderLoader, *ProviderAlias, bool) {
	for _, loader := range loaders {
		if loader.Info().ID == id {
			return loader, nil, true
		}
	}
	for _, loader := range loaders {
		if alias, ok := loader.Info().Alias(id); ok {
			return loader, &alias, true
		}
	}
	return nil, nil, false
}

// Validate checks if the ProviderInfo is valid.
// This means that ID and Name are non-empty, Version is a valid semver
// and every History entry has an ID.
func (p ProviderInfo) Validate() error {
	if p.ID == "" {
		return errors.New("empty id")
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

	for i, alias := range p.History {
		if alias.ID == "" {
			return fmt.Errorf("history entry %d: empty id", i)
		}
	}

	return nil
}

// ProviderLoader gives information about provider without loading it first.
type ProviderLoader interface {
	fmt.Stringer

	// Info information about Provider.
	Info() ProviderInfo

	// Load loads the Provider.
	Load(ctx context.Context) (Provider, error)
}

// Provider exposes methods for searching mangas, getting chapters, pages and images.
type Provider interface {
	fmt.Stringer
	io.Closer

	// Info information about Provider.
	Info() ProviderInfo

	// SetLogger sets logger to use for this provider.
	SetLogger(*logger.Logger)

	// SearchMangas searches for mangas with the given query.
	//
	// Implementation should utilize given logger.
	SearchMangas(
		ctx context.Context,
		query string,
	) ([]mangadata.Manga, error)

	// MangaVolumes gets volumes of the manga.
	//
	// Implementation should utilize given logger.
	MangaVolumes(
		ctx context.Context,
		manga mangadata.Manga,
	) ([]mangadata.Volume, error)

	// VolumeChapters gets chapters of the given volume.
	//
	// Implementation should utilize given logger.
	VolumeChapters(
		ctx context.Context,
		volume mangadata.Volume,
	) ([]mangadata.Chapter, error)

	// ChapterPages gets pages of the given chapter.
	//
	// Implementation should utilize given logger
	ChapterPages(
		ctx context.Context,
		chapter mangadata.Chapter,
	) ([]mangadata.Page, error)

	// GetPageImage gets raw image contents of the given page.
	//
	// Implementation should utilize given logger.
	GetPageImage(
		ctx context.Context,
		page mangadata.Page,
	) ([]byte, error)
}
