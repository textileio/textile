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
	"github.com/textileio/textile/util"
)

const cname = "buckets"

var (
	schema  *jsonschema.Schema
	indexes = []db.IndexConfig{{
		Path: "Path",
	}, {
		Path:   "Name",
		Unique: true,
	}}
)

type Bucket struct {
	ID        string
	Path      string
	Name      string
	CreatedAt int64
	UpdatedAt int64
}

func init() {
	schema = jsonschema.Reflect(&Bucket{})
}

type Buckets struct {
	Threads *dbc.Client
}

func (b *Buckets) Create(ctx context.Context, dbID thread.ID, pth path.Path, name string, opts ...Option) (*Bucket, error) {
	args := &Options{}
	for _, opt := range opts {
		opt(args)
	}

	validName, err := util.ToValidName(name)
	if err != nil {
		return nil, err
	}
	bucket := &Bucket{
		Path:      pth.String(),
		Name:      validName,
		CreatedAt: time.Now().UnixNano(),
		UpdatedAt: time.Now().UnixNano(),
	}
	ids, err := b.Threads.Create(ctx, dbID, cname, dbc.Instances{bucket}, db.WithTxnToken(args.Token))
	if err != nil {
		if isCollNotFoundErr(err) {
			if err := b.addCollection(ctx, dbID, opts...); err != nil {
				return nil, err
			}
			return b.Create(ctx, dbID, pth, name)
		}
	}
	bucket.ID = ids[0]
	return bucket, nil
}

func (b *Buckets) Get(ctx context.Context, dbID thread.ID, name string, opts ...Option) (*Bucket, error) {
	args := &Options{}
	for _, opt := range opts {
		opt(args)
	}

	query := db.Where("Name").Eq(name)
	res, err := b.Threads.Find(ctx, dbID, cname, query, &Bucket{}, db.WithTxnToken(args.Token))
	if err != nil {
		if isCollNotFoundErr(err) {
			if err := b.addCollection(ctx, dbID, opts...); err != nil {
				return nil, err
			}
			return b.Get(ctx, dbID, name)
		}
		return nil, err
	}
	buckets := res.([]*Bucket)
	if len(buckets) == 0 {
		return nil, nil
	}
	return buckets[0], nil
}

func (b *Buckets) List(ctx context.Context, dbID thread.ID, opts ...Option) ([]*Bucket, error) {
	args := &Options{}
	for _, opt := range opts {
		opt(args)
	}

	res, err := b.Threads.Find(ctx, dbID, cname, &db.Query{}, &Bucket{}, db.WithTxnToken(args.Token))
	if err != nil {
		if isCollNotFoundErr(err) {
			if err := b.addCollection(ctx, dbID, opts...); err != nil {
				return nil, err
			}
			return b.List(ctx, dbID)
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
	return b.Threads.Save(ctx, dbID, cname, dbc.Instances{bucket}, db.WithTxnToken(args.Token))
}

func (b *Buckets) Delete(ctx context.Context, dbID thread.ID, id string, opts ...Option) error {
	args := &Options{}
	for _, opt := range opts {
		opt(args)
	}
	return b.Threads.Delete(ctx, dbID, cname, []string{id}, db.WithTxnToken(args.Token))
}

func (b *Buckets) addCollection(ctx context.Context, dbID thread.ID, opts ...Option) error {
	args := &Options{}
	for _, opt := range opts {
		opt(args)
	}
	return b.Threads.NewCollection(ctx, dbID, db.CollectionConfig{
		Name:    cname,
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
