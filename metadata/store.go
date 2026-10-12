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

	// ExternalToID maps another source's id (code:id) to the provider's id, for IDMapper.
	//
	// ["al:30002" => 8, "mal:2" => 8]
	CacheBucketNameExternalToID = "external-to-id"
)

// store holds the cache buckets, opened once with the provider and closed with it.
type store struct {
	buckets map[string]gokv.Store
}

func openStore(open func(bucketName string) (gokv.Store, error)) (store, error) {
	s := store{buckets: map[string]gokv.Store{}}
	for _, name := range []string{CacheBucketNameQueryToIDs, CacheBucketNameTitleToID, CacheBucketNameIDToManga, CacheBucketNameExternalToID} {
		st, err := open(name)
		if err != nil {
			return store{}, errors.Join(fmt.Errorf("open %s: %w", name, err), s.close())
		}
		s.buckets[name] = st
	}
	return s, nil
}

func (s store) close() error {
	var errs []error
	for _, st := range s.buckets {
		errs = append(errs, st.Close())
	}
	return errors.Join(errs...)
}

func (s store) get(bucketName, key string, v any) (bool, error) {
	found, err := s.buckets[bucketName].Get(key, v)
	if err != nil {
		return false, fmt.Errorf("get %s %q: %w", bucketName, key, err)
	}
	return found, nil
}

func (s store) set(bucketName, key string, v any) error {
	if err := s.buckets[bucketName].Set(key, v); err != nil {
		return fmt.Errorf("set %s %q: %w", bucketName, key, err)
	}
	return nil
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

func externalKey(code IDCode, id int) string {
	return string(code) + ":" + strconv.Itoa(id)
}

func (s store) getExternalID(code IDCode, external int) (id int, found bool, err error) {
	found, err = s.get(CacheBucketNameExternalToID, externalKey(code, external), &id)
	return
}

func (s store) setExternalID(code IDCode, external, id int) error {
	return s.set(CacheBucketNameExternalToID, externalKey(code, external), id)
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
