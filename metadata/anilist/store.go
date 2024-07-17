package anilist

import (
	"fmt"
	"strconv"

	"github.com/philippgille/gokv"
)

const (
	// QueryToIDs maps manga query to multiple anilit ids.
	//
	// ["berserk" => [7, 42, 69], "death note" => [887, 3, 134]]
	CacheBucketNameQueryToIDs = "query-to-id"

	// TitleToID maps manga title to anilist id.
	//
	// ["berserk" => 7, "death note" => 3]
	CacheBucketNameTitleToID = "title-to-id"

	// IDToManga maps anilist id to anilist manga.
	//
	// [7 => "{title: ..., image: ..., ...}"]
	CacheBucketNameIDToManga = "id-to-manga"

	// AccessTokenStore holds the authentication data.
	// Currently only supports one auth token.
	CacheBucketNameAccessToken = "access-token"
)

type store struct {
	openStore func(bucketName string) (gokv.Store, error)
	store     gokv.Store
}

func (s *store) open(bucketName string) error {
	store, err := s.openStore(bucketName)
	if store == nil {
		fmt.Println("store is nil when opening bucket " + bucketName)
	}
	s.store = store
	if s.store == nil {
		fmt.Println("assigned store is nil when opening bucket " + bucketName)
	}
	return err
}

func (s *store) Close() error {
	if s.store == nil {
		return nil
	}
	return s.store.Close()
}

func (s *store) getQueryIDs(query string) (ids []int, found bool, err error) {
	err = s.open(CacheBucketNameQueryToIDs)
	if err != nil {
		return
	}
	defer s.Close()

	found, err = s.store.Get(query, &ids)
	return
}

func (s *store) setQueryIDs(query string, ids []int) (err error) {
	err = s.open(CacheBucketNameQueryToIDs)
	if err != nil {
		return
	}
	defer s.Close()

	return s.store.Set(query, ids)
}

func (s *store) getTitleID(title string) (id int, found bool, err error) {
	err = s.open(CacheBucketNameTitleToID)
	if err != nil {
		return
	}
	defer s.Close()

	found, err = s.store.Get(title, &id)
	return
}

func (s *store) setTitleID(title string, id int) (err error) {
	err = s.open(CacheBucketNameTitleToID)
	if err != nil {
		return
	}
	defer s.Close()

	return s.store.Set(title, id)
}

func (s *store) getManga(id int) (manga Manga, found bool, err error) {
	err = s.open(CacheBucketNameIDToManga)
	if err != nil {
		return
	}
	defer s.Close()

	found, err = s.store.Get(strconv.Itoa(id), &manga)
	return
}

func (s *store) setManga(id int, manga Manga) (err error) {
	err = s.open(CacheBucketNameIDToManga)
	if err != nil {
		return
	}
	defer s.Close()

	return s.store.Set(strconv.Itoa(id), manga)
}

func (s *store) getAuthToken(key string) (token string, found bool, err error) {
	err = s.open(CacheBucketNameAccessToken)
	if err != nil {
		return
	}
	defer s.Close()

	if s.store == nil {
		fmt.Println("store is nil when getting auth token")
	}

	found, err = s.store.Get(key, &token)
	return
}

func (s *store) setAuthToken(key, authToken string) (err error) {
	err = s.open(CacheBucketNameAccessToken)
	if err != nil {
		return
	}
	defer s.Close()

	return s.store.Set(key, authToken)
}

func (s *store) deleteAuthToken(key string) (err error) {
	err = s.open(CacheBucketNameAccessToken)
	if err != nil {
		return
	}
	defer s.Close()

	return s.store.Delete(key)
}