package local

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/textileio/go-threads/api/client"
	"github.com/textileio/go-threads/core/thread"
	pb "github.com/textileio/textile/api/buckets/pb"
	"github.com/textileio/textile/api/common"
	"github.com/textileio/textile/buckets"
	"github.com/textileio/textile/cmd"
	"github.com/textileio/textile/util"
)

var (
	// ErrUpToDate indicates there are no locally staged changes.
	ErrUpToDate = errors.New("everything up-to-date")
	// ErrAborted indicates the caller aborted the operation via a confirm function.
	ErrAborted = errors.New("operation aborted by caller")
)

// PathEvent describes a path event that occurred.
// These events can be used to display live info during path uploads/downloads.
type PathEvent struct {
	// Path relative to the bucket's cwd.
	Path string
	// Path cid if known.
	Cid cid.Cid
	// Type of event.
	Type PathEventType
	// Path total size.
	Size int64
	// Progress of the event if known (useful for upload/download progress).
	Progress int64
}

// PathEventType is the type of path event.
type PathEventType int

const (
	// PathStart indicates a path has begun uploading/downloading.
	PathStart PathEventType = iota
	// PathComplete indicates a path has completed uploading/downloading.
	PathComplete
	// FileStart indicates a file under path has begun uploading/downloading.
	FileStart
	// FileProgress indicates a file has made some progress uploading/downloading.
	FileProgress
	// File complete indicates a file has completed uploading/downloading.
	FileComplete
	// FileRemoved indicates a file has been removed.
	FileRemoved
)

// Bucket is a local-first object storage and synchronization model built
// on ThreadDB, IPFS, and Filecoin.
// A bucket represents a dynamic Unixfs directory with auto-updating
// IPNS resolution, website rendering, and Filecoin archivng.
//
// Private buckets are fully encrypted using AES-CTR + AES-512 HMAC (see https://github.com/textileio/dcrypto for more).
// Both Unixfs node metadata (size, links, etc.) and node data (files) are obfunscated by encryption.
// The AES and HMAC keys used for bucket encryption are stored in the ThreadDB collection instance.
// This setup allows for bucket access to inherit from thread ACL rules.
//
// Additionally, files can be further protected by password-based encryption before they are added to the bucket.
// See EncryptLocalPath and DecryptLocalPath for more.
type Bucket struct {
	cwd     string
	conf    *cmd.Config
	clients *cmd.Clients
	auth    AuthFunc
	repo    *Repo
	links   *Links

	pushBlock chan struct{}
	sync.Mutex
}

// Key returns the bucket's unique key identifier, which is also an IPNS public key.
func (b *Bucket) Key() string {
	return b.conf.Viper.GetString("key")
}

// Thread returns the bucket's thread ID.
func (b *Bucket) Thread() (id thread.ID, err error) {
	ids := b.conf.Viper.GetString("thread")
	if ids == "" {
		return thread.Undef, nil
	}
	return thread.Decode(ids)
}

// Path returns the bucket's top-level local filesystem path.
func (b *Bucket) Path() (string, error) {
	conf := b.conf.Viper.ConfigFileUsed()
	if conf == "" {
		return "", ErrNotABucket
	}
	return filepath.Dir(filepath.Dir(conf)), nil
}

