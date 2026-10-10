package metadata

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// SearchMany searches several queries, using the cache and batching what's left when the
// provider is a Batcher. The results are in the order of the queries.
func (p *ProviderWithCache) SearchMany(queries []string) ([][]Metadata, error) {
	return p.SearchManyContext(context.Background(), queries)
}

// SearchManyContext is SearchMany with a context.
func (p *ProviderWithCache) SearchManyContext(ctx context.Context, queries []string) ([][]Metadata, error) {
	results := make([][]Metadata, len(queries))
	cachedIDs := map[int][]int{} // query index -> ids
	var allIDs []int
	var missing []string
	missingAt := map[string][]int{}
	for i, query := range queries {
		ids, found, err := p.store.getQueryIDs(query)
		if err != nil {
			return nil, p.cacheError(err)
		}
		if found {
			cachedIDs[i] = ids
			allIDs = append(allIDs, ids...)
			continue
		}
		if _, ok := missingAt[query]; !ok {
			missing = append(missing, query)
		}
		missingAt[query] = append(missingAt[query], i)
	}

	if len(allIDs) != 0 {
		metas, err := p.SearchByIDsContext(ctx, allIDs)
		if err != nil {
			return nil, err
		}
		for i, ids := range cachedIDs {
			for _, id := range ids {
				if meta, ok := metas[id]; ok {
					results[i] = append(results[i], meta)
				}
			}
		}
	}
	if len(missing) == 0 {
		return results, nil
	}

	p.logger.Log("searching manga metadata with %d queries on %q", len(missing), p.Info().Name)
	var searched [][]Metadata
	if batcher, ok := p.provider.(Batcher); ok {
		var err error
		if searched, err = batcher.SearchMany(ctx, missing); err != nil {
			return nil, err
		}
	} else {
		for _, query := range missing {
			metas, err := p.provider.Search(ctx, query)
			if err != nil {
				return nil, err
			}
			searched = append(searched, metas)
		}
	}

	for j, query := range missing {
		ids := make([]int, len(searched[j]))
		for k, meta := range searched[j] {
			id, err := strconv.Atoi(meta.ID().Raw)
			if err != nil {
				return nil, fmt.Errorf("%s: search %q: result %q: id %q isn't an integer", p.Info().ID, query, meta, meta.ID().Raw)
			}
			if err := p.store.setMeta(id, meta); err != nil {
				return nil, p.cacheError(err)
			}
			ids[k] = id
		}
		if err := p.store.setQueryIDs(query, ids); err != nil {
			return nil, p.cacheError(err)
		}
		for _, i := range missingAt[query] {
			results[i] = searched[j]
		}
	}
	return results, nil
}

// SearchByIDs gets the metadata of several ids, using the cache and batching what's left
// when the provider is a Batcher. Unknown ids are left out.
func (p *ProviderWithCache) SearchByIDs(ids []int) (map[int]Metadata, error) {
	return p.SearchByIDsContext(context.Background(), ids)
}

// SearchByIDsContext is SearchByIDs with a context.
func (p *ProviderWithCache) SearchByIDsContext(ctx context.Context, ids []int) (map[int]Metadata, error) {
	found := make(map[int]Metadata, len(ids))
	var missing []int
	for _, id := range ids {
		if _, ok := found[id]; ok {
			continue
		}
		meta, ok, err := p.store.getMeta(id)
		if err != nil {
			return nil, p.cacheError(err)
		}
		if ok {
			found[id] = meta
		} else if !slices.Contains(missing, id) {
			missing = append(missing, id)
		}
	}
	if len(missing) == 0 {
		return found, nil
	}

	fetched := map[int]Metadata{}
	if batcher, ok := p.provider.(Batcher); ok {
		var err error
		if fetched, err = batcher.SearchByIDs(ctx, missing); err != nil {
			return nil, err
		}
	} else {
		for _, id := range missing {
			meta, ok, err := p.provider.SearchByID(ctx, id)
			if err != nil {
				return nil, err
			}
			if ok {
				fetched[id] = meta
			}
		}
	}
	for id, meta := range fetched {
		if err := p.store.setMeta(id, meta); err != nil {
			return nil, p.cacheError(err)
		}
		found[id] = meta
	}
	return found, nil
}

// FindClosestMany is FindClosest for several queries, with as few requests as possible:
// the bound ids are looked up together, then each round searches the next title of every
// query without a good match yet.
func (p *ProviderWithCache) FindClosestMany(queries []Query) ([]Match, []bool, error) {
	return p.FindClosestManyContext(context.Background(), queries)
}

// FindClosestManyContext is FindClosestMany with a context.
func (p *ProviderWithCache) FindClosestManyContext(ctx context.Context, queries []Query) ([]Match, []bool, error) {
	matches := make([]Match, len(queries))
	found := make([]bool, len(queries))
	done := make([]bool, len(queries))
	titles := make([][]string, len(queries))

	bound := map[int]int{} // query index -> bound id
	var boundIDs []int
	for i, query := range queries {
		for _, t := range query.Titles {
			if strings.TrimSpace(t) != "" && len(titles[i]) < maxMatchSearches {
				titles[i] = append(titles[i], t)
			}
		}
		if len(titles[i]) == 0 {
			return nil, nil, fmt.Errorf("%s: closest match: no titles", p.Info().ID)
		}
		id, ok, err := p.store.getTitleID(query.Titles[0])
		if err != nil {
			return nil, nil, p.cacheError(err)
		}
		if ok {
			bound[i] = id
			boundIDs = append(boundIDs, id)
		}
	}

	if len(boundIDs) != 0 {
		metas, err := p.SearchByIDsContext(ctx, boundIDs)
		if err != nil {
			return nil, nil, err
		}
		for i, id := range bound {
			// the title may be bound by the user, so it wins even if its metadata expired
			if meta, ok := metas[id]; ok {
				matches[i], found[i], done[i] = Match{Metadata: meta, Score: Score(queries[i], meta), Bound: true}, true, true
				continue
			}
			p.logger.Log("id %d bound to title %q not found, searching by title", id, queries[i].Titles[0])
		}
	}

	for round := range maxMatchSearches {
		var search []string
		pending := map[int]string{}
		for i := range queries {
			if done[i] || round >= len(titles[i]) {
				continue
			}
			pending[i] = titles[i][round]
			search = append(search, titles[i][round])
		}
		if len(search) == 0 {
			break
		}

		results, err := p.SearchManyContext(ctx, search)
		if err != nil {
			return nil, nil, err
		}
		byTitle := make(map[string][]Metadata, len(search))
		for j, t := range search {
			byTitle[t] = results[j]
		}
		for i, t := range pending {
			if ranked := Rank(queries[i], byTitle[t]); len(ranked) != 0 && (!found[i] || ranked[0].Score > matches[i].Score) {
				matches[i], found[i] = ranked[0], true
			}
			done[i] = found[i] && !matches[i].Bad()
		}
	}

	for i, match := range matches {
		if !found[i] || match.Bound {
			continue
		}
		title := queries[i].Titles[0]
		p.logger.Log("closest to %q: %q with id %s (score %.2f)", title, match.Metadata, match.Metadata.ID().Raw, match.Score)
		if match.Bad() {
			continue
		}
		id, err := strconv.Atoi(match.Metadata.ID().Raw)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: closest to %q: id %q isn't an integer", p.Info().ID, title, match.Metadata.ID().Raw)
		}
		if err := p.store.setTitleID(title, id); err != nil {
			return nil, nil, p.cacheError(err)
		}
	}
	return matches, found, nil
}
