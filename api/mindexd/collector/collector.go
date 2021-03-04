package collector

import (
	"context"
	"sync"
	"time"

	logger "github.com/ipfs/go-log/v2"
	"github.com/textileio/textile/v2/api/mindexd/store"
)

var (
	log = logger.Logger("collector")
)

// Collector is responsible for fetching storage/retrieval records from
// external Powergate instances, and merging them in a unified database.
type Collector struct {
	lock        sync.Mutex
	cfg         config
	store       *store.Store
	subscribers []chan<- struct{}

	daemonCtx       context.Context
	daemonCtxCancel context.CancelFunc
	daemonClosed    chan (struct{})
}

// New returns a new Collector.
func New(store *store.Store, opts ...Option) (*Collector, error) {
	config := defaultConfig
	for _, o := range opts {
		o(&config)
	}

	daemonCtx, daemonCtxCancel := context.WithCancel(context.Background())
	c := &Collector{
		cfg:   config,
		store: store,

		daemonCtx:       daemonCtx,
		daemonCtxCancel: daemonCtxCancel,
		daemonClosed:    make(chan struct{}),
	}

	go c.runDaemon()

	return c, nil
}

// Subscribe returns a notification channel that will get
// pushed signales whenever a new batch of records was imported.
// This is useful for interested parties knowing about new
// record's data might be available.
func (c *Collector) Subscribe() <-chan struct{} {
	c.lock.Lock()
	defer c.lock.Unlock()

	ch := make(chan struct{}, 1)
	c.subscribers = append(c.subscribers, ch)

	return ch
}

// Close closes the Collector.
func (c *Collector) Close() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.daemonCtxCancel()
	<-c.daemonClosed

	return nil
}

// runDaemon creates the background daemon that
// will poll known Powergate targets to fetch
// new records information.
func (c *Collector) runDaemon() {
	defer close(c.daemonClosed)

	collect := make(chan struct{}, 1)
	if c.cfg.daemonRunOnStart {
		collect <- struct{}{}
	}

	go func() {
		for {
			select {
			case <-c.daemonCtx.Done():
				return
			case <-time.After(c.cfg.daemonFrequency):
				collect <- struct{}{}
			}
		}
	}()

	for {
		select {
		case <-c.daemonCtx.Done():
			log.Infof("closing daemon")
			return
		case <-collect:
			totalImported := c.collectTargets(c.daemonCtx)
			if totalImported > 0 {
				c.notifySubscribers()
			}
			log.Infof("daemon finished importing %d records", totalImported)
		}
	}
}

func (c *Collector) notifySubscribers() {
	c.lock.Lock()
	defer c.lock.Unlock()

	for _, c := range c.subscribers {
		select {
		case c <- struct{}{}:
		default:
			log.Warnf("slow subscriber, skipping")
		}
	}
}
