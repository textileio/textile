package local

import (
	"time"

	cid "github.com/ipfs/go-cid"
	"github.com/textileio/go-threads/core/thread"
)

type newOptions struct {
	name    string
	private bool
	thread  thread.ID
	key     string
	fromCid cid.Cid
	events  chan<- PathEvent
}

// NewOption is used when creating a new bucket.
type NewOption func(*newOptions)

// WithName sets a name for the bucket.
func WithName(name string) NewOption {
	return func(args *newOptions) {
		args.name = name
	}
}

// WithPrivate specifies that an encryption key will be used for the bucket.
func WithPrivate(private bool) NewOption {
	return func(args *newOptions) {
		args.private = private
	}
}

// WithCid indicates that an inited bucket should be boostraped
// with a particular UnixFS DAG.
func WithCid(c cid.Cid) NewOption {
	return func(args *newOptions) {
		args.fromCid = c
	}
}

// WithExistingPathEvents allows the caller to receive path events when pulling
// files from an existing bucket on initialization.
func WithExistingPathEvents(ch chan<- PathEvent) NewOption {
	return func(args *newOptions) {
		args.events = ch
	}
}

type pathOptions struct {
	confirm ConfirmDiffFunc
	force   bool
	hard    bool
	events  chan<- PathEvent
}

// PathOption is used when pushing or pulling bucket paths.
type PathOption func(*pathOptions)

// ConfirmDiffFunc is a caller-provided function which presents a list of bucket changes.
type ConfirmDiffFunc func([]Change) bool

// WithConfirm allows the caller to confirm a list of bucket changes.
func WithConfirm(f ConfirmDiffFunc) PathOption {
	return func(args *pathOptions) {
		args.confirm = f
	}
}

// WithForce indicates that all remote files should be pulled even if they already exist.
func WithForce(b bool) PathOption {
	return func(args *pathOptions) {
		args.force = b
	}
}

// WithHard indicates that locally staged changes should be pruned.
func WithHard(b bool) PathOption {
	return func(args *pathOptions) {
		args.hard = b
	}
}

// WithPathEvents allows the caller to receive path events when pushing or pulling files.
func WithPathEvents(ch chan<- PathEvent) PathOption {
	return func(args *pathOptions) {
		args.events = ch
	}
}

type addOptions struct {
	merge  SelectMergeFunc
	events chan<- PathEvent
}

// SelectMergeFunc is a caller-provided function which is used to select a merge strategy.
type SelectMergeFunc func(description string, isDir bool) (MergeStrategy, error)

// MergeStrategy describes the type of path merge strategy.
type MergeStrategy string

const (
	// Skip skips the path merge.
	Skip MergeStrategy = "Skip"
	// Merge attempts to merge the paths (directories only).
	Merge = "Merge"
	// Replace replaces the old path with the new path.
	Replace = "Replace"
)

// AddOption is used when staging a remote Unixfs dag cid in a local bucket.
type AddOption func(*addOptions)

// WithSelectMerge allows the caller to select the path merge strategy.
func WithSelectMerge(f SelectMergeFunc) AddOption {
	return func(args *addOptions) {
		args.merge = f
	}
}

// WithAddEvents allows the caller to receive path events when staging files from a remote Unixfs dag.
func WithAddEvents(ch chan<- PathEvent) AddOption {
	return func(args *addOptions) {
		args.events = ch
	}
}

type watchOptions struct {
	interval time.Duration
	events   chan<- PathEvent
}

// WatchOption is used when watching a bucket for changes.
type WatchOption func(*watchOptions)

// WatchInterval sets the interval at which local changes are synced remotely.
// In other words, this is the interval at which the local file watcher checks
// for filesystem changes.
func WithInterval(local time.Duration) WatchOption {
	return func(args *watchOptions) {
		args.interval = local
	}
}

// WithWatchEvents allows the caller to receive path events when watching a bucket for changes.
func WithWatchEvents(ch chan<- PathEvent) WatchOption {
	return func(args *watchOptions) {
		args.events = ch
	}
}
