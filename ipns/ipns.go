package ipns

import (
	"context"
	"errors"
	"sync"

	logging "github.com/ipfs/go-log"
	iface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/options"
	opt "github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/libp2p/go-libp2p-core/peer"
	mbase "github.com/multiformats/go-multibase"
	"github.com/textileio/go-threads/core/thread"
	tutil "github.com/textileio/go-threads/util"
	c "github.com/textileio/textile/collections"
	"github.com/textileio/textile/util"
)

var log = logging.Logger("ipns")

const (
	// nameLen is the length of the random IPNS key name.
	nameLen = 16
	// maxCancelPublishTries is the number of time cancelling a publish is allowed to fail.
	maxCancelPublishTries = 10
)

// Manager handles bucket name publishing to IPNS.
type Manager struct {
	keys    *c.IPNSKeys
	keyAPI  iface.KeyAPI
	nameAPI iface.NameAPI

	sync.Mutex
	keyLocks map[string]chan struct{}
	ctxsLock sync.Mutex
	ctxs     map[string]context.CancelFunc
}

// NewManager returns a new IPNS manager.
func NewManager(keys *c.IPNSKeys, keyAPI iface.KeyAPI, nameAPI iface.NameAPI, debug bool) (*Manager, error) {
	if debug {
		if err := tutil.SetLogLevels(map[string]logging.LogLevel{
			"ipns": logging.LevelDebug,
		}); err != nil {
			return nil, err
		}
	}
	return &Manager{
		keys:     keys,
		keyAPI:   keyAPI,
		nameAPI:  nameAPI,
		ctxs:     make(map[string]context.CancelFunc),
		keyLocks: make(map[string]chan struct{}),
	}, nil
}

// CreateKey generates and saves a new IPNS key.
func (m *Manager) CreateKey(ctx context.Context, dbID thread.ID) (keyID string, err error) {
	key, err := m.keyAPI.Generate(ctx, util.MakeToken(nameLen), opt.Key.Type(options.RSAKey))
	if err != nil {
		return
	}
	keyID, err = peer.ToCid(key.ID()).StringOfBase(mbase.Base32)
	if err != nil {
		return
	}
	if err = m.keys.Create(ctx, key.Name(), keyID, dbID); err != nil {
		return
	}
	return keyID, nil
}

// RemoveKey removes an IPNS key.
func (m *Manager) RemoveKey(ctx context.Context, keyID string) error {
	key, err := m.keys.GetByCid(ctx, keyID)
	if err != nil {
		return err
	}
	if _, err = m.keyAPI.Remove(ctx, key.Name); err != nil {
		return err
	}
	return m.keys.Delete(ctx, key.Name)
}

// Publish publishes a path to IPNS with key ID.
// Publishing can take up to a minute. Pending publishes are cancelled by consecutive
// calls with the same key ID, which results in only the most recent publish succeeding.
func (m *Manager) Publish(pth path.Path, keyID string) {
	ptl := m.getSemaphore(keyID)
	try := 0
	for {
		select {
		case ptl <- struct{}{}:
			pctx, cancel := context.WithCancel(context.Background())
			m.ctxsLock.Lock()
			m.ctxs[keyID] = cancel
			m.ctxsLock.Unlock()
			if err := m.publishUnsafe(pctx, pth, keyID); err != nil {
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
			return
		default:
			m.ctxsLock.Lock()
			cancel, ok := m.ctxs[keyID]
			m.ctxsLock.Unlock()
			if ok {
				cancel()
			} else {
				try++
				if try > maxCancelPublishTries {
					log.Warnf("failed to publish path %s: max tries exceeded", pth)
					return
				} else {
					log.Debugf("failed to cancel publish (%v tries remaining)", maxCancelPublishTries-try)
				}
			}
		}
	}
}

// Cancel all pending publishes.
func (m *Manager) Cancel() {
	m.Lock()
	defer m.Unlock()
	for _, cancel := range m.ctxs {
		cancel()
	}
}

func (m *Manager) publishUnsafe(ctx context.Context, pth path.Path, keyID string) error {
	key, err := m.keys.GetByCid(ctx, keyID)
	if err != nil {
		return err
	}
	entry, err := m.nameAPI.Publish(ctx, pth, opt.Name.Key(key.Name))
	if err != nil {
		return err
	}
	log.Debugf("published %s => %s", entry.Value(), entry.Name())
	return nil
}

func (m *Manager) getSemaphore(key string) chan struct{} {
	var ptl chan struct{}
	var ok bool
	m.Lock()
	defer m.Unlock()
	if ptl, ok = m.keyLocks[key]; !ok {
		ptl = make(chan struct{}, 1)
		m.keyLocks[key] = ptl
	}
	return ptl
}
