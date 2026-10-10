package metadata

import (
	"sync"

	"github.com/philippgille/gokv"
)

type ProviderWithCacheOptions struct {
	// Provider is the underlying provider to which the cache is implemented on.
	//
	// Must be non-nil.
	Provider Provider

	// CacheStore returns a gokv.Store implementation for use as a cache storage.
	//
	// It will use the given provider's ID as the dbName.
	CacheStore func(dbName, bucketName string) (gokv.Store, error)
}

// DefaultProviderWithCacheOptions constructs the default ProviderWithCacheOptions.
//
// Note: the Provider must be added afterwards, this (for now) only builds a default CacheStore.
//
// The default CacheStore keeps everything in memory, for as long as the process runs.
func DefaultProviderWithCacheOptions() ProviderWithCacheOptions {
	var mu sync.Mutex
	stores := map[string]memStore{}
	return ProviderWithCacheOptions{
		CacheStore: func(dbName, bucketName string) (gokv.Store, error) {
			mu.Lock()
			defer mu.Unlock()
			key := dbName + "/" + bucketName
			if _, ok := stores[key]; !ok {
				stores[key] = newMemStore()
			}
			return stores[key], nil
		},
	}
}

// ComicInfoXMLOptions tweaks ComicInfoXML generation.
type ComicInfoXMLOptions struct {
	// AddDate whether to add series release date or not.
	AddDate bool

	// AlternativeDate use other date.
	AlternativeDate *Date
}

// DefaultComicInfoOptions constructs default ComicInfoXMLOptions.
func DefaultComicInfoOptions() ComicInfoXMLOptions {
	return ComicInfoXMLOptions{
		AddDate: true,
	}
}
