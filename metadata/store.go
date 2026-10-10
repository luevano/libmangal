package metadata

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"sync"

	"github.com/philippgille/gokv"
	"github.com/philippgille/gokv/util"
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
	defer func() {
		if err != nil {
			err = fmt.Errorf("get %s %q: %w", bucketName, key, err)
		}
	}()
	st, err := s.openStore(bucketName)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, st.Close()) }()
	return st.Get(key, v)
}

func (s store) set(bucketName, key string, v any) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("set %s %q: %w", bucketName, key, err)
		}
	}()
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

// memStore is an in-memory gokv.Store that keeps the values themselves instead of
// encoding them, so interface values (Metadata) come back as their concrete types.
type memStore struct {
	mu   *sync.Mutex
	data map[string]any
}

func newMemStore() memStore {
	return memStore{mu: &sync.Mutex{}, data: map[string]any{}}
}

func (s memStore) Set(k string, v any) error {
	if err := util.CheckKeyAndValue(k, v); err != nil {
		return err
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[k] = rv.Interface()
	return nil
}

func (s memStore) Get(k string, v any) (bool, error) {
	if err := util.CheckKeyAndValue(k, v); err != nil {
		return false, err
	}
	s.mu.Lock()
	stored, ok := s.data[k]
	s.mu.Unlock()
	if !ok {
		return false, nil
	}
	dst := reflect.ValueOf(v).Elem()
	src := reflect.ValueOf(stored)
	if !src.Type().AssignableTo(dst.Type()) {
		return false, fmt.Errorf("cached %q is a %s, can't load it into a %s", k, src.Type(), dst.Type())
	}
	dst.Set(src)
	return true, nil
}

func (s memStore) Delete(k string) error {
	if err := util.CheckKey(k); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, k)
	return nil
}

func (s memStore) Close() error { return nil }
