package anilist

import (
	"context"
	"net/http"
	"sync"
	"testing"
)

// searches read the login while Login and Logout change it (mangal serve)
func TestLoginWhileSearching(t *testing.T) {
	a := newTestAnilist(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"viewer":{"id":1,"name":"david"},"page":{"media":[{"id":7,"title":{"english":"Berserk"}}]}}}`))
	})
	ctx := context.Background()
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			for range 20 {
				if _, err := a.Search(ctx, "berserk"); err != nil {
					t.Error(err)
				}
				_ = a.User()
			}
		})
	}
	for range 20 {
		if err := a.Login(ctx, "token"); err != nil {
			t.Error(err)
		}
		_ = a.Logout()
	}
	wg.Wait()
}
