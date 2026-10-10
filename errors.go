package libmangal

import "errors"

// Errors to check with errors.Is.
var (
	// ErrUnavailable: The chapter can't be downloaded, see mangadata.ChapterInfo.Unavailable.
	ErrUnavailable = errors.New("unavailable")
)
