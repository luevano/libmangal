package metadata

import "time"

// Account is a service the user is logged into or subscribed to
// (libmangal.Account is this same type).
type Account struct {
	// Service is the name of the site or app, e.g. "MangaPlus".
	Service string `json:"service"`

	// User is the user's display name, may be empty.
	User string `json:"user,omitempty"`

	// URL of the user's profile or account page, may be empty.
	URL string `json:"url,omitempty"`

	// Status is a short state, e.g. "logged in", "deluxe", "free device".
	Status string `json:"status"`

	// Expires is when the login or subscription ends (or is next paid), zero if unknown.
	Expires time.Time `json:"expires,omitzero"`

	// Details are extra notes, e.g. "free trial".
	Details []string `json:"details,omitempty"`
}

// AccountOf is the login of the metadata provider (a Provider or a ProviderWithCache):
// the user if authenticated.
func AccountOf(p interface {
	Info() ProviderInfo
	Authenticated() bool
	User() User
}) Account {
	account := Account{Service: p.Info().Name, URL: p.Info().Website, Status: "not logged in"}
	if !p.Authenticated() || p.User() == nil {
		return account
	}
	user := p.User()
	account.User, account.URL, account.Status = user.Name(), user.URL(), "logged in"
	return account
}
