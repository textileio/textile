package threaddb

import (
	"context"
	"encoding/base64"
	"fmt"
	gopath "path"
	"sync"
	"time"

	"github.com/alecthomas/jsonschema"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/interface-go-ipfs-core/path"
	dbc "github.com/textileio/go-threads/api/client"
	"github.com/textileio/go-threads/core/thread"
	db "github.com/textileio/go-threads/db"
	powc "github.com/textileio/powergate/api/client"
	"github.com/textileio/powergate/ffs"
	"github.com/textileio/textile/buckets"
	mdb "github.com/textileio/textile/mongodb"
)

const Version = 1

var (
	bucketsSchema  *jsonschema.Schema
	bucketsIndexes = []db.Index{{
		Path: "path",
	}}
	bucketsConfig db.CollectionConfig
)

// Bucket represents the buckets threaddb collection schema.
type Bucket struct {
	Key       string              `json:"_id"`
	Owner     string              `json:"owner"`
	Name      string              `json:"name"`
	Version   int                 `json:"version"`
	LinkKey   string              `json:"key,omitempty"`
	Path      string              `json:"path"`
	Metadata  map[string]Metadata `json:"metadata"`
	Archives  Archives            `json:"archives"`
	CreatedAt int64               `json:"created_at"`
	UpdatedAt int64               `json:"updated_at"`
}

// Metadata contains metadata about a bucket item (a file or folder).
type Metadata struct {
	Key       string                  `json:"key,omitempty"`
	Roles     map[string]buckets.Role `json:"roles"`
	UpdatedAt int64                   `json:"updated_at"`
}

// NewDefaultMetadata returns the default metadata for a path.
func NewDefaultMetadata(owner thread.PubKey, key []byte, ts time.Time) Metadata {
	roles := make(map[string]buckets.Role)
	if owner != nil {
		if key == nil {
			roles["*"] = buckets.Reader
		}
		roles[owner.String()] = buckets.Admin
	}
	md := Metadata{
		Roles:     roles,
		UpdatedAt: ts.UnixNano(),
	}
	md.SetFileEncryptionKey(key)
	return md
}

// GetFileEncryptionKey returns the file encryption key as bytes if present.
func (m *Metadata) GetFileEncryptionKey() []byte {
	return keyBytes(m.Key)
}

// SetFileEncryptionKey sets the file encryption key.
func (m *Metadata) SetFileEncryptionKey(key []byte) {
	if key != nil {
		m.Key = base64.StdEncoding.EncodeToString(key)
	}
}

// Archives contains all archives for a single bucket.
type Archives struct {
	Current Archive   `json:"current"`
	History []Archive `json:"history"`
}

// Archive is a single archive containing a list of deals.
type Archive struct {
	Cid   string `json:"cid"`
	Deals []Deal `json:"deals"`
}

// Deal contains details about a Filecoin deal.
type Deal struct {
	ProposalCid string `json:"proposal_cid"`
	Miner       string `json:"miner"`
}

// IsPrivate returns whether or not the bucket is private.
func (b *Bucket) IsPrivate() bool {
	return b.LinkKey != ""
}

// GetLinkEncryptionKey returns the bucket encryption key as bytes if present.
// Version 0 buckets use the link key for all files and folders.
// Version 1 buckets only use the link for folders.
func (b *Bucket) GetLinkEncryptionKey() []byte {
	return keyBytes(b.LinkKey)
}

// GetFileEncryptionKeyForPath returns the encryption key for path.
// Version 0 buckets use the link key for all paths.
// Version 1 buckets use a different key defined in path metadata.
func (b *Bucket) GetFileEncryptionKeyForPath(pth string) ([]byte, error) {
	if b.Version == 0 {
		return b.GetLinkEncryptionKey(), nil
	} else {
		md, _, ok := b.GetMetadataForPath(pth)
		if !ok {
			return nil, fmt.Errorf("could not resolve path: %s", pth)
		}
		return keyBytes(md.Key), nil
	}
}

func keyBytes(k string) []byte {
	if k == "" {
		return nil
	}
	b, _ := base64.StdEncoding.DecodeString(k)
	return b
}

// GetMetadataForPath returns metadata for path.
// The returned metadata could be from an exact path match or
// the nearest parent, i.e., path was added as part of a folder.
func (b *Bucket) GetMetadataForPath(pth string) (md Metadata, at string, ok bool) {
	if b.Version == 0 {
		return md, at, true
	}
	// Check for an exact match
	if md, ok = b.Metadata[pth]; ok {
		return md, pth, true
	}
	// Check if we can see this path via a parent
	parent := pth
	var done bool
	for {
		if done {
			break
		}
		parent = gopath.Dir(parent)
		if parent == "." {
			parent = ""
			done = true
		}
		if md, ok = b.Metadata[parent]; ok {
			return md, parent, true
		}
	}
	return md, at, false
}

