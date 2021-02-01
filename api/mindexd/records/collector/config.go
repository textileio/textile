package collector

import (
	"fmt"
	"time"
)

var (
	defaultConfig = config{
		collectOnStart: false,
		collectFreq:    60 * time.Minute,
		fetchLimit:     50,
	}
)

type config struct {
	collectOnStart bool
	collectFreq    time.Duration
	fetchLimit     int
	pows           []PowTarget
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
func WithCollectOnStart(enabled bool) Option {
	return func(c *config) {
		c.collectOnStart = enabled
	}
}

// WithCollectFrequency indicates the frequency (in mins)
// for the collector daemon.
func WithCollectFrequency(freqMins int) Option {
	return func(c *config) {
		c.collectFreq = time.Duration(freqMins) * time.Minute
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

func (pt *PowTarget) String() string {
	return fmt.Sprintf("%s at %s", pt.Name, pt.APIEndpoint)
}
