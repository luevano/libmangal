package kitsu

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"
)

// searches and progress read the login while Login and Logout change it (mangal serve)
func TestLoginWhileSearching(t *testing.T) {
	k := newTestKitsu(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/edge/users":
			_, _ = w.Write([]byte(selfUser))
		case strings.HasPrefix(r.URL.Path, "/api/edge/library-entries"):
			_, _ = w.Write([]byte(`{"data": [{"id": "900", "type": "libraryEntries", "attributes": {"status": "current", "progress": 10}}]}`))
		default:
			_, _ = w.Write([]byte(`{"data": [` + soloLeveling + `], "included": ` + soloLevelingIncluded + `}`))
		}
	})
	ctx := context.Background()
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			for range 20 {
				if _, err := k.Search(ctx, "solo leveling"); err != nil {
					t.Error(err)
				}
				_ = k.User()
				// unauthorized when logged out, never a panic
				_ = k.SetMangaProgress(ctx, 7, 5)
			}
		})
	}
	for range 20 {
		if err := k.Login(ctx, "token"); err != nil {
			t.Error(err)
		}
		_ = k.Logout()
	}
	wg.Wait()
}
