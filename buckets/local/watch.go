package local

import (
	"context"
	"errors"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/radovskyb/watcher"
	"github.com/textileio/go-threads/api/client"
	"github.com/textileio/textile/buckets"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	fileSystemWatchInterval = time.Millisecond * 100
	reconnectInterval       = time.Second * 5
)

// WatchState is used to inform Watch callers about the connection state.
type WatchState struct {
	// State of the watch connection (online/offline).
	State ConnectionState
	// Error returned by the watch operation.
	Err error
	// Fatal indicates whether or not the associated error is fatal.
	// (Connectivity related errors are not fatal.)
	Fatal bool
}

// ConnectionState indicates an online/offline state.
type ConnectionState int

const (
	// Offline indicates the remote is currently not reachable.
	Offline ConnectionState = iota
	// Online indicates a connection with the remote has been established.
	Online
)

func (cs ConnectionState) String() string {
	switch cs {
	case Online:
		return "online"
	case Offline:
		return "offline"
	default:
		return "unknown state"
	}
}

// Watch watches for and auto-pushes local bucket changes at an interval,
// and listens for and auto-pulls remote changes as they arrive.
// Use the WithOffline option to keep watching while the local network is offline.
// Returns a channel of watch connectivity states.
// Cancel context to stop watching.
func (b *Bucket) Watch(ctx context.Context, opts ...WatchOption) (<-chan WatchState, error) {
	ctx, err := b.context(ctx)
	if err != nil {
		return nil, err
	}
	args := &watchOptions{}
	for _, opt := range opts {
		opt(args)
	}
	if !args.offline {
		return b.watchWhileConnected(ctx, args.events)
	}

	bc := backoff.NewConstantBackOff(reconnectInterval)
	outerState := make(chan WatchState)
	go func() {
		defer close(outerState)
		err := backoff.Retry(func() error {
			state, err := b.watchWhileConnected(ctx, args.events)
			if err != nil {
				outerState <- WatchState{Err: err, Fatal: true}
				return nil // Stop retrying
			}
			for s := range state {
				outerState <- s
				if s.Err != nil {
					if s.Fatal {
						return nil // Stop retrying
					} else {
						return s.Err // Connection error, keep trying
					}
				}
			}
			return nil
		}, bc)
		if err != nil {
			outerState <- WatchState{Err: err, Fatal: true}
		}
	}()
	return outerState, nil
}

// watchWhileConnected will watch until context is canceled or an error occurs.
func (b *Bucket) watchWhileConnected(ctx context.Context, pevents chan<- PathEvent) (<-chan WatchState, error) {
	id, err := b.Thread()
	if err != nil {
		return nil, err
	}
	bp, err := b.Path()
	if err != nil {
		return nil, err
	}

	state := make(chan WatchState)
	go func() {
		defer close(state)
		w := watcher.New()
		defer w.Close()
		w.SetMaxEvents(1)
		if err := w.AddRecursive(bp); err != nil {
			state <- WatchState{Err: err, Fatal: true}
			return
		}

		// Start listening for remote changes
		events, err := b.clients.Threads.Listen(ctx, id, []client.ListenOption{{
			Type:       client.ListenAll,
			InstanceID: b.Key(),
		}})
		if err != nil {
			state <- WatchState{Err: err, Fatal: !isConnectionErr(err)}
			return
		}
		errs := make(chan error)
		go func() {
			for e := range events {
				if e.Err != nil {
					errs <- e.Err // events will close on error
				} else if err := b.watchPull(ctx, pevents); err != nil {
					errs <- err
					return
				}
			}
		}()

		// Start listening for local changes
		go func() {
			if err := w.Start(fileSystemWatchInterval); err != nil {
				errs <- err
			}
		}()
		go func() {
			for {
				select {
				case <-w.Event:
					if err := b.watchPush(ctx, pevents); err != nil {
						errs <- err
					}
				case err := <-w.Error:
					errs <- err
				case <-w.Closed:
					return
				}
			}
		}()

		// If we made it here, we must be online
		state <- WatchState{State: Online}

		// Manually sync once on startup
		if err := b.watchPush(ctx, pevents); err != nil {
			state <- WatchState{Err: err, Fatal: !isConnectionErr(err)}
			return
		}

		for {
			select {
			case err := <-errs:
				state <- WatchState{Err: err, Fatal: !isConnectionErr(err)}
				return
			case <-ctx.Done():
				return
			}
		}
	}()
	return state, nil
}

func (b *Bucket) watchPush(ctx context.Context, events chan<- PathEvent) error {
	select {
	case b.pushBlock <- struct{}{}:
		defer func() {
			<-b.pushBlock
		}()
		if _, err := b.PushLocal(ctx, WithPathEvents(events)); errors.Is(err, ErrUpToDate) {
			return nil
		} else if errors.Is(err, buckets.ErrNonFastForward) {
			// Pull remote changes
			if _, err = b.PullRemote(ctx, WithPathEvents(events)); err != nil {
				return err
			}
			// Now try pushing again
			if _, err = b.PushLocal(ctx, WithPathEvents(events)); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		return nil
	}
}

func (b *Bucket) watchPull(ctx context.Context, events chan<- PathEvent) error {
	select {
	case b.pushBlock <- struct{}{}:
		if _, err := b.PullRemote(ctx, WithPathEvents(events)); !errors.Is(err, ErrUpToDate) {
			<-b.pushBlock
			return err
		}
		<-b.pushBlock
	default:
		// Ignore if there's a push in progress
	}
	return nil
}

func isConnectionErr(err error) bool {
	return status.Code(err) == codes.Unavailable
}
