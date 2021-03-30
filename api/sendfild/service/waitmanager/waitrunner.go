package waitmanager

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/textileio/textile/v2/api/sendfild/service/interfaces"
)

type WaitResult struct {
	LatestMessageCid string
	Err              error
}

type WaitRunner struct {
	messageCid       string
	confidence       uint64
	waitTimeout      time.Duration
	txnStore         interfaces.TxnStore
	filecoinClient   interfaces.FilecoinClient
	closeLotusClient func()
	listeners        []chan WaitResult
	lock             sync.Mutex
	mainCtx          context.Context
	mainCtxCancel    context.CancelFunc
	waitCtx          context.Context
	waitCtxCancel    context.CancelFunc
	done             bool
}

func NewWaitRunner(ctx context.Context, messageCid string, confidence uint64, waitTimeout time.Duration, txnStore interfaces.TxnStore, cb interfaces.FilecoinClientBuilder) (*WaitRunner, error) {
	mainCtx, mainCtxCancel := context.WithCancel(ctx)

	w := &WaitRunner{
		messageCid:    messageCid,
		confidence:    confidence,
		waitTimeout:   waitTimeout,
		txnStore:      txnStore,
		mainCtx:       mainCtx,
		mainCtxCancel: mainCtxCancel,
	}

	var err error
	w.filecoinClient, w.closeLotusClient, err = cb(mainCtx)
	if err != nil {
		return nil, fmt.Errorf("creating lotus client: %v", err)
	}

	w.waitCtx, w.waitCtxCancel = context.WithTimeout(w.mainCtx, w.waitTimeout)
	w.waitAndNotify()

	return w, nil
}

type CancelListenerFunc = func()

func (w *WaitRunner) AddListener(listener chan WaitResult) (CancelListenerFunc, error) {
	w.lock.Lock()
	defer w.lock.Unlock()
	if w.done {
		return nil, fmt.Errorf("runner already done, can't add listener")
	}
	w.listeners = append(w.listeners, listener)
	return func() {
		w.lock.Lock()
		defer w.lock.Unlock()
		index := -1
		for i, l := range w.listeners {
			if l == listener {
				index = i
				break
			}
		}
		if index >= 0 {
			// ToDo: Write unit test to make sure adding multiple listeners and removing some works for remaining listeners.
			w.listeners[index] = w.listeners[len(w.listeners)-1]
			w.listeners = w.listeners[:len(w.listeners)-1]
		}
	}, nil
}

func (w *WaitRunner) Close() error {
	w.lock.Lock()
	defer w.lock.Unlock()
	w.closeLotusClient()
	w.waitCtxCancel()
	w.mainCtxCancel()
	return nil
}

func (w *WaitRunner) waitAndNotify() {
	go func() {
		c, err := cid.Decode(w.messageCid)
		if err != nil {
			if err := w.txnStore.Fail(w.mainCtx, w.messageCid, err.Error()); err != nil {
				w.notifyErr(fmt.Errorf("failing txn in store: %v", err))
				return
			}
			w.notify(w.messageCid)
			return
		}

		if err := w.txnStore.SetWaiting(w.mainCtx, w.messageCid, true); err != nil {
			w.notifyErr(fmt.Errorf("setting txn to waiting: %v", err))
			return
		}

		res, err := w.filecoinClient.StateWaitMsg(w.waitCtx, c, w.confidence)

		if setWaitingErr := w.txnStore.SetWaiting(w.mainCtx, w.messageCid, false); setWaitingErr != nil {
			w.notifyErr(fmt.Errorf("setting txn to not waiting: %v", setWaitingErr))
			return
		}

		// The err check here corresponds to the StateWaitMsg, don't move since this is correct.
		if err != nil {
			// If for some reason the lotus node doesn't know about the cid, consider that a final error.
			if strings.Contains(err.Error(), "block not found") {
				log.Errorf("calling StateWaitMsg block not found: %s", err.Error())
				if err := w.txnStore.Fail(w.mainCtx, w.messageCid, "block not found in lotus"); err != nil {
					w.notifyErr(fmt.Errorf("failing txn in store: %v", err))
					return
				}
				w.notify(w.messageCid)
				return
			}
			// ugly but seems to be the only way to detect this
			if strings.Contains(err.Error(), "context canceled") {
				w.notifyErr(fmt.Errorf("waiting for txn status timed out, but txn is still processing, query txn again if needed"))
				return
			}
			w.notifyErr(fmt.Errorf("calling StateWaitMsg: %v", err))
			return
		}

		if res.Receipt.ExitCode.IsError() {
			if res.Receipt.ExitCode.IsSendFailure() {
				log.Errorf("received exit code send failure: %s", res.Receipt.ExitCode.String())
			} else {
				log.Infof("received exit code error: %s", res.Receipt.ExitCode.String())
			}
			if err := w.txnStore.Fail(w.mainCtx, w.messageCid, fmt.Sprintf("error exit code: %v", res.Receipt.ExitCode.Error())); err != nil {
				w.notifyErr(fmt.Errorf("failing txn in store: %v", err))
				return
			}

			cidToNotify := w.messageCid
			if res.Message.Defined() {
				cidToNotify = res.Message.String()
			}

			w.notify(cidToNotify)
			return
		}

		if err := w.txnStore.Activate(w.mainCtx, w.messageCid, res.Message.String()); err != nil {
			w.notifyErr(fmt.Errorf("activating txn in store: %v", err))
			return
		}
		w.notify(res.Message.String())
	}()
}

func (w *WaitRunner) notifyErr(err error) {
	w.lock.Lock()
	defer w.lock.Unlock()
	w.done = true
	log.Infof("notifying error: %s", err.Error())
	for _, l := range w.listeners {
		l <- WaitResult{Err: err}
		close(l)
	}
}

func (w *WaitRunner) notify(messageCid string) {
	w.lock.Lock()
	defer w.lock.Unlock()
	w.done = true
	log.Infof("notifying complete for cid: %s", messageCid)
	for _, l := range w.listeners {
		l <- WaitResult{LatestMessageCid: messageCid}
		close(l)
	}
}
