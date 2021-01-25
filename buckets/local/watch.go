package local

import (
	"context"
	"errors"
	"time"

	"github.com/radovskyb/watcher"
	"github.com/textileio/go-threads/api/client"
	"github.com/textileio/textile/v2/buckets"
	"github.com/textileio/textile/v2/cmd"
)

const (
	fileSystemWatchInterval = time.Millisecond * 100
	reconnectInterval       = time.Second * 5
)

// Watch watches for and auto-pushes local bucket changes at an interval,
// and listens for and auto-pulls remote changes as they arrive.
// Use the WithOffline option to keep watching during network interruptions.
// Returns a channel of watch connectivity states.
// Cancel context to stop watching.
func (b *Bucket) Watch(ctx context.Context, opts ...WatchOption) (<-chan cmd.WatchState, error) {
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
	return cmd.Watch(ctx, func(ctx context.Context) (<-chan cmd.WatchState, error) {
		return b.watchWhileConnected(ctx, args.events)
	}, reconnectInterval)
}

// watchWhileConnected will watch until context is canceled or an error occurs.
func (b *Bucket) watchWhileConnected(ctx context.Context, pevents chan<- Event) (<-chan cmd.WatchState, error) {
	id, err := b.Thread()
	if err != nil {
		return nil, err
	}
	bp, err := b.Path()
	if err != nil {
		return nil, err
	}

	state := make(chan cmd.WatchState)
	go func() {
		defer close(state)
		w := watcher.New()
		defer w.Close()
		w.SetMaxEvents(1)
		if err := w.AddRecursive(bp); err != nil {
			state <- cmd.WatchState{Err: err, Aborted: true}
			return
		}

		// Start listening for remote changes
		events, err := b.clients.Threads.Listen(ctx, id, []client.ListenOption{{
			Type:       client.ListenAll,
			InstanceID: b.Key(),
		}})
		if err != nil {
			state <- cmd.WatchState{Err: err, Aborted: !cmd.IsConnectionError(err)}
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

		// Manually sync once on startup
		if err := b.watchPush(ctx, pevents); err != nil {
			state <- cmd.WatchState{Err: err, Aborted: !cmd.IsConnectionError(err)}
			return
		}

		// If we made it here, we must be online
		state <- cmd.WatchState{State: cmd.Online}

		for {
			select {
			case err := <-errs:
				state <- cmd.WatchState{Err: err, Aborted: !cmd.IsConnectionError(err)}
				return
			case <-ctx.Done():
				return
			}
		}
	}()
	return state, nil
}

func (b *Bucket) watchPush(ctx context.Context, events chan<- Event) error {
	b.pushBlock <- struct{}{}
	defer func() {
		<-b.pushBlock
	}()
	if _, err := b.PushLocal(ctx, WithEvents(events)); errors.Is(err, ErrUpToDate) {
		return nil
	} else if errors.Is(err, buckets.ErrNonFastForward) {
		// Pull remote changes
		if _, err = b.PullRemote(ctx, WithEvents(events)); err != nil {
			return err
		}
		// Now try pushing again
		if _, err = b.PushLocal(ctx, WithEvents(events)); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}

func (b *Bucket) watchPull(ctx context.Context, events chan<- Event) error {
	select {
	case b.pushBlock <- struct{}{}:
		if _, err := b.PullRemote(ctx, WithEvents(events)); !errors.Is(err, ErrUpToDate) {
			<-b.pushBlock
			return err
		}
		<-b.pushBlock
	default:
		// Ignore if there's a push in progress
	}
	return nil
}
