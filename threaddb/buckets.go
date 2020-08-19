package threaddb

import (
	"context"
	"encoding/base64"
	"fmt"
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

var (
	bucketsSchema  *jsonschema.Schema
	bucketsIndexes = []db.Index{{
		Path: "path",
	}}
	bucketsConfig db.CollectionConfig
)

// Bucket represents the buckets threaddb collection schema.
type Bucket struct {
	Key       string   `json:"_id"`
	Name      string   `json:"name"`
	Path      string   `json:"path"`
	EncKey    string   `json:"key,omitempty"`
	DNSRecord string   `json:"dns_record,omitempty"`
	Archives  Archives `json:"archives"`
	CreatedAt int64    `json:"created_at"`
	UpdatedAt int64    `json:"updated_at"`
}

// GetEncKey returns the encryption key as bytes if present.
func (b *Bucket) GetEncKey() []byte {
	if b.EncKey == "" {
		return nil
	}
	key, _ := base64.StdEncoding.DecodeString(b.EncKey)
	return key
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
func (b *Buckets) New(ctx context.Context, dbID thread.ID, key string, pth path.Path, opts ...BucketOption) (*Bucket, error) {
	args := &BucketOptions{}
	for _, opt := range opts {
		opt(args)
	}
	var encKey string
	if args.Key != nil {
		encKey = base64.StdEncoding.EncodeToString(args.Key)
	}
	now := time.Now().UnixNano()
	bucket := &Bucket{
		Key:       key,
		Name:      args.Name,
		Path:      pth.String(),
		EncKey:    encKey,
		Archives:  Archives{Current: Archive{Deals: []Deal{}}, History: []Archive{}},
		CreatedAt: now,
		UpdatedAt: now,
	}
	id, err := b.Create(ctx, dbID, bucket, WithToken(args.Token))
	if err != nil {
		return nil, fmt.Errorf("creating bucket in thread: %s", err)
	}
	bucket.Key = string(id)

	if err := b.baCol.Create(ctx, key); err != nil {
		return nil, fmt.Errorf("saving BucketArchive data: %s", err)
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
	ba, err := b.baCol.Get(ctx, key)
	if err != nil {
		return ffs.Failed, "", fmt.Errorf("getting ffs instance data: %s", err)
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
	ba, err := b.baCol.Get(ctx, key)
	if err != nil {
		return fmt.Errorf("getting ffs instance data: %s", err)
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
