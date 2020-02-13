package collections

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/textileio/go-threads/api/client"
	s "github.com/textileio/go-threads/store"
)

type Bucket struct {
	ID        string
	Path      string
	Name      string
	ProjectID string
	Created   int64
	Updated   int64
}

type Buckets struct {
	threads *client.Client
	storeID *uuid.UUID
	token   string
}

func (f *Buckets) GetName() string {
	return "Bucket"
}

func (f *Buckets) GetInstance() interface{} {
	return &Bucket{}
}

func (f *Buckets) GetIndexes() []*s.IndexConfig {
	return []*s.IndexConfig{{
		Path: "Path",
	}, {
		Path:   "Name",
		Unique: true,
	}, {
		Path: "ProjectID",
	}}
}

func (f *Buckets) GetStoreID() *uuid.UUID {
	return f.storeID
}

func (f *Buckets) Create(
	ctx context.Context,
	pth path.Resolved,
	name string,
	projectID string,
) (*Bucket, error) {
	ctx = AuthCtx(ctx, f.token)
	bucket := &Bucket{
		Path:      pth.String(),
		Name:      name,
		ProjectID: projectID,
		Created:   time.Now().Unix(),
		Updated:   time.Now().Unix(),
	}
	if err := f.threads.ModelCreate(ctx, f.storeID.String(), f.GetName(), bucket); err != nil {
		return nil, err
	}
	return bucket, nil
}

func (f *Buckets) GetByName(ctx context.Context, name string) (*Bucket, error) {
	ctx = AuthCtx(ctx, f.token)
	query := s.JSONWhere("Name").Eq(name)
	res, err := f.threads.ModelFind(ctx, f.storeID.String(), f.GetName(), query, []*Bucket{})
	if err != nil {
		return nil, err
	}
	buckets := res.([]*Bucket)
	if len(buckets) == 0 {
		return nil, nil
	}
	return buckets[0], nil
}

func (f *Buckets) List(ctx context.Context, projectID string) ([]*Bucket, error) {
	ctx = AuthCtx(ctx, f.token)
	query := s.JSONWhere("ProjectID").Eq(projectID)
	res, err := f.threads.ModelFind(ctx, f.storeID.String(), f.GetName(), query, []*Bucket{})
	if err != nil {
		return nil, err
	}
	return res.([]*Bucket), nil
}

func (f *Buckets) Save(ctx context.Context, bucket *Bucket) error {
	ctx = AuthCtx(ctx, f.token)
	return f.threads.ModelSave(ctx, f.storeID.String(), f.GetName(), bucket)
}

func (f *Buckets) Delete(ctx context.Context, id string) error {
	ctx = AuthCtx(ctx, f.token)
	return f.threads.ModelDelete(ctx, f.storeID.String(), f.GetName(), id)
}
