package indexer

import (
	"time"
)

var (
	defaultConfig = config{
		daemonRunOnStart: false,
		daemonFrequency:  60 * time.Minute,
	}
)

type config struct {
	daemonRunOnStart     bool
	daemonFrequency      time.Duration
	daemonSnapshotMaxAge time.Duration
}

// Option allows to configure an indexer.
type Option func(*config)

// WithRunOnStart indicates if the index must be generated
// when the deamon is started.
func WithRunOnStart(enabled bool) Option {
	return func(c *config) {
		c.daemonRunOnStart = enabled
	}
}

// WithFrequency indicates the frequency
// for the indexer daemon.
func WithFrequency(freq time.Duration) Option {
	return func(c *config) {
		c.daemonFrequency = freq
	}
}

// WithSnapshotMaxAge indicates the max-age of a snapshot,
// indicating that a new one should be generated.
func WithSnapshotMaxAge(age time.Duration) Option {
	return func(c *config) {
		c.daemonSnapshotMaxAge = age
	}
}
