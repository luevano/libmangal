package metadata

import (
	"errors"
	"strconv"

	"github.com/philippgille/gokv"
)

const (
	// QueryToIDs maps manga query to multiple metadata ids.
	//
	// ["berserk" => [7, 42, 69], "death note" => [887, 3, 134]]
	CacheBucketNameQueryToIDs = "query-to-ids"

	// TitleToID maps manga title to metadata id.
	//
	// ["berserk" => 7, "death note" => 3]
	CacheBucketNameTitleToID = "title-to-id"

	// IDToManga maps anilist id to metadata manga.
	//
	// [7 => "{title: ..., image: ..., ...}"]
	CacheBucketNameIDToManga = "id-to-manga"
)

type store struct {
	openStore func(bucketName string) (gokv.Store, error)
}

// get opens the bucket for a single operation, so concurrent calls don't share a handle
// and file-backed stores aren't held open between calls.
func (s store) get(bucketName, key string, v any) (found bool, err error) {
	st, err := s.openStore(bucketName)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, st.Close()) }()
	return st.Get(key, v)
}

func (s store) set(bucketName, key string, v any) (err error) {
	st, err := s.openStore(bucketName)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, st.Close()) }()
	return st.Set(key, v)
}

func (s store) getQueryIDs(query string) (ids []int, found bool, err error) {
	found, err = s.get(CacheBucketNameQueryToIDs, query, &ids)
	return
}

func (s store) setQueryIDs(query string, ids []int) error {
	return s.set(CacheBucketNameQueryToIDs, query, ids)
}

func (s store) getTitleID(title string) (id int, found bool, err error) {
	found, err = s.get(CacheBucketNameTitleToID, title, &id)
	return
}

func (s store) setTitleID(title string, id int) error {
	return s.set(CacheBucketNameTitleToID, title, id)
}

func (s store) getMeta(id int) (manga Metadata, found bool, err error) {
	found, err = s.get(CacheBucketNameIDToManga, strconv.Itoa(id), &manga)
	return
}

func (s store) setMeta(id int, manga Metadata) error {
	return s.set(CacheBucketNameIDToManga, strconv.Itoa(id), &manga)
}