// BucketOptions defines options for interacting with buckets.
type BucketOptions struct {
	Name  string
	Key   []byte
	Token thread.Token
}

// BucketOption holds a bucket option.
type BucketOption func(*BucketOptions)

// WithNewBucketName specifies a name for a bucket.
// Note: This is only valid when creating a new bucket.
func WithNewBucketName(n string) BucketOption {
	return func(args *BucketOptions) {
		args.Name = n
	}
}

// WithNewBucketKey sets the bucket encryption key.
func WithNewBucketKey(k []byte) BucketOption {
	return func(args *BucketOptions) {
		args.Key = k
	}
}

// WithNewBucketToken sets the threaddb token.
func WithNewBucketToken(t thread.Token) BucketOption {
	return func(args *BucketOptions) {
		args.Token = t
	}
}

func init() {
	reflector := jsonschema.Reflector{ExpandedStruct: true}
	bucketsSchema = reflector.Reflect(&Bucket{})
	bucketsConfig = db.CollectionConfig{
		Name:    buckets.CollectionName,
		Schema:  bucketsSchema,
		Indexes: bucketsIndexes,
		WriteValidator: `
			if (instance && !instance.version) {
			  return true
			}
			var type = event.patch.type
			var patch = event.patch.json_patch
			var restricted = ["owner", "name", "version", "key", "archives", "created_at"]
			switch (type) {
			  case "create":
			    if (patch.owner !== "" && writer !== patch.owner) {
			      return "permission denied" // writer must match new bucket owner
			    }
			    break
			  case "save":
			    if (instance.owner === "") {
			      return true
			    }
			    if (writer !== instance.owner) {
			      for (i = 0; i < restricted.length; i++) {
			        if (patch[restricted[i]]) {
			          return "permission denied"
			        }
			      }
			    }
			    if (!patch.metadata) {
			      return true
			    }
			    var keys = Object.keys(patch.metadata)
			    for (i = 0; i < keys.length; i++) {
			      var p = patch.metadata[keys[i]]
			      var x = instance.metadata[keys[i]]
			      if (x) {
			        if (!x.roles[writer]) {
			          x.roles[writer] = 0
			        }
			        if (!x.roles["*"]) {
			          x.roles["*"] = 0
			        }
			        if (!p) {
                      if (x.roles[writer] < 3) {
			            patch.metadata[keys[i]] = x // reset patch, no admin access to delete items
			          }
			          continue
			        } else {
			          if (p.roles && x.roles[writer] < 3) {
			            return "permission denied" // no admin access to edit roles
			          }
			        }
			        if (x.roles[writer] < 2 && x.roles["*"] < 2) {
			          return "permission denied" // no write access
			        }
			      } else {
			        if (writer !== instance.owner) {
			          return "permission denied" // no owner access to create items
			        }
			      }
			    }
			    break
			  case "delete":
			    if (instance.owner !== "" && writer !== instance.owner) {
			      return "permission denied" // no owner access to delete instance
			    }
			    break
			}
			return true
		`,
		ReadFilter: `
			if (!instance.version) {
			  return instance
			}
			if (instance.owner === "") {
			  return instance
			}
			var filtered = {}
			var keys = Object.keys(instance.metadata)
			for (i = 0; i < keys.length; i++) {
			  var x = instance.metadata[keys[i]]
			  if (x.roles[reader] > 0 || x.roles["*"] > 0) {
			    filtered[keys[i]] = x
			  }
			}
			instance.metadata = filtered
			return instance
		`,
	}
}

