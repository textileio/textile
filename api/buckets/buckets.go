package buckets

import (
	"context"
	"fmt"
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
		Path:   "name",
		Unique: true,
	}, {
		Path:   "slug",
		Unique: true,
	}, {
		Path: "path",
	}}
)

type Bucket struct {
	ID        string `json:"_id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
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

func (b *Buckets) Create(ctx context.Context, dbID thread.ID, pth path.Path, name string, opts ...Option) (*Bucket, error) {
	args := &Options{}
	for _, opt := range opts {
		opt(args)
	}

	slg, ok := util.ToValidName(name)
	if !ok {
		return nil, fmt.Errorf("name '%s' is not available", name)
	}
	now := time.Now().UnixNano()
	bucket := &Bucket{
		Name:      name,
		Slug:      strings.Join([]string{slg, dbID.String()}, "-"),
		Path:      pth.String(),
		CreatedAt: now,
		UpdatedAt: now,
	}
	ids, err := b.Threads.Create(ctx, dbID, cname, dbc.Instances{bucket}, db.WithTxnToken(args.Token))
	if err != nil {
		if isCollNotFoundErr(err) {
			if err := b.addCollection(ctx, dbID, opts...); err != nil {
				return nil, err
			}
			return b.Create(ctx, dbID, pth, name)
		}
		return nil, err
	}
	bucket.ID = ids[0]
	return bucket, nil
}

func (b *Buckets) Get(ctx context.Context, dbID thread.ID, name string, opts ...Option) (*Bucket, error) {
	args := &Options{}
	for _, opt := range opts {
		opt(args)
	}

	query := db.Where("name").Eq(name)
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
