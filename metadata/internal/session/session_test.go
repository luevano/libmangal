package session

import (
	"errors"
	"testing"
)

func TestSession(t *testing.T) {
	var s Session[*string]
	if s.Logout() || s.Token() != "" {
		t.Fatal("zero value logged in")
	}
	name := "david"
	var seen string
	err := s.Login("token", func() (*string, error) {
		seen = s.Token()
		return &name, nil
	})
	if token, user := s.Get(); err != nil || token != "token" || user != &name || seen != "token" {
		t.Fatalf("login: %v, %q %v, user got %q", err, token, user, seen)
	}
	if err := s.Login("other", func() (*string, error) { return nil, errors.New("no") }); err == nil {
		t.Fatal("failed login")
	}
	if token, user := s.Get(); token != "" || user != nil {
		t.Errorf("failed login didn't log out: %q %v", token, user)
	}
	_ = s.Login("token", func() (*string, error) { return &name, nil })
	if !s.Logout() || s.Logout() || s.Token() != "" {
		t.Error("logout")
	}
}
