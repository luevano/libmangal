package kitsu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/luevano/libmangal/metadata"
)

var _ metadata.IDMapper = (*Kitsu)(nil)

// mappingSites are the externalSite of the mappings IDsFrom can look up.
var mappingSites = map[metadata.IDCode]string{
	metadata.IDCodeAnilist:     "anilist/manga",
	metadata.IDCodeMyAnimeList: "myanimelist/manga",
}

type mappingAttributes struct {
	ExternalSite string `json:"externalSite"`
	ExternalID   string `json:"externalId"`
}

// mappingsDocument is a document with the count of all the matches (data is a page).
type mappingsDocument struct {
	document
	Meta struct {
		Count int `json:"count"`
	} `json:"meta"`
}

// IDsFrom maps AniList or MyAnimeList ids to Kitsu ones with Kitsu's mappings (pageLimit
// ids per request), unknown ones are left out. When an id maps to several Kitsu mangas, the
// lowest Kitsu id is used.
func (p *Kitsu) IDsFrom(ctx context.Context, code metadata.IDCode, ids []int) (map[int]int, error) {
	site, ok := mappingSites[code]
	if !ok {
		return nil, fmt.Errorf("kitsu: map %s ids: %w", code, errors.ErrUnsupported)
	}
	found := make(map[int]int, len(ids))
	for start := 0; start < len(ids); start += pageLimit {
		chunk := ids[start:min(start+pageLimit, len(ids))]
		if err := p.mappings(ctx, site, chunk, found); err != nil {
			return nil, fmt.Errorf("map %s ids %v: %w", code, chunk, err)
		}
	}
	return found, nil
}

// mappings looks up the Kitsu mangas of the ids of site into found, following the pages
// (an id can have more than one mapping).
func (p *Kitsu) mappings(ctx context.Context, site string, ids []int, found map[int]int) error {
	wanted := make(map[string]int, len(ids))
	strIDs := make([]string, len(ids))
	for i, id := range ids {
		strIDs[i] = strconv.Itoa(id)
		wanted[strIDs[i]] = id
	}
	params := url.Values{
		"filter[externalSite]": {site},
		"filter[externalId]":   {strings.Join(strIDs, ",")},
		// the item's id only comes with the item included
		"include":          {"item"},
		"fields[mappings]": {"externalSite,externalId,item"},
		"fields[manga]":    {"subtype"},
		"page[limit]":      {strconv.Itoa(pageLimit)},
	}
	for offset := 0; ; {
		params.Set("page[offset]", strconv.Itoa(offset))
		var doc mappingsDocument
		if err := p.request(ctx, http.MethodGet, "mappings", params, nil, &doc); err != nil {
			return err
		}
		resources, err := doc.resources()
		if err != nil {
			return fmt.Errorf("kitsu: decode response: %w", err)
		}
		for _, r := range resources {
			var attrs mappingAttributes
			if err := json.Unmarshal(r.Attributes, &attrs); err != nil {
				return fmt.Errorf("kitsu: decode mapping %s: %w", r.ID, err)
			}
			external, ok := wanted[attrs.ExternalID]
			if !ok || attrs.ExternalSite != site {
				continue
			}
			for _, item := range r.Relationships["item"].refs() {
				id, err := strconv.Atoi(item.ID)
				if item.Type != "manga" || err != nil || id < 1 {
					continue
				}
				if old, ok := found[external]; !ok || id < old {
					found[external] = id
				}
			}
		}
		offset += len(resources)
		if len(resources) == 0 || offset >= doc.Meta.Count {
			return nil
		}
	}
}
