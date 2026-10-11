package libmangal

import (
	"context"

	"github.com/luevano/libmangal/metadata"
)

// Account is a service the user is logged into or subscribed to.
type Account = metadata.Account

// AccountProvider is a Provider with a user account (e.g. a subscription).
type AccountProvider interface {
	// Account gets the user's account, found false if the provider has none set up.
	Account(ctx context.Context) (account Account, found bool, err error)
}

// Account gets the account of the provider, found false if it has none.
func (c *Client) Account() (Account, bool, error) {
	return c.AccountContext(context.Background())
}

// AccountContext is Account with a context.
func (c *Client) AccountContext(ctx context.Context) (Account, bool, error) {
	provider, ok := c.provider.(AccountProvider)
	if !ok {
		return Account{}, false, nil
	}
	return provider.Account(ctx)
}
