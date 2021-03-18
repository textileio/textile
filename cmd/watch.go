package cmd

import (
	"context"
	"strings"
	"time"

	backoff "github.com/cenkalti/backoff/v4"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// WatchState is used to inform Watch callers about the connection state.
type WatchState struct {
	// State of the watch connection (online/offline).
	State ConnectionState
	// Err returned by the watch operation.
	Err error
	// Aborted indicates whether or not the associated error aborted the watch.
	// (Connectivity related errors do not abort the watch.)
	Aborted bool
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

// WatchFunc is a function wrapper for a function used by Watch.
type WatchFunc func(context.Context) (<-chan WatchState, error)

// Watch calls watchFunc until it returns an error.
// Normally, watchFunc would block while doing work that can fail,
// e.g., the local network goes offline.
// If watchFunc return an error, it will be called
// again at the given interval so long as the returned error is non-fatal.
// Returns a channel of watch connectivity states.
// Cancel context to stop watching.
func Watch(ctx context.Context, watchFunc WatchFunc, reconnectInterval time.Duration) (<-chan WatchState, error) {
	bc := backoff.NewConstantBackOff(reconnectInterval)
	outerState := make(chan WatchState)
	go func() {
		defer close(outerState)
		err := backoff.Retry(func() error {
			state, err := watchFunc(ctx)
			if err != nil {
				outerState <- WatchState{Err: err, Aborted: true}
				return nil // Stop retrying
			}
			for s := range state {
				outerState <- s
				if s.Err != nil {
					if s.Aborted {
						return nil // Stop retrying
					} else {
						return s.Err // Connection error, keep trying
					}
				}
			}
			return nil
		}, bc)
		if err != nil {
			outerState <- WatchState{Err: err, Aborted: true}
		}
	}()
	return outerState, nil
}

// IsConnectionError returns true if the error is related to a dropped connection.
func IsConnectionError(err error) bool {
	return status.Code(err) == codes.Unavailable || strings.Contains(err.Error(), "RST_STREAM")
}
