package client

import (
	"github.com/textileio/go-threads/core/thread"
	mdb "github.com/textileio/textile/v2/mongodb"
)

type options struct {
	parentKey         thread.PubKey
	parentEmail       string
	parentAccountType mdb.AccountType
}

type Option func(*options)

// WithParent is used to create a billing hierarchy between two customers.
func WithParent(key thread.PubKey, email string, accountType mdb.AccountType) Option {
	return func(args *options) {
		args.parentKey = key
		args.parentEmail = email
		args.parentAccountType = accountType
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
