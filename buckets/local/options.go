package local

import (
	cid "github.com/ipfs/go-cid"
)

type newOptions struct {
	name     string
	private  bool
	fromCid  cid.Cid
	strategy InitStrategy
	events   chan<- Event
	unfreeze bool
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

// WithCid indicates an inited bucket should be boostraped with a particular UnixFS DAG.
func WithCid(c cid.Cid) NewOption {
	return func(args *newOptions) {
		args.fromCid = c
	}
}

// WithUnfreeze indicates that the bucket should be bootstrapped from Filecoin.
func WithUnfreeze(enabled bool) NewOption {
	return func(args *newOptions) {
		args.unfreeze = enabled
	}
}

// InitStrategy describes the type of init strategy.
type InitStrategy int

const (
	// Hybrid indicates locally staged changes should be accepted, excluding deletions.
	Hybrid InitStrategy = iota
	// Soft indicates locally staged changes should be accepted, including deletions.
	Soft
	// Hard indicates locally staged changes should be discarded.
	Hard
)

// WithStrategy allows for selecting the init strategy. Hybrid is the default.
func WithStrategy(strategy InitStrategy) NewOption {
	return func(args *newOptions) {
		args.strategy = strategy
	}
}

// WithInitEvents allows the caller to receive events when pulling
// files from an existing bucket on initialization.
func WithInitEvents(ch chan<- Event) NewOption {
	return func(args *newOptions) {
		args.events = ch
	}
}

type pathOptions struct {
	confirm ConfirmDiffFunc
	force   bool
	hard    bool
	events  chan<- Event
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

// WithForce indicates all remote files should be pulled even if they already exist.
func WithForce(b bool) PathOption {
	return func(args *pathOptions) {
		args.force = b
	}
}

// WithHard indicates locally staged changes should be discarded.
func WithHard(b bool) PathOption {
	return func(args *pathOptions) {
		args.hard = b
	}
}

// WithEvents allows the caller to receive events when pushing or pulling files.
func WithEvents(ch chan<- Event) PathOption {
	return func(args *pathOptions) {
		args.events = ch
	}
}

type addOptions struct {
	merge  SelectMergeFunc
	events chan<- Event
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

// WithAddEvents allows the caller to receive events when staging files from a remote Unixfs dag.
func WithAddEvents(ch chan<- Event) AddOption {
	return func(args *addOptions) {
		args.events = ch
	}
}

type watchOptions struct {
	offline bool
	events  chan<- Event
}

// WatchOption is used when watching a bucket for changes.
type WatchOption func(*watchOptions)

// WithOffline will keep watching for bucket changes while the local network is offline.
func WithOffline(offline bool) WatchOption {
	return func(args *watchOptions) {
		args.offline = offline
	}
}

// WithWatchEvents allows the caller to receive events when watching a bucket for changes.
func WithWatchEvents(ch chan<- Event) WatchOption {
	return func(args *watchOptions) {
		args.events = ch
	}
}
