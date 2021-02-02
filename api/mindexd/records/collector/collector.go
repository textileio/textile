package collector

import (
	"context"
	"fmt"
	"time"

	logger "github.com/ipfs/go-log/v2"
	"github.com/textileio/textile/v2/api/mindexd/records/collector/store"
	"go.mongodb.org/mongo-driver/mongo"
)

var (
	log = logger.Logger("records-collector")
)

type Collector struct {
	cfg   config
	store *store.Store

	daemonCtx       context.Context
	daemonCtxCancel context.CancelFunc
	daemonClosed    chan (struct{})
}

func New(db *mongo.Database, opts ...Option) (*Collector, error) {
	config := defaultConfig
	for _, o := range opts {
		o(&config)
	}

	store, err := store.New(db)
	if err != nil {
		return nil, fmt.Errorf("creating store: %s", err)
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

func (c *Collector) Close() error {
	c.daemonCtxCancel()
	<-c.daemonClosed

	return nil
}

func (c *Collector) runDaemon() {
	defer close(c.daemonClosed)

	for _, t := range c.cfg.pows {
		log.Infof("Powergate target: %s", t)
	}

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
			c.collectTargets(c.daemonCtx)
		}
	}
}
