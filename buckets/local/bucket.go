package local

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/db"
	pb "github.com/textileio/textile/v2/api/bucketsd/pb"
	"github.com/textileio/textile/v2/api/common"
	"github.com/textileio/textile/v2/buckets"
	"github.com/textileio/textile/v2/cmd"
	"github.com/textileio/textile/v2/util"
)

var (
	// ErrUpToDate indicates there are no locally staged changes.
	ErrUpToDate = errors.New("everything up-to-date")
	// ErrAborted indicates the caller aborted the operation via a confirm function.
	ErrAborted = errors.New("operation aborted by caller")
)

// Event describes a path event that occurred.
// These events can be used to display live info during path uploads/downloads.
type Event struct {
	// Type of event.
	Type EventType
	// Path relative to the bucket's cwd.
	Path string
	// Cid of associated Path.
	Cid cid.Cid
	// Size of the total operation or completed file.
	Size int64
	// Complete is the amount of Size that is complete (useful for upload/download progress).
	Complete int64
}

// EventType is the type of path event.
type EventType int

const (
	// EventProgress indicates a file has made some progress uploading/downloading.
	EventProgress EventType = iota
	// EventFileComplete indicates a file has completed uploading/downloading.
	EventFileComplete
	// EventFileRemoved indicates a file has been removed.
	EventFileRemoved
)

// Bucket is a local-first object storage and synchronization model built
// on ThreadDB, IPFS, and Filecoin.
// A bucket represents a dynamic Unixfs directory with auto-updating
// IPNS resolution, website rendering, and Filecoin archivng.
//
// Private buckets are fully encrypted using AES-CTR + AES-512 HMAC (see https://github.com/textileio/dcrypto for more).
// Both Unixfs node metadata (size, links, etc.) and node data (files) are obfuscated by encryption.
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
	retrID  string

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
		return b.cwd, nil
	}
	return filepath.Dir(filepath.Dir(conf)), nil
}

// LocalSize returns the cumalative size of the bucket's local files.
func (b *Bucket) LocalSize() (int64, error) {
	if b.repo == nil {
		return 0, nil
	}
	bp, err := b.Path()
	if err != nil {
		return 0, err
	}
	var size int64
	err = filepath.Walk(bp, func(n string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("getting fileinfo of %s: %s", n, err)
		}
		if !info.IsDir() {
			f := strings.TrimPrefix(n, bp+string(os.PathSeparator))
			if Ignore(n) || (strings.HasPrefix(f, b.conf.Dir) && f != buckets.SeedName) {
				return nil
			}
			size += info.Size()
		}
		return err
	})
	return size, err
}

// RetrievalID returns the retrieval-id if the bucket creation
// was bootstrapped from a Filecoin archive. It only has a non-empty
// value in this use-case.
func (b *Bucket) RetrievalID() string {
	return b.retrID
}

// Info wraps info about a bucket.
type Info struct {
	Thread    thread.ID           `json:"thread"`
	Key       string              `json:"key"`
	Owner     string              `json:"owner,omitempty"`
	Name      string              `json:"name,omitempty"`
	Version   int                 `json:"version"`
	LinkKey   string              `json:"link_key,omitempty"`
	Path      Path                `json:"path"`
	Metadata  map[string]Metadata `json:"metadata,omitempty"`
	Archives  *Archives           `json:"archives,omitempty"`
	CreatedAt time.Time           `json:"created_at"`
	UpdatedAt time.Time           `json:"updated_at"`
}

// Path wraps path.Resolved so it can be JSON-marshalable.
type Path struct {
	path.Resolved
}

func (p Path) MarshalJSON() ([]byte, error) {
	return []byte("\"" + p.String() + "\""), nil
}

// Metadata wraps metadata about a bucket item.
type Metadata struct {
	Key       string                  `json:"key,omitempty"`
	Roles     map[string]buckets.Role `json:"roles,omitempty"`
	UpdatedAt time.Time               `json:"updated_at"`
}

// Archives contains info about bucket Filecoin archives.
type Archives struct {
	Current Archive   `json:"current"`
	History []Archive `json:"history,omitempty"`
}

// Archive is a single Filecoin archive containing a list of deals.
type Archive struct {
	Cid   string `json:"cid"`
	Deals []Deal `json:"deals"`
}

