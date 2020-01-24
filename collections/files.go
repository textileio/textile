package collections

import (
	"context"

	"github.com/google/uuid"
	"github.com/textileio/go-threads/api/client"
	s "github.com/textileio/go-threads/store"
)

type File struct {
	ID        string
	Path      string
	Name      string
	ProjectID string
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
	}}
}

func (f *Files) GetStoreID() *uuid.UUID {
	return f.storeID
}

func (f *Files) Create(ctx context.Context, path, name, projID string) (*File, error) {
	ctx = AuthCtx(ctx, f.token)
	file := &File{
		Path:      path,
		Name:      name,
		ProjectID: projID,
	}
	if err := f.threads.ModelCreate(ctx, f.storeID.String(), f.GetName(), file); err != nil {
		return nil, err
	}
	return file, nil
}

func (f *Files) Get(ctx context.Context, id string) (*File, error) {
	ctx = AuthCtx(ctx, f.token)
	file := &File{}
	if err := f.threads.ModelFindByID(ctx, f.storeID.String(), f.GetName(), id, file); err != nil {
		return nil, err
	}
	return file, nil
}

func (f *Files) Delete(ctx context.Context, id string) error {
	ctx = AuthCtx(ctx, f.token)
	return f.threads.ModelDelete(ctx, f.storeID.String(), f.GetName(), id)
}
