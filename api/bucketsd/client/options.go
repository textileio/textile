package client

import (
	"github.com/ipfs/go-cid"
	"github.com/ipfs/interface-go-ipfs-core/path"
	pb "github.com/textileio/textile/v2/api/bucketsd/pb"
)

type createOptions struct {
	name     string
	private  bool
	fromCid  cid.Cid
	unfreeze bool
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

// WithUnfreeze indicates that an inited bucket should be boostraped
// from an imported archive.
func WithUnfreeze(enabled bool) CreateOption {
	return func(args *createOptions) {
		args.unfreeze = enabled
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

type ArchiveOption func(*pb.ArchiveRequest)

// WithArchiveConfig allows you to provide a custom ArchiveConfig for a single call to Archive.
func WithArchiveConfig(config *pb.ArchiveConfig) ArchiveOption {
	return func(req *pb.ArchiveRequest) {
		req.ArchiveConfig = config
	}
}

// WithSkipAutomaticVerifiedDeal allows to skip backend logic to automatically make
// a verified deal for the archive.
func WithSkipAutomaticVerifiedDeal(enabled bool) ArchiveOption {
	return func(req *pb.ArchiveRequest) {
		req.SkipAutomaticVerifiedDeal = enabled
	}
}
