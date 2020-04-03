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
	DB *dbc.Client
}

func (b *Buckets) Create(ctx context.Context, dbID thread.ID, pth path.Path, name string) (*Bucket, error) {
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
	ids, err := b.DB.Create(ctx, dbID, cname, dbc.Instances{bucket})
	if err != nil {
		if isCollNotFoundErr(err) {
			if err := b.addCollection(ctx, dbID); err != nil {
				return nil, err
			}
			return b.Create(ctx, dbID, pth, name)
		}
	}
	bucket.ID = ids[0]
	return bucket, nil
}

func (b *Buckets) Get(ctx context.Context, dbID thread.ID, name string) (*Bucket, error) {
	query := db.Where("Name").Eq(name)
	res, err := b.DB.Find(ctx, dbID, cname, query, []*Bucket{})
	if err != nil {
		if isCollNotFoundErr(err) {
			if err := b.addCollection(ctx, dbID); err != nil {
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

func (b *Buckets) List(ctx context.Context, dbID thread.ID) ([]*Bucket, error) {
	res, err := b.DB.Find(ctx, dbID, cname, &db.Query{}, []*Bucket{})
	if err != nil {
		if isCollNotFoundErr(err) {
			if err := b.addCollection(ctx, dbID); err != nil {
				return nil, err
			}
			return b.List(ctx, dbID)
		}
		return nil, err
	}
	return res.([]*Bucket), nil
}

func (b *Buckets) Save(ctx context.Context, dbID thread.ID, bucket *Bucket) error {
	return b.DB.Save(ctx, dbID, cname, dbc.Instances{bucket})
}

func (b *Buckets) Delete(ctx context.Context, dbID thread.ID, id string) error {
	return b.DB.Delete(ctx, dbID, cname, []string{id})
}

func (b *Buckets) addCollection(ctx context.Context, dbID thread.ID) error {
	return b.DB.NewCollection(ctx, dbID, db.CollectionConfig{
		Name:    cname,
		Schema:  schema,
		Indexes: indexes,
	})
}

func isCollNotFoundErr(err error) bool {
	return strings.Contains(err.Error(), "collection not found")
}
