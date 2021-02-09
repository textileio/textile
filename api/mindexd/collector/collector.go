package collector

import (
	"context"
	"sync"
	"time"

	logger "github.com/ipfs/go-log/v2"
	"github.com/textileio/textile/v2/api/mindexd/store"
)

var (
	log = logger.Logger("records-collector")
)

type Collector struct {
	lock        sync.Mutex
	cfg         config
	store       *store.Store
	subscribers []chan<- struct{}

	daemonCtx       context.Context
	daemonCtxCancel context.CancelFunc
	daemonClosed    chan (struct{})
}

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

func (c *Collector) Subscribe() <-chan struct{} {
	c.lock.Lock()
	defer c.lock.Unlock()

	ch := make(chan struct{}, 1)
	c.subscribers = append(c.subscribers, ch)

	return ch
}

func (c *Collector) Close() error {
	c.daemonCtxCancel()
	<-c.daemonClosed

	return nil
}

func (c *Collector) runDaemon() {
	defer close(c.daemonClosed)

	collect := make(chan struct{}, 1)
	if c.cfg.daemonRunOnStart {
		collect <- struct{}{}
	}

	go func() {
		select {
		case <-c.daemonCtx.Done():
			return
		case <-time.After(c.cfg.daemonFrequency):
			collect <- struct{}{}
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
