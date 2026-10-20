package libmangal

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/luevano/libmangal/mangadata"
)

// isNil is also true for a nil pointer in the interface, e.g. a provider's (*Manga)(nil).
func isNil(v any) bool {
	if v == nil {
		return true
	}
	switch rv := reflect.ValueOf(v); rv.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Map, reflect.Slice, reflect.Func, reflect.Chan:
		return rv.IsNil()
	}
	return false
}

// mangaProblem is why the manga isn't usable (it has no id, the key everything is stored
// under, or no title, its directory name), "" if it is.
func mangaProblem(manga mangadata.Manga) string {
	if isNil(manga) {
		return "nil manga"
	}
	info := manga.Info()
	switch {
	case strings.TrimSpace(info.ID) == "":
		return fmt.Sprintf("manga without an id (title %q, url %q)", info.Title, info.URL)
	case strings.TrimSpace(info.Title) == "":
		return fmt.Sprintf("manga %q without a title", info.ID)
	}
	return ""
}

// validItems are the items the provider returned without the unusable ones (nil, or a
// problem), each logged. If every item is unusable it's an error matching ErrInvalidResult:
// a reply the provider misread, not "found nothing".
func validItems[T any](c *Client, what string, items []T, problem func(T) string) ([]T, error) {
	var valid []T
	for i, item := range items {
		reason := "nil " + what
		if !isNil(item) {
			if reason = problem(item); reason == "" {
				valid = append(valid, item)
				continue
			}
		}
		c.logger.Log("provider %q: dropped %s %d of %d: %s", c.provider.Info().ID, what, i+1, len(items), reason)
	}
	if len(valid) == len(items) {
		return items, nil
	}
	if len(valid) == 0 {
		return nil, fmt.Errorf("provider %q: all %d %ss unusable: %w", c.provider.Info().ID, len(items), what, ErrInvalidResult)
	}
	return valid, nil
}

func noProblem[T any](T) string { return "" }