// BucketInfo wraps info about a bucket.
type BucketInfo struct {
	Key       string        `json:"key"`
	Name      string        `json:"name"`
	Path      path.Resolved `json:"path"`
	Thread    thread.ID     `json:"id"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// Info returns info about a bucket from the remote.
func (b *Bucket) Info(ctx context.Context) (info BucketInfo, err error) {
	ctx, err = b.context(ctx)
	if err != nil {
		return
	}
	rep, err := b.clients.Buckets.Root(ctx, b.Key())
	if err != nil {
		return
	}
	return pbRootToInfo(rep.Root)
}

func pbRootToInfo(r *pb.Root) (info BucketInfo, err error) {
	name := "unnamed"
	if r.Name != "" {
		name = r.Name
	}
	id, err := thread.Decode(r.Thread)
	if err != nil {
		return
	}
	pth, err := util.NewResolvedPath(r.Path)
	if err != nil {
		return
	}
	return BucketInfo{
		Key:       r.Key,
		Name:      name,
		Path:      pth,
		Thread:    id,
		CreatedAt: time.Unix(0, r.CreatedAt),
		UpdatedAt: time.Unix(0, r.UpdatedAt),
	}, nil
}

// Roots wraps local and remote root cids.
// If the bucket is not private (encrypted), these will be the same.
type Roots struct {
	Local  cid.Cid `json:"local"`
	Remote cid.Cid `json:"remote"`
}

// Roots returns the bucket's current local and remote root cids.
func (b *Bucket) Roots(ctx context.Context) (roots Roots, err error) {
	lc, rc, err := b.repo.Root()
	if err != nil {
		return
	}
	if !rc.Defined() {
		rc, err = b.getRemoteRoot(ctx)
		if err != nil {
			return
		}
	}
	return Roots{Local: lc, Remote: rc}, nil
}

// Links wraps remote link info for a bucket.
type Links struct {
	// URL is the thread URL, which maps to a ThreadDB collection instance.
	URL string `json:"url"`
	// WWW is the URL at which the bucket will be rendered as a website (requires remote DNS configuration).
	WWW string `json:"www"`
	// IPNS is the bucket IPNS address.
	IPNS string `json:"ipns"`
}

// RemoteLinks returns the remote links for the bucket.
func (b *Bucket) RemoteLinks(ctx context.Context) (links Links, err error) {
	if b.links != nil {
		return *b.links, nil
	}
	ctx, err = b.context(ctx)
	if err != nil {
		return
	}
	res, err := b.clients.Buckets.Links(ctx, b.Key())
	if err != nil {
		return
	}
	links = Links{URL: res.URL, WWW: res.WWW, IPNS: res.IPNS}
	b.links = &links
	return links, err
}

// DBInfo returns info about the bucket's ThreadDB.
// This info can be used to add replicas or additional peers to the bucket.
func (b *Bucket) DBInfo(ctx context.Context) (info *client.DBInfo, err error) {
	ctx, err = b.context(ctx)
	if err != nil {
		return
	}
	id, err := b.Thread()
	if err != nil {
		return
	}
	return b.clients.Threads.GetDBInfo(ctx, id)
}

// CatRemotePath writes the content of the remote path to writer.
func (b *Bucket) CatRemotePath(ctx context.Context, pth string, w io.Writer) error {
	ctx, err := b.context(ctx)
	if err != nil {
		return err
	}
	return b.clients.Buckets.PullPath(ctx, b.Key(), pth, w)
}

// Destroy deletes completely deletes the local and remote bucket.
func (b *Bucket) Destroy(ctx context.Context) error {
	b.Lock()
	defer b.Unlock()
	bp, err := b.Path()
	if err != nil {
		return err
	}
	ctx, err = b.context(ctx)
	if err != nil {
		return err
	}
	if err := b.clients.Buckets.Remove(ctx, b.Key()); err != nil {
		cmd.Fatal(err)
	}
	_ = os.RemoveAll(filepath.Join(bp, buckets.SeedName))
	_ = os.RemoveAll(filepath.Join(bp, b.conf.Dir))
	return nil
}

func (b *Bucket) loadLocalRepo(ctx context.Context, pth, name string, setCidVersion bool) error {
	r, err := NewRepo(pth, name, options.BalancedLayout)
	if err != nil {
		return err
	}
	b.repo = r
	if setCidVersion {
		if err = b.setRepoCidVersion(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (b *Bucket) setRepoCidVersion(ctx context.Context) error {
	r, err := b.Roots(ctx)
	if err != nil {
		return err
	}
	b.repo.SetCidVersion(int(r.Remote.Version()))
	return nil
}

func (b *Bucket) containsPath(pth string) (c bool, err error) {
	bp, err := b.Path()
	if err != nil {
		return
	}
	ar, err := filepath.Abs(bp)
	if err != nil {
		return
	}
	ap, err := filepath.Abs(pth)
	if err != nil {
		return
	}
	return strings.HasPrefix(ap, ar), nil
}

func (b *Bucket) getRemoteRoot(ctx context.Context) (cid.Cid, error) {
	ctx, err := b.context(ctx)
	if err != nil {
		return cid.Undef, err
	}
	rr, err := b.clients.Buckets.Root(ctx, b.Key())
	if err != nil {
		return cid.Undef, err
	}
	rp, err := util.NewResolvedPath(rr.Root.Path)
	if err != nil {
		return cid.Undef, err
	}
	return rp.Cid(), nil
}

func (b *Bucket) context(ctx context.Context) (context.Context, error) {
	id, err := b.Thread()
	if err != nil {
		return nil, err
	}
	ctx = common.NewThreadIDContext(ctx, id)
	if b.auth != nil {
		ctx = b.auth(ctx)
	}
	return ctx, nil
}
