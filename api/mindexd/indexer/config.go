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
	daemonRunOnStart bool
	daemonFrequency  time.Duration
}

type Option func(*config)

// WithRunOnStart indicates if the index must be generated
// when the deamon is started.
func WithRunOnStart(enabled bool) Option {
	return func(c *config) {
		c.daemonRunOnStart = enabled
	}
}

// WithFrequency indicates the frequency (in mins)
// for the indexer daemon.
func WithFrequency(freqMins int) Option {
	return func(c *config) {
		c.daemonFrequency = time.Duration(freqMins) * time.Minute
	}
}
