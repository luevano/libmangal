package anilist

import (
	"context"
	"errors"
	"fmt"

	"github.com/luevano/libmangal/metadata"
)

var _ metadata.IDMapper = (*Anilist)(nil)

// malIDsPerRequest is the page size of a lookup by MyAnimeList ids (AniList's maximum).
const malIDsPerRequest = 50

type malMappingData struct {
	Page struct {
		PageInfo struct {
			HasNextPage bool `json:"hasNextPage"`
		} `json:"pageInfo"`
		Media []*struct {
			ID    int `json:"id"`
			IDMal int `json:"idMal"`
		} `json:"media"`
	} `json:"page"`
}

// IDsFrom maps MyAnimeList ids to AniList ones (AniList's idMal, malIDsPerRequest ids per
// request), unknown ones are left out. When an id maps to several AniList mangas, the
// lowest AniList id is used. Other sources are an errors.ErrUnsupported.
func (p *Anilist) IDsFrom(ctx context.Context, code metadata.IDCode, ids []int) (map[int]int, error) {
	if code != metadata.IDCodeMyAnimeList {
		return nil, fmt.Errorf("anilist: map %s ids: %w", code, errors.ErrUnsupported)
	}
	found := make(map[int]int, len(ids))
	requests := (len(ids) + malIDsPerRequest - 1) / malIDsPerRequest
	for start := 0; start < len(ids); start += malIDsPerRequest {
		chunk := ids[start:min(start+malIDsPerRequest, len(ids))]
		reportRequest(ctx, "looking up AniList ids", start/malIDsPerRequest+1, requests)
		if err := p.malMappings(ctx, chunk, found); err != nil {
			return nil, fmt.Errorf("map %s ids %v: %w", code, chunk, err)
		}
	}
	return found, nil
}

// malMappings looks up the AniList mangas of the MyAnimeList ids into found, following
// the pages (an id can be on more than one manga).
func (p *Anilist) malMappings(ctx context.Context, ids []int, found map[int]int) error {
	wanted := make(map[int]bool, len(ids))
	for _, id := range ids {
		wanted[id] = true
	}
	for page := 1; ; page++ {
		body := apiRequestBody{
			Query: `
query ($ids: [Int], $page: Int) {
	Page (page: $page, perPage: ` + fmt.Sprint(malIDsPerRequest) + `) {
		pageInfo { hasNextPage }
		media (idMal_in: $ids, type: MANGA) { id idMal }
	}
}`,
			Variables: map[string]any{"ids": ids, "page": page},
		}
		data, err := sendRequest[malMappingData](ctx, p, body)
		if err != nil {
			return err
		}
		for _, m := range data.Page.Media {
			if m == nil || m.ID < 1 || !wanted[m.IDMal] {
				continue
			}
			if old, ok := found[m.IDMal]; !ok || m.ID < old {
				found[m.IDMal] = m.ID
			}
		}
		if !data.Page.PageInfo.HasNextPage || len(data.Page.Media) == 0 {
			return nil
		}
	}
}