// Deal contains info about an archive's Filecoin deal.
type Deal struct {
	ProposalCid string `json:"proposal_cid"`
	Miner       string `json:"miner"`
}

// Info returns info about a bucket from the remote.
func (b *Bucket) Info(ctx context.Context) (info Info, err error) {
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

func pbRootToInfo(r *pb.Root) (info Info, err error) {
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
	md, err := pbMetadataToInfo(r.PathMetadata)
	if err != nil {
		return
	}
	return Info{
		Thread:    id,
		Key:       r.Key,
		Owner:     r.Owner,
		Name:      name,
		Version:   int(r.Version),
		LinkKey:   r.LinkKey,
		Path:      Path{pth},
		Metadata:  md,
		Archives:  pbArchivesToInfo(r.Archives),
		CreatedAt: time.Unix(0, r.CreatedAt),
		UpdatedAt: time.Unix(0, r.UpdatedAt),
	}, nil
}

func pbMetadataToInfo(pbm map[string]*pb.Metadata) (map[string]Metadata, error) {
	if pbm == nil {
		return nil, nil
	}
	md := make(map[string]Metadata)
	for p, m := range pbm {
		roles, err := buckets.RolesFromPb(m.Roles)
		if err != nil {
			return nil, err
		}
		md[p] = Metadata{
			Key:       m.Key,
			Roles:     roles,
			UpdatedAt: time.Unix(0, m.UpdatedAt),
		}
	}
	return md, nil
}

func pbArchivesToInfo(pba *pb.Archives) *Archives {
	if pba == nil || pba.Current.Cid == "" {
		return nil
	}
	archives := &Archives{
		Current: Archive{
			Cid:   pba.Current.Cid,
			Deals: pbDealsToInfo(pba.Current.DealInfo),
		},
	}
	if len(pba.History) == 0 {
		return archives
	}
	archives.History = make([]Archive, len(pba.History))
	for i, a := range pba.History {
		archives.History[i] = Archive{
			Cid:   a.Cid,
			Deals: pbDealsToInfo(a.DealInfo),
		}
	}
	return archives
}

func pbDealsToInfo(pbd []*pb.DealInfo) []Deal {
	deals := make([]Deal, len(pbd))
	for i, d := range pbd {
		deals[i] = Deal{
			ProposalCid: d.ProposalCid,
			Miner:       d.Miner,
		}
	}
	return deals
}

// Roots wraps local and remote root cids.
// If the bucket is not private (encrypted), these will be the same.
type Roots struct {
	Local  cid.Cid `json:"local"`
	Remote cid.Cid `json:"remote"`
}

// Roots returns the bucket's current local and remote root cids.
func (b *Bucket) Roots(ctx context.Context) (roots Roots, err error) {
	var lc, rc cid.Cid
	if b.repo != nil {
		lc, rc, err = b.repo.Root()
		if err != nil {
			return
		}
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
func (b *Bucket) RemoteLinks(ctx context.Context, pth string) (links Links, err error) {
	if b.links != nil {
		return *b.links, nil
	}
	ctx, err = b.context(ctx)
	if err != nil {
		return
	}
	res, err := b.clients.Buckets.Links(ctx, b.Key(), pth)
	if err != nil {
		return
	}
	links = Links{URL: res.Url, WWW: res.Www, IPNS: res.Ipns}
	b.links = &links
	return links, err
}

// DBInfo returns info about the bucket's ThreadDB.
// This info can be used to add replicas or additional peers to the bucket.
func (b *Bucket) DBInfo(ctx context.Context) (info db.Info, cc db.CollectionConfig, err error) {
	ctx, err = b.context(ctx)
	if err != nil {
		return
	}
	id, err := b.Thread()
	if err != nil {
		return
	}
	info, err = b.clients.Threads.GetDBInfo(ctx, id)
	if err != nil {
		return
	}
	cc, err = b.clients.Threads.GetCollectionInfo(ctx, id, buckets.CollectionName)
	if err != nil {
		return
	}
	return info, cc, nil
}

// CatRemotePath writes the content of the remote path to writer.
func (b *Bucket) CatRemotePath(ctx context.Context, pth string, w io.Writer) error {
	ctx, err := b.context(ctx)
	if err != nil {
		return err
	}
	return b.clients.Buckets.PullPath(ctx, b.Key(), pth, w)
}

// Destroy completely deletes the local and remote bucket.
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
	if b.repo == nil {
		return nil
	}
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
