// Package session is the login of a metadata provider: searches read it while Login and
// Logout change it from another goroutine (a long-running mangal serve).
package session

import (
	"sync"
	"sync/atomic"
)

// Session is a token and its user, the zero value is logged out.
type Session[U any] struct {
	// mu serializes the changes, so a Logout during a Login isn't undone by it
	mu  sync.Mutex
	cur atomic.Pointer[state[U]]
}

type state[U any] struct {
	token string
	user  U
}

// Get is the token and user, both at once. An empty token is logged out.
func (s *Session[U]) Get() (token string, user U) {
	if c := s.cur.Load(); c != nil {
		return c.token, c.user
	}
	return "", user
}

// Token is the token, empty when logged out.
func (s *Session[U]) Token() string {
	token, _ := s.Get()
	return token
}

// Login sets token, then user gets its user with it (the requests see the token). When
// user fails it's logged out.
func (s *Session[U]) Login(token string, user func() (U, error)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cur.Store(&state[U]{token: token})
	u, err := user()
	if err != nil {
		s.cur.Store(nil)
		return err
	}
	s.cur.Store(&state[U]{token: token, user: u})
	return nil
}

// Logout logs out, false if it wasn't logged in.
func (s *Session[U]) Logout() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.cur.Swap(nil)
	return c != nil && c.token != ""
}
