package client

import (
	"github.com/libp2p/go-libp2p-core/crypto"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-threads/core/thread"
)

type options struct {
	devToken string
	appKey   crypto.PrivKey
	appAddr  ma.Multiaddr
	thread   thread.ID
	progress chan<- int64
}

type Option func(*options)

func WithDevToken(token string) Option {
	return func(args *options) {
		args.devToken = token
	}
}

func WithAppCredentials(addr ma.Multiaddr, key crypto.PrivKey) Option {
	return func(args *options) {
		args.appAddr = addr
		args.appKey = key
	}
}

func WithThread(id thread.ID) Option {
	return func(args *options) {
		args.thread = id
	}
}

// WithProgress writes progress updates to the given channel.
func WithProgress(ch chan<- int64) Option {
	return func(args *options) {
		args.progress = ch
	}
}
