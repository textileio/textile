package client

import (
	"github.com/textileio/go-threads/core/thread"
	mdb "github.com/textileio/textile/v2/mongodb"
)

type options struct {
	email       string
	accountType mdb.AccountType
	parentKey   thread.PubKey
}

type Option func(*options)

// WithEmail attaches an email address to the new customer.
func WithEmail(email string) Option {
	return func(args *options) {
		args.email = email
	}
}

// WithParentKey is used to create a billing hierarchy between two customers.
func WithParentKey(parentKey thread.PubKey) Option {
	return func(args *options) {
		args.parentKey = parentKey
	}
}

// WithAccountType attaches an account type to the new customer.
func WithAccountType(accountType mdb.AccountType) Option {
	return func(args *options) {
		args.accountType = accountType
	}
}

type listOptions struct {
	offset int64
	limit  int64
}

type ListOption func(*listOptions)

// WithOffset is used to fetch the next page when paginating.
func WithOffset(offset int64) ListOption {
	return func(args *listOptions) {
		args.offset = offset
	}
}

// WithLimit is used to set a page size when paginating.
func WithLimit(limit int64) ListOption {
	return func(args *listOptions) {
		args.limit = limit
	}
}
