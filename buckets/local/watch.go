package local

import (
	"context"
	"errors"
	"time"

	"github.com/radovskyb/watcher"
	"github.com/textileio/go-threads/api/client"
	"github.com/textileio/textile/buckets"
)

const defaultInterval = time.Millisecond * 100

// Watch watches for and auto-pushes local bucket changes at an interval,
// and listens for and auto-pulls remote changes as they arrive
// This will watch until context is canceled.
func (b *Bucket) Watch(ctx context.Context, opts ...WatchOption) error {
	args := &watchOptions{
		interval: defaultInterval,
	}
	for _, opt := range opts {
		opt(args)
	}
	errs := make(chan error)

	// Manually sync once on startup
	if err := b.watchPush(ctx, args.events); err != nil {
		return err
	}

	// Start listening for local changes
	w := watcher.New()
	defer w.Close()
	w.SetMaxEvents(1)
	bp, err := b.Path()
	if err != nil {
		return err
	}
	if err := w.AddRecursive(bp); err != nil {
		return err
	}
	go func() {
		if err := w.Start(args.interval); err != nil {
			errs <- err
		}
	}()
	go func() {
		for {
			select {
			case <-w.Event:
				if err := b.watchPush(ctx, args.events); err != nil {
					errs <- err
				}
			case err := <-w.Error:
				errs <- err
			case <-w.Closed:
				return
			}
		}
	}()

	// Start listening for remote changes
	id, err := b.Thread()
	if err != nil {
		return err
	}
	events, err := b.clients.Threads.Listen(ctx, id, []client.ListenOption{{
		Type:       client.ListenAll,
		InstanceID: b.Key(),
	}})
	if err != nil {
		return err
	}
	go func() {
		for e := range events {
			if e.Err != nil {
				errs <- err // events will close on error
			} else if err := b.watchPull(ctx, args.events); err != nil {
				errs <- err
				return
			}
		}
	}()

	for {
		select {
		case err := <-errs:
			return err
		case <-ctx.Done():
			return nil
		}
	}
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
