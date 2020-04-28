package buckets

import (
	"context"
	"errors"
	"sync"

	iface "github.com/ipfs/interface-go-ipfs-core"
	opt "github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/ipfs/interface-go-ipfs-core/path"
	c "github.com/textileio/textile/collections"
)

// IPNSManager handles bucket name publishing to IPNS.
type IPNSManager struct {
	keys     *c.IPNSKeys
	api      iface.NameAPI
	lock     sync.Mutex
	locks    map[string]chan struct{}
	ctxsLock sync.Mutex
	ctxs     map[string]context.CancelFunc
}

// NewIPNSManager returns a new IPNS manager.
func NewIPNSManager(keys *c.IPNSKeys, api iface.NameAPI) *IPNSManager {
	return &IPNSManager{
		keys:  keys,
		api:   api,
		ctxs:  make(map[string]context.CancelFunc),
		locks: make(map[string]chan struct{}),
	}
}

// Cancel all pending publishes.
func (m *IPNSManager) Cancel() {
	m.lock.Unlock()
	defer m.lock.Unlock()
	for _, cancel := range m.ctxs {
		cancel()
	}
}

func (m *IPNSManager) publish(pth path.Path, keyID string) {
	ptl := m.getSemaphore(keyID)
	select {
	case ptl <- struct{}{}:
		pctx, cancel := context.WithCancel(context.Background())
		m.ctxsLock.Lock()
		m.ctxs[keyID] = cancel
		m.ctxsLock.Unlock()
		err := m.publishUnsafe(pctx, pth, keyID)
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				// Logging as a warning because this often fails with "context deadline exceeded",
				// even if the entry can be found on the network (not fully saturated).
				// The publish deadline seems to be fixed at one minute. ¯\_(ツ)_/¯
				log.Warnf("error publishing path %s: %v", pth, err)
			} else {
				log.Debugf("publishing path %s was cancelled: %v", pth, err)
			}
		}
		cancel()
		m.ctxsLock.Lock()
		delete(m.ctxs, keyID)
		m.ctxsLock.Unlock()
		<-ptl
	default:
		m.ctxsLock.Lock()
		cancel, ok := m.ctxs[keyID]
		if !ok {
			return
		}
		m.ctxsLock.Unlock()
		cancel()
		m.publish(pth, keyID)
	}
}

func (m *IPNSManager) publishUnsafe(ctx context.Context, pth path.Path, keyID string) error {
	key, err := m.keys.GetByCid(ctx, keyID)
	if err != nil {
		return err
	}
	entry, err := m.api.Publish(ctx, pth, opt.Name.Key(key.Name))
	if err != nil {
		return err
	}
	log.Debugf("published %s => %s", entry.Value(), entry.Name())
	return nil
}

func (m *IPNSManager) getSemaphore(key string) chan struct{} {
	var ptl chan struct{}
	var ok bool
	m.lock.Lock()
	defer m.lock.Unlock()
	if ptl, ok = m.locks[key]; !ok {
		ptl = make(chan struct{}, 1)
		m.locks[key] = ptl
	}
	return ptl
}
