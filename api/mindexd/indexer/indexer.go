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
	daemonClosed    chan struct{}
	daemonSub       <-chan struct{}
}

func New(pow *pow.Client, sub <-chan struct{}, powAdminToken string, store *store.Store, opts ...Option) (*Indexer, error) {
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
		daemonSub:       sub,
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
			log.Infof("daemon shutting down")
			return
		case <-time.After(i.cfg.daemonFrequency):
			log.Infof("daemon ticker fired")
			collect <- struct{}{}
		case <-i.daemonSub:
			log.Infof("received new records notification")
			collect <- struct{}{}
		}
	}()

	go func() {
		lastSnapshot, err := i.store.GetLastIndexSnapshotTime(i.daemonCtx)
		if err != nil {
			log.Errorf("get last index snapshot: %s", err)
			return
		}
		select {
		case <-i.daemonCtx.Done():
			log.Infof("daemon snapshot shutting down")
		case <-time.After(15 * time.Minute):
			if time.Now().Sub(lastSnapshot) > i.cfg.daemonSnapshotMaxAge {
				if err := i.store.GenerateMinerIndexSnapshot(i.daemonCtx); err != nil {
					log.Errorf("generating index snapshot: %s", err)
				}
			}
			lastSnapshot = time.Now()
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