// Buckets is a wrapper around a threaddb collection that performs object storage on IPFS and Filecoin.
type Buckets struct {
	Collection

	baCol    *mdb.BucketArchives
	pgClient *powc.Client

	lock   sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewBuckets returns a new buckets collection mananger.
func NewBuckets(tc *dbc.Client, pgc *powc.Client, col *mdb.BucketArchives) (*Buckets, error) {
	ctx, cancel := context.WithCancel(context.Background())
	return &Buckets{
		Collection: Collection{
			c:      tc,
			config: bucketsConfig,
		},
		baCol:    col,
		pgClient: pgc,

		ctx:    ctx,
		cancel: cancel,
	}, nil
}

// Create a bucket instance.
func (b *Buckets) New(ctx context.Context, dbID thread.ID, key string, pth path.Path, now time.Time, owner thread.PubKey, metadata map[string]Metadata, opts ...BucketOption) (*Bucket, error) {
	args := &BucketOptions{}
	for _, opt := range opts {
		opt(args)
	}
	var encKey string
	if args.Key != nil {
		encKey = base64.StdEncoding.EncodeToString(args.Key)
	}
	if args.Token.Defined() {
		tokenOwner, err := args.Token.PubKey()
		if err != nil {
			return nil, err
		}
		if tokenOwner != nil && owner.String() != tokenOwner.String() {
			return nil, fmt.Errorf("creating bucket: token owner mismatch")
		}
	}
	if metadata == nil {
		metadata = make(map[string]Metadata)
	}
	bucket := &Bucket{
		Key:       key,
		Name:      args.Name,
		Version:   Version,
		LinkKey:   encKey,
		Path:      pth.String(),
		Metadata:  metadata,
		Archives:  Archives{Current: Archive{Deals: []Deal{}}, History: []Archive{}},
		CreatedAt: now.UnixNano(),
		UpdatedAt: now.UnixNano(),
	}
	if owner != nil {
		bucket.Owner = owner.String()
	}
	id, err := b.Create(ctx, dbID, bucket, WithToken(args.Token))
	if err != nil {
		return nil, fmt.Errorf("creating bucket in thread: %s", err)
	}
	bucket.Key = string(id)

	if _, err := b.baCol.Create(ctx, key); err != nil {
		return nil, fmt.Errorf("creating BucketArchive data: %s", err)
	}
	return bucket, nil
}

// IsArchivingEnabled returns whether or not Powergate archiving is enabled.
func (b *Buckets) IsArchivingEnabled() bool {
	return b.pgClient != nil
}

// SaveSafe a bucket instance.
func (b *Buckets) SaveSafe(ctx context.Context, dbID thread.ID, bucket *Bucket, opts ...Option) error {
	ensureNoNulls(bucket)
	return b.Save(ctx, dbID, bucket, opts...)
}

func ensureNoNulls(b *Bucket) {
	if b.Metadata == nil {
		b.Metadata = make(map[string]Metadata)
	}
	if len(b.Archives.History) == 0 {
		current := b.Archives.Current
		if len(current.Deals) == 0 {
			b.Archives.Current = Archive{Deals: []Deal{}}
		}
		b.Archives = Archives{Current: current, History: []Archive{}}
	}
}

// ArchiveStatus returns the last known archive status on Powergate. If the return status is Failed,
// an extra string with the error message is provided.
func (b *Buckets) ArchiveStatus(ctx context.Context, key string) (ffs.JobStatus, string, error) {
	ba, err := b.baCol.GetOrCreate(ctx, key)
	if err != nil {
		return ffs.Failed, "", fmt.Errorf("getting BucketArchive data: %s", err)
	}

	if ba.Archives.Current.JobID == "" {
		return ffs.Failed, "", buckets.ErrNoCurrentArchive
	}
	current := ba.Archives.Current
	if current.Aborted {
		return ffs.Failed, "", fmt.Errorf("job status tracking was aborted: %s", current.AbortedMsg)
	}
	return ffs.JobStatus(current.JobStatus), current.FailureMsg, nil
}

// ArchiveWatch allows to have the last log execution for the last archive, plus realtime
// human-friendly log output of how the current archive is executing.
// If the last archive is already done, it will simply return the log history and close the channel.
func (b *Buckets) ArchiveWatch(ctx context.Context, key string, ffsToken string, ch chan<- string) error {
	ba, err := b.baCol.GetOrCreate(ctx, key)
	if err != nil {
		return fmt.Errorf("getting BucketArchive data: %s", err)
	}

	if ba.Archives.Current.JobID == "" {
		return buckets.ErrNoCurrentArchive
	}
	current := ba.Archives.Current
	if current.Aborted {
		return fmt.Errorf("job status tracking was aborted: %s", current.AbortedMsg)
	}
	c, err := cid.Cast(current.Cid)
	if err != nil {
		return fmt.Errorf("parsing current archive cid: %s", err)
	}
	ctx = context.WithValue(ctx, powc.AuthKey, ffsToken)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ffsCh := make(chan powc.LogEvent)
	if err := b.pgClient.FFS.WatchLogs(ctx, ffsCh, c, powc.WithJidFilter(ffs.JobID(current.JobID)), powc.WithHistory(true)); err != nil {
		return fmt.Errorf("watching log events in Powergate: %s", err)
	}
	for le := range ffsCh {
		if le.Err != nil {
			return le.Err
		}
		ch <- le.LogEntry.Msg
	}
	return nil
}

func (b *Buckets) Close() error {
	b.cancel()
	b.wg.Wait()
	return nil
}
