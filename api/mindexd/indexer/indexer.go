package indexer

import (
	"context"
	"fmt"
	"time"

	logger "github.com/ipfs/go-log/v2"
	pow "github.com/textileio/powergate/v2/api/client"
	"github.com/textileio/textile/v2/api/mindexd/index/indexer/store"
	records "github.com/textileio/textile/v2/api/mindexd/records/collector/store"
	"go.mongodb.org/mongo-driver/mongo"
)

var (
	log = logger.Logger("index-indexer")
)

type Indexer struct {
	cfg config
	pow *pow.Client

	store  *store.Store
	rstore *records.Store

	daemonCtx       context.Context
	daemonCtxCancel context.CancelFunc
	daemonClosed    chan (struct{})
}

func New(db *mongo.Database, pow *pow.Client, rstore *records.Store, opts ...Option) (*Indexer, error) {
	config := defaultConfig
	for _, o := range opts {
		o(&config)
	}

	store, err := store.New(db)
	if err != nil {
		return nil, fmt.Errorf("creating store: %s", err)
	}

	daemonCtx, daemonCtxCancel := context.WithCancel(context.Background())
	i := &Indexer{
		cfg:    config,
		pow:    pow,
		store:  store,
		rstore: rstore,

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
