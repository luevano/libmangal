package metadata

import "errors"

// Errors to check with errors.Is, the metadata providers' errors match them when it applies.
var (
	ErrNotFound     = errors.New("not found")
	ErrRateLimited  = errors.New("rate limited")
	ErrUnauthorized = errors.New("unauthorized")
)
