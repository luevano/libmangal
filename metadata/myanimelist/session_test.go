package myanimelist

import (
	"context"
	"net/http"
	"sync"
	"testing"
)

// searches read the login while Login and Logout change it (mangal serve)
func TestLoginWhileSearching(t *testing.T) {
	m := newTestMAL(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/users/@me" {
			_, _ = w.Write([]byte(`{"id":1,"name":"david"}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"node":{"id":2,"title":"Berserk"}}]}`))
	})
	ctx := context.Background()
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			for range 20 {
				if _, err := m.Search(ctx, "berserk"); err != nil {
					t.Error(err)
				}
				_ = m.User()
			}
		})
	}
	for range 20 {
		if err := m.Login(ctx, "token"); err != nil {
			t.Error(err)
		}
		_ = m.Logout()
	}
	wg.Wait()
}
