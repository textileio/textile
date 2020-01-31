package collections

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/textileio/go-threads/api/client"
	s "github.com/textileio/go-threads/store"
)

type File struct {
	ID        string
	Path      string
	Name      string
	ProjectID string
	Created   int64
}

type Files struct {
	threads *client.Client
	storeID *uuid.UUID
	token   string
}

func (f *Files) GetName() string {
	return "File"
}

func (f *Files) GetInstance() interface{} {
	return &File{}
}

func (f *Files) GetIndexes() []*s.IndexConfig {
	return []*s.IndexConfig{{
		Path:   "Path",
		Unique: true,
	}, {
		Path: "ProjectID",
	}}
}

func (f *Files) GetStoreID() *uuid.UUID {
	return f.storeID
}

func (f *Files) Create(ctx context.Context, pth path.Resolved, name, projectID string) (*File, error) {
	ctx = AuthCtx(ctx, f.token)
	file := &File{
		Path:      pth.String(),
		Name:      name,
		ProjectID: projectID,
		Created:   time.Now().Unix(),
	}
	if err := f.threads.ModelCreate(ctx, f.storeID.String(), f.GetName(), file); err != nil {
		return nil, err
	}
	return file, nil
}

func (f *Files) GetByPath(ctx context.Context, pth path.Resolved) (*File, error) {
	ctx = AuthCtx(ctx, f.token)
	query := s.JSONWhere("Path").Eq(pth.String())
	res, err := f.threads.ModelFind(ctx, f.storeID.String(), f.GetName(), query, []*File{})
	if err != nil {
		return nil, err
	}
	files := res.([]*File)
	if len(files) == 0 {
		return nil, nil
	}
	return files[0], nil
}

func (f *Files) List(ctx context.Context, projectID string) ([]*File, error) {
	ctx = AuthCtx(ctx, f.token)
	query := s.JSONWhere("ProjectID").Eq(projectID)
	res, err := f.threads.ModelFind(ctx, f.storeID.String(), f.GetName(), query, []*File{})
	if err != nil {
		return nil, err
	}
	return res.([]*File), nil
}

func (f *Files) Delete(ctx context.Context, id string) error {
	ctx = AuthCtx(ctx, f.token)
	return f.threads.ModelDelete(ctx, f.storeID.String(), f.GetName(), id)
}
