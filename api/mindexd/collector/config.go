package collector

import (
	"time"
)

var (
	defaultConfig = config{
		daemonRunOnStart: false,
		daemonFrequency:  60 * time.Minute,

		fetchTimeout: time.Minute,
		fetchLimit:   50,
	}
)

type config struct {
	daemonRunOnStart bool
	daemonFrequency  time.Duration
	fetchTimeout     time.Duration
	fetchLimit       int
}

// Option parametrizes a collector configuration.
type Option func(*config)

// WithRunOnStart indicates if a collection should
// be fired on start.
func WithRunOnStart(enabled bool) Option {
	return func(c *config) {
		c.daemonRunOnStart = enabled
	}
}

// WithFrequency indicates the frequency for the collector daemon.
func WithFrequency(freq time.Duration) Option {
	return func(c *config) {
		c.daemonFrequency = freq
	}
}

// WithFetchLimit indicates the maximum record batch
// size to be fetched from targets.
func WithFetchLimit(limit int) Option {
	return func(c *config) {
		c.fetchLimit = limit
	}
}

// WithFetchTimeout indicates the maximum amount of time
// that fetching from a target can take.
func WithFetchTimeout(timeout time.Duration) Option {
	return func(c *config) {
		c.fetchTimeout = timeout
	}
}
