package waitmanager

import (
	"context"
	"fmt"
	"sync"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/textileio/powergate/v2/lotus"
	"github.com/textileio/textile/v2/api/sendfild/service/store"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var log = logging.Logger("sendfil-waitmanager")

type WaitManager struct {
	clientBuilder lotus.ClientBuilder
	store         *store.Store
	confidence    uint64
	waitTimeout   time.Duration
	waiting       map[primitive.ObjectID]*WaitRunner
	waitingLck    sync.Mutex
	ticker        *time.Ticker
	mainCtx       context.Context
	mainCtxCancel context.CancelFunc
}

func New(cb lotus.ClientBuilder, store *store.Store, confidence uint64, waitTimeout time.Duration, retryWaitFrequency time.Duration) (*WaitManager, error) {
	ctx, cancel := context.WithCancel(context.Background())

	w := &WaitManager{
		clientBuilder: cb,
		store:         store,
		confidence:    confidence,
		waitTimeout:   waitTimeout,
		waiting:       make(map[primitive.ObjectID]*WaitRunner),
		ticker:        time.NewTicker(retryWaitFrequency),
		mainCtx:       ctx,
		mainCtxCancel: cancel,
	}

	if err := w.waitAllPending(ctx, true); err != nil {
		cancel()
		return nil, fmt.Errorf("calling waitAllPending: %v", err)
	}

	w.bindTicker(ctx)

	return w, nil
}

func (w *WaitManager) RegisterTxn(objID primitive.ObjectID, messageCid string) error {
	runner, err := w.getOrCreateRunner(objID, messageCid)
	if err != nil {
		return fmt.Errorf("getting wait runner: %v", err)
	}
	runner.Start()
	return nil
}

func (w *WaitManager) Subscribe(objID primitive.ObjectID, messageCid string, listener chan WaitResult) (CancelListenerFunc, error) {
	runner, err := w.getOrCreateRunner(objID, messageCid)
	if err != nil {
		return nil, fmt.Errorf("getting wait runner: %v", err)
	}
	cancel := runner.AddListener(listener)
	runner.Start()
	return cancel, nil
}

func (w *WaitManager) Close() error {
	w.ticker.Stop()
	w.mainCtxCancel()
	return nil
}

func (w *WaitManager) getOrCreateRunner(objID primitive.ObjectID, messageCid string) (*WaitRunner, error) {
	w.waitingLck.Lock()
	defer w.waitingLck.Unlock()

	runner, found := w.waiting[objID]
	if !found {
		var err error
		runner, err = NewWaitRunner(w.mainCtx, messageCid, w.confidence, w.waitTimeout, w.store, w.clientBuilder)
		if err != nil {
			return nil, err
		}
		w.waiting[objID] = runner
		listener := make(chan WaitResult)
		_ = runner.AddListener(listener)
		go func() {
			var err error
			select {
			case <-listener:
				err = w.deleteWaitRunner(objID)
			case <-w.mainCtx.Done():
				err = w.deleteWaitRunner(objID)
			}
			if err != nil {
				log.Errorf("deleting wait runner: %v", err)
			}
		}()
	}
	return runner, nil
}

func (w *WaitManager) waitAllPending(ctx context.Context, isInitialRun bool) error {
	txns, err := w.store.GetAllPending(ctx, !isInitialRun)
	if err != nil {
		return err
	}
	log.Infof("found %v txns to initiate waiting on", len(txns))
	for _, txn := range txns {
		latestMessageCid, err := txn.LatestMsgCid()
		if err != nil {
			return err
		}
		if err := w.RegisterTxn(txn.ID, latestMessageCid.Cid); err != nil {
			return err
		}
	}
	return nil
}

func (w *WaitManager) bindTicker(ctx context.Context) {
	go func() {
		for {
			select {
			case <-w.ticker.C:
				if err := w.waitAllPending(ctx, false); err != nil {
					log.Errorf("waitAllPending from ticker: %v", err)
				}
			case <-ctx.Done():
				log.Info("unbinding ticker")
				return
			}
		}
	}()
}

func (w *WaitManager) deleteWaitRunner(objID primitive.ObjectID) error {
	w.waitingLck.Lock()
	defer w.waitingLck.Unlock()
	runner, ok := w.waiting[objID]
	if !ok {
		return nil
	}
	err := runner.Close()
	delete(w.waiting, objID)
	return err
}
