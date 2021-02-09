package indexer

import (
	"context"
	"time"

	logger "github.com/ipfs/go-log/v2"
	pow "github.com/textileio/powergate/v2/api/client"
	"github.com/textileio/textile/v2/api/mindexd/store"
)

var (
	log = logger.Logger("indexer")
)

type Indexer struct {
	cfg           config
	store         *store.Store
	pow           *pow.Client
	powAdminToken string

	daemonCtx       context.Context
	daemonCtxCancel context.CancelFunc
	daemonClosed    chan (struct{})
}

func New(pow *pow.Client, powAdminToken string, store *store.Store, opts ...Option) (*Indexer, error) {
	config := defaultConfig
	for _, o := range opts {
		o(&config)
	}

	daemonCtx, daemonCtxCancel := context.WithCancel(context.Background())
	i := &Indexer{
		cfg:           config,
		store:         store,
		pow:           pow,
		powAdminToken: powAdminToken,

		daemonCtx:       daemonCtx,
		daemonCtxCancel: daemonCtxCancel,
		daemonClosed:    make(chan struct{}),
	}

	i.runDaemon()

	// TTODO: do daemon to make history snapshots, add options.

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
