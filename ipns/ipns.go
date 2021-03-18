package ipns

import (
	"context"
	"errors"
	"sync"
	"time"

	logging "github.com/ipfs/go-log/v2"
	iface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/libp2p/go-libp2p-core/peer"
	mbase "github.com/multiformats/go-multibase"
	cron "github.com/robfig/cron/v3"
	"github.com/textileio/go-threads/core/thread"
	tutil "github.com/textileio/go-threads/util"
	mdb "github.com/textileio/textile/v2/mongodb"
	"github.com/textileio/textile/v2/util"
	"golang.org/x/sync/errgroup"
)

var log = logging.Logger("ipns")

const (
	// nameLen is the length of the random IPNS key name.
	nameLen = 16
	// publishTimeout
	publishTimeout = time.Minute * 2
	// maxCancelPublishTries is the number of time cancelling a publish is allowed to fail.
	maxCancelPublishTries = 10
	// list all keys timeout
	listKeysTimeout = time.Hour
)

// Manager handles bucket name publishing to IPNS.
type Manager struct {
	keys    *mdb.IPNSKeys
	keyAPI  iface.KeyAPI
	nameAPI iface.NameAPI

	sync.Mutex
	keyLocks             map[string]chan struct{}
	ctxsLock             sync.Mutex
	ctxs                 map[string]context.CancelFunc
	republisher          *cron.Cron
	republishConcurrency int
}

// NewManager returns a new IPNS manager.
func NewManager(
	keys *mdb.IPNSKeys,
	keyAPI iface.KeyAPI,
	nameAPI iface.NameAPI,
	republishConcurrency int,
	debug bool,
) (*Manager, error) {
	if debug {
		if err := tutil.SetLogLevels(map[string]logging.LogLevel{
			"ipns": logging.LevelDebug,
		}); err != nil {
			return nil, err
		}
	}
	return &Manager{
		keys:                 keys,
		keyAPI:               keyAPI,
		nameAPI:              nameAPI,
		ctxs:                 make(map[string]context.CancelFunc),
		keyLocks:             make(map[string]chan struct{}),
		republisher:          cron.New(),
		republishConcurrency: republishConcurrency,
	}, nil
}

// StartRepublishing initializes a key republishing cron
func (m *Manager) StartRepublishing(schedule string) error {
	if _, err := m.republisher.AddFunc(schedule, func() {
		if err := m.republish(); err != nil {
			log.Errorf("republishing ipns keys: %v", err)
		}
	}); err != nil {
		log.Errorf("republishing aborted: %v", err)
		return err
	}
	m.republisher.Start()
	return nil
}

// CreateKey generates and saves a new IPNS key.
func (m *Manager) CreateKey(ctx context.Context, dbID thread.ID, path path.Path) (keyID string, err error) {
	key, err := m.keyAPI.Generate(ctx, util.MakeToken(nameLen), options.Key.Type(options.RSAKey))
	if err != nil {
		return
	}
	keyID, err = peer.ToCid(key.ID()).StringOfBase(mbase.Base32)
	if err != nil {
		return
	}
	if err = m.keys.Create(ctx, key.Name(), keyID, dbID, path.String()); err != nil {
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
	ctx, cancel := context.WithTimeout(context.Background(), publishTimeout)
	defer cancel()
	key, err := m.keys.GetByCid(ctx, keyID)
	if err != nil {
		log.Error("key not found: %s", keyID)
		return
	}
	err = m.keys.SetPath(ctx, pth.String(), key.Name)
	if err != nil {
		log.Error("set path failed: %s", keyID)
		return
	}
	m.publish(pth, keyID)
}

// Close manager.
func (m *Manager) Close() {
	ctx := m.republisher.Stop()
	<-ctx.Done()
	log.Info("republisher was shutdown")
	m.cancel()
	log.Info("all pending ipns publishes were cancelled")
}

// cancel all pending publishes.
func (m *Manager) cancel() {
	m.Lock()
	defer m.Unlock()
	m.ctxsLock.Lock()
	defer m.ctxsLock.Unlock()
	for _, cancel := range m.ctxs {
		cancel()
	}
}

func (m *Manager) publish(pth path.Path, keyID string) {
	ptl := m.getSemaphore(keyID)
	try := 0
	for {
		select {
		case ptl <- struct{}{}:
			pctx, cancel := context.WithTimeout(context.Background(), publishTimeout)
			m.ctxsLock.Lock()
			m.ctxs[keyID] = cancel
			m.ctxsLock.Unlock()
			if err := m.publishUnsafe(pctx, pth, keyID); err != nil {
				if !errors.Is(err, context.Canceled) {
					// The publish saturation did not meet the default level before the context expired.
					// In most cases, the entry can still be discovered on the network.
					log.Debugf("error publishing path %s: %v", pth, err)
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
					log.Debugf("failed to publish path %s: max tries exceeded", pth)
					return
				} else {
					log.Debugf("failed to cancel publish (%v tries remaining)", maxCancelPublishTries-try)
				}
			}
		}
	}
}
func (m *Manager) publishUnsafe(ctx context.Context, pth path.Path, keyID string) error {
	key, err := m.keys.GetByCid(ctx, keyID)
	if err != nil {
		return err
	}
	entry, err := m.nameAPI.Publish(ctx, pth, options.Name.Key(key.Name))
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

func (m *Manager) republish() error {
	ctx, cancel := context.WithTimeout(context.Background(), listKeysTimeout)
	defer cancel()
	start := time.Now()
	keys, err := m.keys.List(ctx)
	if err != nil {
		return err
	}
	withPath := 0
	eg, gctx := errgroup.WithContext(context.Background())
	lim := make(chan struct{}, m.republishConcurrency)
	for _, key := range keys {
		key := key
		if key.Path != "" {
			withPath++
			lim <- struct{}{}
			eg.Go(func() error {
				defer func() { <-lim }()
				if gctx.Err() != nil {
					return nil
				}
				m.publish(path.New(key.Path), key.Cid)
				return nil
			})
		}
	}
	for i := 0; i < cap(lim); i++ {
		lim <- struct{}{}
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	log.Infof("republished %d/%d keys in %v", withPath, len(keys), time.Since(start))
	return nil
}
