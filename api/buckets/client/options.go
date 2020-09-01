package client

import (
	"github.com/ipfs/go-cid"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/textileio/textile/buckets"
)

type createOptions struct {
	name    string
	private bool
	fromCid cid.Cid
}

type CreateOption func(*createOptions)

// WithName sets a name for the bucket.
func WithName(name string) CreateOption {
	return func(args *createOptions) {
		args.name = name
	}
}

// WithPrivate specifies that an encryption key will be used for the bucket.
func WithPrivate(private bool) CreateOption {
	return func(args *createOptions) {
		args.private = private
	}
}

// WithCid indicates that an inited bucket should be boostraped
// with a particular UnixFS DAG.
func WithCid(c cid.Cid) CreateOption {
	return func(args *createOptions) {
		args.fromCid = c
	}
}

type options struct {
	root     path.Resolved
	roles    map[string]buckets.Role
	progress chan<- int64
}

type Option func(*options)

// WithFastForwardOnly instructs the remote to reject non-fast-forward updates by comparing root with the remote.
func WithFastForwardOnly(root path.Resolved) Option {
	return func(args *options) {
		args.root = root
	}
}

// WithAccessRoles defines path access roles to be merged with the remote.
// roles is a map of string marshaled public keys to path roles.
// A non-nil error is returned if the map keys are not unmarshalable to public keys.
// To delete a role for a public key, set its value to buckets.None.
func WithAccessRoles(roles map[string]buckets.Role) Option {
	return func(args *options) {
		args.roles = roles
	}
}

// WithProgress writes progress updates to the given channel.
func WithProgress(ch chan<- int64) Option {
	return func(args *options) {
		args.progress = ch
	}
}
