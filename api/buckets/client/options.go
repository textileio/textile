package client

import (
	"github.com/ipfs/go-cid"
	"github.com/ipfs/interface-go-ipfs-core/path"
)

type initOptions struct {
	name    string
	private bool
	fromCid cid.Cid
}

type InitOption func(*initOptions)

// WithName sets a name for the bucket.
func WithName(name string) InitOption {
	return func(args *initOptions) {
		args.name = name
	}
}

// WithPrivate specifies that an encryption key will be used for the bucket.
func WithPrivate(private bool) InitOption {
	return func(args *initOptions) {
		args.private = private
	}
}

// WithCid indicates that an inited bucket should be boostraped
// with a particular UnixFS DAG.
func WithCid(c cid.Cid) InitOption {
	return func(args *initOptions) {
		args.fromCid = c
	}
}

type options struct {
	root     path.Resolved
	progress chan<- int64
}

type Option func(*options)

// WithFastForwardOnly instructs the remote to reject non-fast-forward updates by comparing root with the remote.
func WithFastForwardOnly(root path.Resolved) Option {
	return func(args *options) {
		args.root = root
	}
}

// WithProgress writes progress updates to the given channel.
func WithProgress(ch chan<- int64) Option {
	return func(args *options) {
		args.progress = ch
	}
}
