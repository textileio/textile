package buckets

import (
	"context"
	"encoding/json"
	"time"

	"github.com/alecthomas/jsonschema"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/textileio/go-threads/api/client"
	"github.com/textileio/go-threads/core/thread"
	s "github.com/textileio/go-threads/store"
	"github.com/textileio/textile/util"
)

const (
	cname = "buckets"
)

var (
	schema  []byte
	indexes = []*s.IndexConfig{{
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
	var err error
	schema, err = json.Marshal(jsonschema.Reflect(&Bucket{}))
	if err != nil {
		panic(err)
	}
}

type Buckets struct {
	threads *client.Client
}

func (b *Buckets) Create(ctx context.Context, storeID thread.ID, pth path.Path, name string) (*Bucket, error) {
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
	if err := b.threads.ModelCreate(ctx, storeID.String(), cname, bucket); err != nil {
		// @todo: check error type
		if err = b.threads.RegisterSchema(ctx, storeID.String(), cname, string(schema), indexes...); err != nil {
			return nil, err
		}
	}
	return bucket, nil
}

func (b *Buckets) Get(ctx context.Context, storeID thread.ID, name string) (*Bucket, error) {
	query := s.JSONWhere("Name").Eq(name)
	res, err := b.threads.ModelFind(ctx, storeID.String(), cname, query, []*Bucket{})
	if err != nil {
		return nil, err
	}
	buckets := res.([]*Bucket)
	if len(buckets) == 0 {
		return nil, nil
	}
	return buckets[0], nil
}

func (b *Buckets) List(ctx context.Context, storeID thread.ID) ([]*Bucket, error) {
	res, err := b.threads.ModelFind(ctx, storeID.String(), cname, &s.JSONQuery{}, []*Bucket{})
	if err != nil {
		return nil, err
	}
	return res.([]*Bucket), nil
}

func (b *Buckets) Save(ctx context.Context, storeID thread.ID, bucket *Bucket) error {
	return b.threads.ModelSave(ctx, storeID.String(), cname, bucket)
}

func (b *Buckets) Delete(ctx context.Context, storeID thread.ID, id string) error {
	return b.threads.ModelDelete(ctx, storeID.String(), cname, id)
}
