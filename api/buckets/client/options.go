package client

import (
	"github.com/ipfs/go-cid"
	"github.com/ipfs/interface-go-ipfs-core/path"
)

type pushOptions struct {
	root     path.Resolved
	progress chan<- int64
}

type PushOption func(*pushOptions)

// WithFastForwardOnly instructs the remote to reject non-fast-forward updates by comparing root with the remote.
func WithFastForwardOnly(root path.Resolved) PushOption {
	return func(args *pushOptions) {
		args.root = root
	}
}

// WithProgress writes progress updates to the given channel.
func WithProgress(ch chan<- int64) PushOption {
	return func(args *pushOptions) {
		args.progress = ch
	}
}

type initOptions struct {
	bootstrapCid cid.Cid
}

type InitOption func(*initOptions)

// WithCid indicates that a inited bucket should be boostraped
// with a particular UnixFS DAG.
func WithCid(c cid.Cid) InitOption {
	return func(args *initOptions) {
		args.bootstrapCid = c
	}
}
