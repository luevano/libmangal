package anilist

import (
	"context"
	"fmt"
	"strings"

	"github.com/luevano/libmangal/metadata"
	"github.com/luevano/libmangal/progress"
)

var _ metadata.Batcher = (*Anilist)(nil)

const (
	// searchesPerRequest keeps a batched search under the API's query complexity
	// limit (500): 10 searches are ~490, 20 are rejected.
	searchesPerRequest = 8
	// idsPerRequest is the page size of a lookup by ids.
	idsPerRequest = 25
)

// SearchMany searches several queries with as few requests as possible
// (searchesPerRequest per request), the results are in the order of the queries.
func (p *Anilist) SearchMany(ctx context.Context, queries []string) ([][]metadata.Metadata, error) {
	results := make([][]metadata.Metadata, 0, len(queries))
	requests := (len(queries) + searchesPerRequest - 1) / searchesPerRequest
	for start := 0; start < len(queries); start += searchesPerRequest {
		chunk := queries[start:min(start+searchesPerRequest, len(queries))]
		reportRequest(ctx, "searching AniList", start/searchesPerRequest+1, requests)

		var decl, fields strings.Builder
		vars := map[string]any{}
		for i, query := range chunk {
			name := fmt.Sprintf("q%d", i)
			vars[name] = query
			if i > 0 {
				decl.WriteString(", ")
			}
			fmt.Fprintf(&decl, "$%s: String", name)
			fmt.Fprintf(&fields, "\n\t%s: Page (page: 1, perPage: 30) {\n\t\tmedia (search: $%s, type: MANGA, format_not_in: [NOVEL]) {\n\t\t\t%s\n\t\t}\n\t}", name, name, queryCommon)
		}
		body := apiRequestBody{
			Query:     fmt.Sprintf("query (%s) {%s\n}", decl.String(), fields.String()),
			Variables: vars,
		}

		data, err := sendRequest[map[string]struct {
			Media medias `json:"media"`
		}](ctx, p, body)
		if err != nil {
			return nil, fmt.Errorf("search %q: %w", chunk, err)
		}
		for i := range chunk {
			results = append(results, data[fmt.Sprintf("q%d", i)].Media.GetAsMetas(p.options.TagRankMin))
		}
	}
	p.logger.Log("searched %d queries on Anilist", len(queries))
	return results, nil
}

// SearchByIDs gets the metadata of several ids with as few requests as possible,
// unknown ids are left out.
func (p *Anilist) SearchByIDs(ctx context.Context, ids []int) (map[int]metadata.Metadata, error) {
	found := make(map[int]metadata.Metadata, len(ids))
	requests := (len(ids) + idsPerRequest - 1) / idsPerRequest
	for start := 0; start < len(ids); start += idsPerRequest {
		chunk := ids[start:min(start+idsPerRequest, len(ids))]
		reportRequest(ctx, "looking up AniList metadata", start/idsPerRequest+1, requests)
		body := apiRequestBody{
			Query: `
query ($ids: [Int]) {
	Page (page: 1, perPage: ` + fmt.Sprint(idsPerRequest) + `) {
		media (id_in: $ids, type: MANGA) {
			` + queryCommon + `
		}
	}
}`,
			Variables: map[string]any{"ids": chunk},
		}
		data, err := sendRequest[mangasData](ctx, p, body)
		if err != nil {
			return nil, fmt.Errorf("get mangas %v: %w", chunk, err)
		}
		for _, manga := range data.Page.Media {
			if manga != nil {
				manga.TagRankMin = p.options.TagRankMin
				found[manga.IDProvider] = manga
			}
		}
	}
	return found, nil
}

// reportRequest reports the request of a batch, when it takes more than one.
func reportRequest(ctx context.Context, doing string, request, requests int) {
	if requests > 1 {
		progress.Report(ctx, progress.Status{Source: info.ID, Message: doing + ": request", Done: request, Total: requests})
	}
}
