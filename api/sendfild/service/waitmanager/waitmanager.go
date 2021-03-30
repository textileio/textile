package waitmanager

import (
	"context"
	"fmt"
	"sync"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/textileio/go-threads/util"
	"github.com/textileio/textile/v2/api/sendfild/service/interfaces"
)

var log = logging.Logger("waitmanager")

type WaitManager struct {
	filecoinClientBuilder interfaces.FilecoinClientBuilder
	txnStore              interfaces.TxnStore
	confidence            uint64
	waitTimeout           time.Duration
	waiting               map[string]*WaitRunner
	waitingLck            sync.Mutex
	ticker                *time.Ticker
	mainCtx               context.Context
	mainCtxCancel         context.CancelFunc
}

func New(cb interfaces.FilecoinClientBuilder, txnStore interfaces.TxnStore, confidence uint64, waitTimeout time.Duration, retryWaitFrequency time.Duration, debug bool) (*WaitManager, error) {
	if debug {
		if err := util.SetLogLevels(map[string]logging.LogLevel{
			"waitmanager": logging.LevelDebug,
		}); err != nil {
			return nil, err
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	w := &WaitManager{
		filecoinClientBuilder: cb,
		txnStore:              txnStore,
		confidence:            confidence,
		waitTimeout:           waitTimeout,
		waiting:               make(map[string]*WaitRunner),
		ticker:                time.NewTicker(retryWaitFrequency),
		mainCtx:               ctx,
		mainCtxCancel:         cancel,
	}

	if err := w.waitAllPending(ctx, true); err != nil {
		cancel()
		return nil, fmt.Errorf("calling waitAllPending: %v", err)
	}

	w.bindTicker(ctx)

	return w, nil
}

func (w *WaitManager) RegisterTxn(txnID string, messageCid string) error {
	_, err := w.getOrCreateRunner(txnID, messageCid)
	if err != nil {
		return fmt.Errorf("getting wait runner: %v", err)
	}
	return nil
}

func (w *WaitManager) Subscribe(txnID string, messageCid string, listener chan WaitResult) (CancelListenerFunc, error) {
	runner, err := w.getOrCreateRunner(txnID, messageCid)
	if err != nil {
		return nil, fmt.Errorf("getting wait runner: %v", err)
	}
	return runner.AddListener(listener)
}

func (w *WaitManager) Close() error {
	w.ticker.Stop()
	w.mainCtxCancel()
	return nil
}

func (w *WaitManager) getOrCreateRunner(txnID string, messageCid string) (*WaitRunner, error) {
	w.waitingLck.Lock()
	defer w.waitingLck.Unlock()

	runner, found := w.waiting[txnID]
	if !found {
		log.Infof("creating new wait runner %s for cid %s", txnID, messageCid)
		var err error
		runner, err = NewWaitRunner(w.mainCtx, messageCid, w.confidence, w.waitTimeout, w.txnStore, w.filecoinClientBuilder)
		if err != nil {
			return nil, err
		}
		w.waiting[txnID] = runner
		listener := make(chan WaitResult)
		if _, err := runner.AddListener(listener); err != nil {
			return nil, err
		}
		go func() {
			var err error
			select {
			case <-listener:
				err = w.deleteWaitRunner(txnID)
			case <-w.mainCtx.Done():
				err = w.deleteWaitRunner(txnID)
			}
			if err != nil {
				log.Errorf("deleting wait runner: %v", err)
			}
		}()
	}
	return runner, nil
}

func (w *WaitManager) waitAllPending(ctx context.Context, isInitialRun bool) error {
	txns, err := w.txnStore.GetAllPending(ctx, !isInitialRun)
	if err != nil {
		return err
	}
	log.Infof("found %v txns to initiate waiting on", len(txns))
	for _, txn := range txns {
		if err := w.RegisterTxn(txn.Id, txn.MessageCid); err != nil {
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

func (w *WaitManager) deleteWaitRunner(txnID string) error {
	w.waitingLck.Lock()
	defer w.waitingLck.Unlock()
	log.Infof("deleting wait runner: %s", txnID)
	runner, ok := w.waiting[txnID]
	if !ok {
		return nil
	}
	err := runner.Close()
	delete(w.waiting, txnID)
	return err
}
