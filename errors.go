package libmangal

import (
	"errors"

	"github.com/luevano/libmangal/netutil"
)

// Errors to check with errors.Is.
var (
	// ErrUnavailable: The chapter can't be downloaded, see mangadata.ChapterInfo.Unavailable.
	ErrUnavailable = errors.New("unavailable")

	// ErrMangaByIDUnsupported: The provider doesn't implement MangaByIDProvider.
	ErrMangaByIDUnsupported = errors.New("manga by id unsupported")

	// ErrNotFound: The site doesn't have it. Provider errors should match it,
	// metadata.ErrNotFound is the same value (see netutil).
	ErrNotFound = netutil.ErrNotFound

	// ErrRateLimited: The site is rate limiting. Provider errors should match it,
	// metadata.ErrRateLimited is the same value (see netutil, RetryAfter).
	ErrRateLimited = netutil.ErrRateLimited
)
