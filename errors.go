package libmangal

import (
	"errors"
	"fmt"

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

// OpenError: Client.ReadChapter couldn't open the file with the default app, so no
// reading progress was set.
type OpenError struct {
	Path string
	Err  error
}

func (e *OpenError) Error() string {
	return fmt.Sprintf("open %s: %v", e.Path, e.Err)
}

func (e *OpenError) Unwrap() error {
	return e.Err
}
