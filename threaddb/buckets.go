package threaddb

import (
	"context"
	"encoding/base64"
	"fmt"
	gopath "path"
	"strings"
	"sync"
	"time"

	"github.com/alecthomas/jsonschema"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/textileio/dcrypto"
	dbc "github.com/textileio/go-threads/api/client"
	"github.com/textileio/go-threads/core/thread"
	db "github.com/textileio/go-threads/db"
	powc "github.com/textileio/powergate/v2/api/client"
	userPb "github.com/textileio/powergate/v2/api/gen/powergate/user/v1"
	"github.com/textileio/textile/v2/buckets"
	mdb "github.com/textileio/textile/v2/mongodb"
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

// SetFileEncryptionKey sets the file encryption key.
func (m *Metadata) SetFileEncryptionKey(key []byte) {
	if key != nil {
		m.Key = base64.StdEncoding.EncodeToString(key)
	}
}

// Archives contains info about bucket Filecoin archives.
type Archives struct {
	Current Archive   `json:"current"`
	History []Archive `json:"history"`
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
	}

	md, _, ok := b.GetMetadataForPath(pth, true)
	if !ok {
		return nil, fmt.Errorf("could not resolve path: %s", pth)
	}
	return keyBytes(md.Key), nil
}

func keyBytes(k string) []byte {
	if k == "" {
		return nil
	}
	b, _ := base64.StdEncoding.DecodeString(k)
	return b
}

// GetFileEncryptionKeysForPrefix returns a map of keys for every path under prefix.
func (b *Bucket) GetFileEncryptionKeysForPrefix(pre string) (map[string][]byte, error) {
	if b.Version == 0 {
		return map[string][]byte{"": b.GetLinkEncryptionKey()}, nil
	}

	keys := make(map[string][]byte)
	for p := range b.Metadata {
		if strings.HasPrefix(p, pre) {
			md, _, ok := b.GetMetadataForPath(p, true)
			if !ok {
				return nil, fmt.Errorf("could not resolve path: %s", p)
			}
			keys[p] = keyBytes(md.Key)
		}
	}
	return keys, nil
}

// RotateFileEncryptionKeysForPrefix re-generates existing metadata keys for every path under prefix.
func (b *Bucket) RotateFileEncryptionKeysForPrefix(pre string) error {
	if b.Version == 0 {
		return nil
	}

	for p, md := range b.Metadata {
		if strings.HasPrefix(p, pre) {
			if md.Key != "" {
				key, err := dcrypto.NewKey()
				if err != nil {
					return err
				}
				md.SetFileEncryptionKey(key)
			}
		}
	}
	return nil
}

// GetMetadataForPath returns metadata for path.
// The returned metadata could be from an exact path match or
// the nearest parent, i.e., path was added as part of a folder.
// If requireKey is true, metadata w/o a key is ignored and the search continues toward root.
func (b *Bucket) GetMetadataForPath(pth string, requireKey bool) (md Metadata, at string, ok bool) {
	if b.Version == 0 {
		return md, at, true
	}

	// Check for an exact match
	if md, ok = b.Metadata[pth]; ok {
		if !b.IsPrivate() || !requireKey || md.Key != "" {
			return md, pth, true
		}
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
			if !b.IsPrivate() || !requireKey || md.Key != "" {
				return md, parent, true
			}
		}
	}
	return md, at, false
}

// SetMetadataAtPath create new or merges existing metadata at path.
func (b *Bucket) SetMetadataAtPath(pth string, md Metadata) {
	if b.Version == 0 {
		return
	}

	x, ok := b.Metadata[pth]
	if ok {
		if md.Key != "" {
			x.Key = md.Key
		}
		if md.Roles != nil {
			x.Roles = md.Roles
		}
		x.UpdatedAt = md.UpdatedAt
		b.Metadata[pth] = x
	} else {
		if md.Roles == nil {
			md.Roles = make(map[string]buckets.Role)
		}
		b.Metadata[pth] = md
	}
}

// UnsetMetadataWithPrefix removes metadata with the path prefix.
func (b *Bucket) UnsetMetadataWithPrefix(pre string) {
	if b.Version == 0 {
		return
	}

	for p := range b.Metadata {
		if strings.HasPrefix(p, pre) {
			delete(b.Metadata, p)
		}
	}
}

