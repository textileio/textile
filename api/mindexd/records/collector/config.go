package collector

import (
	"fmt"
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

	fetchLimit int

	pows []PowTarget
}

// PowTarget describes a Powergate instance to
// collect deal/retrieval records.
type PowTarget struct {
	Name        string
	APIEndpoint string
	AdminToken  string
}

type Option func(*config)

// WithCollectorOnStart indicates if a collection should
// be fired on start.
func WithRunOnStart(enabled bool) Option {
	return func(c *config) {
		c.daemonRunOnStart = enabled
	}
}

// WithFrequency indicates the frequency (in mins)
// for the collector daemon.
func WithFrequency(freqMins int) Option {
	return func(c *config) {
		c.daemonFrequency = time.Duration(freqMins) * time.Minute
	}
}

// WithTargets indicates the Powergate targets for records
// collection.
func WithTargets(targets ...PowTarget) Option {
	return func(c *config) {
		c.pows = targets
	}
}

// WithFetchLimit indicates the maximum record batch
// size to be fetched from targets.
func WithFetchLimit(limit int) Option {
	return func(c *config) {
		c.fetchLimit = limit
	}
}

// WithFetchTimeout indicates the maximum amount of time that fetching
// from a target can take.
func WithFetchTimeout(limit int) Option {
	return func(c *config) {
		c.fetchLimit = limit
	}
}

func (pt *PowTarget) String() string {
	return fmt.Sprintf("%s at %s", pt.Name, pt.APIEndpoint)
}
