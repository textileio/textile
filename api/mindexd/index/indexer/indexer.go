package indexer

import (
	"context"
	"time"

	logger "github.com/ipfs/go-log/v2"
	"go.mongodb.org/mongo-driver/mongo"
)

var (
	log = logger.Logger("index-indexer")
)

type Indexer struct {
	cfg config

	daemonCtx       context.Context
	daemonCtxCancel context.CancelFunc
	daemonClosed    chan (struct{})
}

func New(db *mongo.Database, opts ...Option) (*Indexer, error) {
	config := defaultConfig
	for _, o := range opts {
		o(&config)
	}

	daemonCtx, daemonCtxCancel := context.WithCancel(context.Background())
	i := &Indexer{
		cfg: config,

		daemonCtx:       daemonCtx,
		daemonCtxCancel: daemonCtxCancel,
		daemonClosed:    make(chan struct{}),
	}

	i.runDaemon()
	return i, nil
}

func (i *Indexer) Close() error {
	i.daemonCtxCancel()
	<-i.daemonClosed

	return nil
}

func (i *Indexer) runDaemon() {
	defer close(i.daemonClosed)

	collect := make(chan struct{}, 1)
	if i.cfg.daemonRunOnStart {
		collect <- struct{}{}
	}

	go func() {
		select {
		case <-i.daemonCtx.Done():
			return
		case <-time.After(i.cfg.daemonFrequency):
			collect <- struct{}{}
		}
	}()

	for {
		select {
		case <-i.daemonCtx.Done():
			log.Infof("closing daemon")
			return
		case <-collect:
			i.generateIndex(i.daemonCtx)
		}
	}
}
