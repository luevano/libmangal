package kitsu

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/luevano/libmangal/metadata"
)

var _ metadata.Batcher = (*Kitsu)(nil)

// SearchMany searches several queries, the results are in the order of the queries.
//
// Kitsu has no batched search, it's one request per query.
func (p *Kitsu) SearchMany(ctx context.Context, queries []string) ([][]metadata.Metadata, error) {
	results := make([][]metadata.Metadata, len(queries))
	for i, query := range queries {
		metas, err := p.Search(ctx, query)
		if err != nil {
			return nil, err
		}
		results[i] = metas
	}
	return results, nil
}

// SearchByIDs gets the metadata of several ids with as few requests as possible
// (pageLimit ids per request), unknown ids are left out.
func (p *Kitsu) SearchByIDs(ctx context.Context, ids []int) (map[int]metadata.Metadata, error) {
	found := make(map[int]metadata.Metadata, len(ids))
	for start := 0; start < len(ids); start += pageLimit {
		chunk := ids[start:min(start+pageLimit, len(ids))]
		strIDs := make([]string, len(chunk))
		for i, id := range chunk {
			strIDs[i] = strconv.Itoa(id)
		}

		params := mangaParams()
		params.Set("filter[id]", strings.Join(strIDs, ","))
		params.Set("page[limit]", strconv.Itoa(pageLimit))
		var doc document
		if err := p.request(ctx, http.MethodGet, "manga", params, nil, &doc); err != nil {
			return nil, fmt.Errorf("get mangas %v: %w", chunk, err)
		}
		mangas, err := doc.mangas()
		if err != nil {
			return nil, fmt.Errorf("get mangas %v: kitsu: decode response: %w", chunk, err)
		}
		for _, m := range mangas {
			found[m.IDProvider] = m
		}
	}
	return found, nil
}