// ensureNoNulls inflates any values that are nil due to schema updates.
func (b *Bucket) ensureNoNulls() {
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

func (b *Bucket) Copy() *Bucket {
	md := make(map[string]Metadata)
	for k, v := range b.Metadata {
		md[k] = v
	}
	ar := Archives{
		Current: copyArchives(b.Archives.Current),
		History: make([]Archive, len(b.Archives.History)),
	}
	for i, j := range b.Archives.History {
		ar.History[i] = copyArchives(j)
	}
	return &Bucket{
		Key: b.Key,
		Owner: b.Owner,
		Name: b.Name,
		Version: b.Version,
		LinkKey: b.LinkKey,
		Path: b.Path,
		Metadata: md,
		Archives: ar,
		CreatedAt: b.CreatedAt,
		UpdatedAt: b.UpdatedAt,
	}
}

func copyArchives(archive Archive) Archive {
	a := Archive{
		Cid: archive.Cid,
		Deals: make([]Deal, len(archive.Deals)),
	}
	for i, j := range archive.Deals {
		a.Deals[i] = j
	}
	return a
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
			      if (patch.path && writer !== instance.owner) {
			        return "permission denied"
			      } else {
			        patch.metadata = {}
			      }
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
			        // merge all parents, taking most privileged role
			        if (keys[i].length > 0) {
			          var parts = keys[i].split("/")
			          parts.unshift("")
			          var path = ""
			          for (j = 0; j < parts.length; j++) {
			            if (path.length > 0) {
			              path += "/"
			            }
			            path += parts[j]
			            var y = instance.metadata[path]
			            if (!y) {
			              continue
			            }
			            if (!y.roles[writer]) {
			              y.roles[writer] = 0
			            }
			            if (!y.roles["*"]) {
			              y.roles["*"] = 0
			            }
			            if (y.roles[writer] > x.roles[writer]) {
			              x.roles[writer] = y.roles[writer]
			            }
			            if (y.roles["*"] > x.roles["*"]) {
			              x.roles["*"] = y.roles["*"]
			            }
			          }
			        }
			        // check access against merged roles
			        if (!p) {
			          if (x.roles[writer] < 3) {
			            return "permission denied" // no admin access to delete items
			          }
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
			outer: for (i = 0; i < keys.length; i++) {
			  var m = instance.metadata[keys[i]]
			  var parts = keys[i].split("/")
			  if (keys[i].length > 0) {
			    parts.unshift("")
			  }
			  var path = ""
			  for (j = 0; j < parts.length; j++) {
			    if (path.length > 0) {
			      path += "/"
			    }
			    path += parts[j]
			    var x = instance.metadata[path]
			    if (x && (x.roles[reader] > 0 || x.roles["*"] > 0)) {
			      filtered[keys[i]] = m
			      continue outer
			    }
			  }
			}
			instance.metadata = filtered
			if (Object.keys(instance.metadata).length === 0) {
			  delete instance.key
			}
			return instance
		`,
	}
}

// Buckets is a wrapper around a threaddb collection that performs object storage on IPFS and Filecoin.
type Buckets struct {
	Collection

	baCol    *mdb.BucketArchives
	pgClient *powc.Client

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
func (b *Buckets) New(
	ctx context.Context,
	dbID thread.ID,
	key string,
	pth path.Path,
	now time.Time,
	owner thread.PubKey,
	metadata map[string]Metadata, opts ...BucketOption,
) (*Bucket, error) {
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
	if _, err := b.Create(ctx, dbID, bucket, WithToken(args.Token)); err != nil {
		return nil, fmt.Errorf("creating bucket in thread: %s", err)
	}
	if _, err := b.baCol.Create(ctx, key); err != nil {
		return nil, fmt.Errorf("creating BucketArchive data: %s", err)
	}
	if err := b.Get(ctx, dbID, key, &bucket, WithToken(args.Token)); err != nil {
		return nil, fmt.Errorf("getting bucket in thread: %s", err)
	}
	return bucket, nil
}

// GetSafe gets a bucket instance and inflates any values that are nil due to schema updates.
func (b *Buckets) GetSafe(ctx context.Context, dbID thread.ID, key string, buck *Bucket, opts ...Option) error {
	if err := b.Get(ctx, dbID, key, buck, opts...); err != nil {
		return err
	}
	buck.ensureNoNulls()
	return nil
}

// IsArchivingEnabled returns whether or not Powergate archiving is enabled.
func (b *Buckets) IsArchivingEnabled() bool {
	return b.pgClient != nil
}

// ArchiveStatus returns the last known archive status on Powergate. If the return status is Failed,
// an extra string with the error message is provided.
func (b *Buckets) ArchiveStatus(ctx context.Context, key string) (userPb.JobStatus, string, error) {
	ba, err := b.baCol.GetOrCreate(ctx, key)
	if err != nil {
		return userPb.JobStatus_JOB_STATUS_UNSPECIFIED, "",
			fmt.Errorf("getting BucketArchive data: %s", err)
	}

	if ba.Archives.Current.JobID == "" {
		return userPb.JobStatus_JOB_STATUS_UNSPECIFIED, "", buckets.ErrNoCurrentArchive
	}
	current := ba.Archives.Current
	if current.Aborted {
		return userPb.JobStatus_JOB_STATUS_UNSPECIFIED, "",
			fmt.Errorf("job status tracking was aborted: %s", current.AbortedMsg)
	}
	if statusName, found := userPb.JobStatus_name[int32(current.Status)]; !found {
		return userPb.JobStatus_JOB_STATUS_UNSPECIFIED, "",
			fmt.Errorf("unknown powergate job status: %s", statusName)
	}
	return userPb.JobStatus(current.Status), current.FailureMsg, nil
}

// ArchiveWatch allows to have the last log execution for the last archive, plus realtime
// human-friendly log output of how the current archive is executing.
// If the last archive is already done, it will simply return the log history and close the channel.
func (b *Buckets) ArchiveWatch(ctx context.Context, key string, powToken string, ch chan<- string) error {
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
	ctx = context.WithValue(ctx, powc.AuthKey, powToken)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	logsCh := make(chan powc.WatchLogsEvent)
	if err := b.pgClient.Data.WatchLogs(
		ctx,
		logsCh,
		c.String(),
		powc.WithJobIDFilter(current.JobID),
		powc.WithHistory(true),
	); err != nil {
		return fmt.Errorf("watching log events in Powergate: %s", err)
	}
	for le := range logsCh {
		if le.Err != nil {
			return le.Err
		}
		ch <- le.Res.LogEntry.Message
	}
	return nil
}

func (b *Buckets) Close() error {
	b.cancel()
	b.wg.Wait()
	return nil
}
