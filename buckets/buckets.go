package buckets

import (
	"context"
	"strings"
	"time"

	"github.com/alecthomas/jsonschema"
	"github.com/ipfs/interface-go-ipfs-core/path"
	dbc "github.com/textileio/go-threads/api/client"
	"github.com/textileio/go-threads/core/thread"
	db "github.com/textileio/go-threads/db"
)

const (
	CollectionName = "buckets"
	SeedName       = ".textilebucketseed"
)

var (
	schema  *jsonschema.Schema
	indexes = []db.IndexConfig{{
		Path: "path",
	}}
)

type Bucket struct {
	Key       string `json:"_id"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	DNSRecord string `json:"dns_record,omitempty"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

func init() {
	schema = jsonschema.Reflect(&Bucket{})
}

type Buckets struct {
	Threads *dbc.Client
}

func (b *Buckets) Create(ctx context.Context, dbID thread.ID, key, name string, pth path.Path, opts ...Option) (*Bucket, error) {
	args := &Options{}
	for _, opt := range opts {
		opt(args)
	}
	now := time.Now().UnixNano()
	bucket := &Bucket{
		Key:       key,
		Name:      name,
		Path:      pth.String(),
		CreatedAt: now,
		UpdatedAt: now,
	}
	ids, err := b.Threads.Create(ctx, dbID, CollectionName, dbc.Instances{bucket}, db.WithTxnToken(args.Token))
	if err != nil {
		if isCollNotFoundErr(err) {
			if err := b.addCollection(ctx, dbID, opts...); err != nil {
				return nil, err
			}
			return b.Create(ctx, dbID, key, name, pth, opts...)
		}
		return nil, err
	}
	bucket.Key = ids[0]
	return bucket, nil
}

func (b *Buckets) Get(ctx context.Context, dbID thread.ID, key string, opts ...Option) (*Bucket, error) {
	args := &Options{}
	for _, opt := range opts {
		opt(args)
	}
	buck := &Bucket{}
	if err := b.Threads.FindByID(ctx, dbID, CollectionName, key, buck, db.WithTxnToken(args.Token)); err != nil {
		if isCollNotFoundErr(err) {
			if err := b.addCollection(ctx, dbID, opts...); err != nil {
				return nil, err
			}
			return b.Get(ctx, dbID, key, opts...)
		}
		return nil, err
	}
	return buck, nil
}

func (b *Buckets) List(ctx context.Context, dbID thread.ID, opts ...Option) ([]*Bucket, error) {
	args := &Options{}
	for _, opt := range opts {
		opt(args)
	}
	res, err := b.Threads.Find(ctx, dbID, CollectionName, &db.Query{}, &Bucket{}, db.WithTxnToken(args.Token))
	if err != nil {
		if isCollNotFoundErr(err) {
			if err := b.addCollection(ctx, dbID, opts...); err != nil {
				return nil, err
			}
			return b.List(ctx, dbID, opts...)
		}
		return nil, err
	}
	return res.([]*Bucket), nil
}

func (b *Buckets) Save(ctx context.Context, dbID thread.ID, bucket *Bucket, opts ...Option) error {
	args := &Options{}
	for _, opt := range opts {
		opt(args)
	}
	return b.Threads.Save(ctx, dbID, CollectionName, dbc.Instances{bucket}, db.WithTxnToken(args.Token))
}

func (b *Buckets) Delete(ctx context.Context, dbID thread.ID, key string, opts ...Option) error {
	args := &Options{}
	for _, opt := range opts {
		opt(args)
	}
	return b.Threads.Delete(ctx, dbID, CollectionName, []string{key}, db.WithTxnToken(args.Token))
}

func (b *Buckets) addCollection(ctx context.Context, dbID thread.ID, opts ...Option) error {
	args := &Options{}
	for _, opt := range opts {
		opt(args)
	}
	return b.Threads.NewCollection(ctx, dbID, db.CollectionConfig{
		Name:    CollectionName,
		Schema:  schema,
		Indexes: indexes,
	}, db.WithManagedDBToken(args.Token))
}

type Options struct {
	Token thread.Token
}

type Option func(*Options)

func WithToken(t thread.Token) Option {
	return func(args *Options) {
		args.Token = t
	}
}

func isCollNotFoundErr(err error) bool {
	return strings.Contains(err.Error(), "collection not found")
}
