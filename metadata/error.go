package metadata

import (
	"errors"

	"github.com/luevano/libmangal/netutil"
)

// Errors to check with errors.Is, the metadata providers' errors match them when it applies.
//
// ErrNotFound and ErrRateLimited are netutil's (and libmangal's) sentinels.
var (
	ErrNotFound     = netutil.ErrNotFound
	ErrRateLimited  = netutil.ErrRateLimited
	ErrUnauthorized = errors.New("unauthorized")
)
