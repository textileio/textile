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

func (f *Buckets) Create(ctx context.Context, pth path.Resolved, storeID, name, projectID string) (*Bucket, error) {
	validName, err := toValidName(name)
	if err != nil {
		return nil, err
	}
	ctx = AuthCtx(ctx, f.token)
	bucket := &Bucket{
		Path:      pth.String(),
		Name:      validName,
		ProjectID: projectID,
		Created:   time.Now().Unix(),
		Updated:   time.Now().Unix(),
	}
	var sid string
	if storeID == "" {
		sid = f.storeID.String()
	} else {
		sid = storeID
	}
	if err := f.threads.ModelCreate(ctx, sid, f.GetName(), bucket); err != nil {
		return nil, err
	}
	return bucket, nil
}

func (f *Buckets) GetByName(ctx context.Context, storeID, name string) (*Bucket, error) {
	ctx = AuthCtx(ctx, f.token)
	var sid string
	if storeID == "" {
		sid = f.storeID.String()
	} else {
		sid = storeID
	}
	query := s.JSONWhere("Name").Eq(name)
	res, err := f.threads.ModelFind(ctx, sid, f.GetName(), query, []*Bucket{})
	if err != nil {
		return nil, err
	}
	buckets := res.([]*Bucket)
	if len(buckets) == 0 {
		return nil, nil
	}
	return buckets[0], nil
}

func (f *Buckets) List(ctx context.Context, storeID string, projectID string) ([]*Bucket, error) {
	ctx = AuthCtx(ctx, f.token)
	var sid string
	if storeID == "" {
		sid = f.storeID.String()
	} else {
		sid = storeID
	}
	query := s.JSONWhere("ProjectID").Eq(projectID)
	res, err := f.threads.ModelFind(ctx, sid, f.GetName(), query, []*Bucket{})
	if err != nil {
		return nil, err
	}
	return res.([]*Bucket), nil
}

func (f *Buckets) Save(ctx context.Context, storeID string, bucket *Bucket) error {
	ctx = AuthCtx(ctx, f.token)
	var sid string
	if storeID == "" {
		sid = f.storeID.String()
	} else {
		sid = storeID
	}
	return f.threads.ModelSave(ctx, sid, f.GetName(), bucket)
}

func (f *Buckets) Delete(ctx context.Context, storeID, id string) error {
	ctx = AuthCtx(ctx, f.token)
	var sid string
	if storeID == "" {
		sid = f.storeID.String()
	} else {
		sid = storeID
	}
	return f.threads.ModelDelete(ctx, sid, f.GetName(), id)
}
