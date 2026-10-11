package kitsu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/luevano/libmangal/metadata"
)

// libraryEntry is the user's list entry of a manga.
type libraryEntry struct {
	ID string
	// Status is one of current, planned, completed, on_hold, dropped.
	Status   string
	Progress int
}

type libraryEntryAttributes struct {
	Status   string `json:"status,omitempty"`
	Progress int    `json:"progress"`
}

type libraryEntryResource struct {
	ID            string                    `json:"id,omitempty"`
	Type          string                    `json:"type"`
	Attributes    libraryEntryAttributes    `json:"attributes"`
	Relationships map[string]map[string]ref `json:"relationships,omitempty"`
}

type libraryEntryBody struct {
	Data libraryEntryResource `json:"data"`
}

// SetMangaProgress sets the reading progress for a given manga metadata id.
func (p *Kitsu) SetMangaProgress(ctx context.Context, id, chapterNumber int) error {
	if id == 0 {
		return errors.New("set progress: kitsu: invalid manga id 0")
	}
	if !p.Authenticated() || p.user == nil {
		return fmt.Errorf("set progress of manga %d: kitsu: %w", id, metadata.ErrUnauthorized)
	}

	entry, err := p.libraryEntry(ctx, id)
	if err != nil {
		return fmt.Errorf("set progress of manga %d: get library entry: %w", id, err)
	}

	var body libraryEntryBody
	method, path := http.MethodPost, "library-entries"
	if entry != nil {
		// reading an older chapter (or rereading) shouldn't lower the progress
		if entry.Progress >= chapterNumber {
			p.logger.Log("progress of manga %d is already %d (%s), not setting %d", id, entry.Progress, entry.Status, chapterNumber)
			return nil
		}
		// the status is kept
		method, path = http.MethodPatch, "library-entries/"+entry.ID
		body.Data = libraryEntryResource{ID: entry.ID, Type: "libraryEntries", Attributes: libraryEntryAttributes{Progress: chapterNumber}}
	} else {
		body.Data = libraryEntryResource{
			Type:       "libraryEntries",
			Attributes: libraryEntryAttributes{Status: "current", Progress: chapterNumber},
			Relationships: map[string]map[string]ref{
				"user":  {"data": {ID: strconv.Itoa(p.user.IDProvider), Type: "users"}},
				"media": {"data": {ID: strconv.Itoa(id), Type: "manga"}},
			},
		}
	}

	if err := p.request(ctx, method, path, url.Values{}, body, nil); err != nil {
		return fmt.Errorf("set progress of manga %d: %w", id, err)
	}
	return nil
}

// libraryEntry is the authenticated user's entry of the manga, nil if not in the library.
func (p *Kitsu) libraryEntry(ctx context.Context, mangaID int) (*libraryEntry, error) {
	params := url.Values{}
	params.Set("filter[user_id]", strconv.Itoa(p.user.IDProvider))
	params.Set("filter[manga_id]", strconv.Itoa(mangaID))
	params.Set("fields[libraryEntries]", "status,progress")
	params.Set("page[limit]", "1")

	var doc document
	if err := p.request(ctx, http.MethodGet, "library-entries", params, nil, &doc); err != nil {
		return nil, err
	}
	entries, err := doc.resources()
	if err != nil {
		return nil, fmt.Errorf("kitsu: decode response: %w", err)
	}
	if len(entries) == 0 {
		return nil, nil
	}
	var attrs libraryEntryAttributes
	if err := json.Unmarshal(entries[0].Attributes, &attrs); err != nil {
		return nil, fmt.Errorf("kitsu: decode response: %w", err)
	}
	return &libraryEntry{ID: entries[0].ID, Status: attrs.Status, Progress: attrs.Progress}, nil
}
